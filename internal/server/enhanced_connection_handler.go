package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"cursorIM/internal/chat"
	"cursorIM/internal/connection"
	"cursorIM/internal/middleware"
	"cursorIM/internal/protocol"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EnhancedWebSocketHandler 增强的 WebSocket 处理器，支持协议适配
func EnhancedWebSocketHandler(connMgr connection.ConnectionManager, messageService *chat.MessageService, tcpStyle bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var userID string
		var err error

		if tcpStyle {
			// TCP-style WebSocket 需要连接后认证
			ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				log.Printf("Failed to upgrade WebSocket connection: %v", err)
				return
			}

			// 立即处理认证
			userID, err = authenticateTCPStyleWS(ws)
			if err != nil {
				log.Printf("TCP-style WebSocket authentication failed: %v", err)
				ws.Close()
				return
			}

			log.Printf("User %s authenticated via TCP-style WebSocket", userID)

			// 处理 TCP-style WebSocket 连接（使用 Protobuf）
			conn := connection.NewEnhancedWebSocketConnection(ws, userID, connection.ConnectionTypeTCPWS)
			handleEnhancedAuthenticatedConnection(conn, userID, connMgr, messageService)
		} else {
			// 标准 WebSocket 先认证
			token := c.Query("token")
			if token == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
				return
			}

			// 验证 token
			userID, err = middleware.ValidateToken(token)
			if err != nil {
				log.Printf("WebSocket connection failed - invalid token: %v", err)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
				return
			}

			log.Printf("User %s attempting to establish standard WebSocket connection", userID)

			// 升级 HTTP 连接为 WebSocket
			ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				log.Printf("Failed to upgrade WebSocket connection: %v", err)
				return
			}

			// 处理标准 WebSocket 连接（使用 JSON）
			conn := connection.NewEnhancedWebSocketConnection(ws, userID, connection.ConnectionTypeWebSocket)
			handleEnhancedAuthenticatedConnection(conn, userID, connMgr, messageService)
		}
	}
}

// handleEnhancedAuthenticatedConnection 处理已认证的增强连接
func handleEnhancedAuthenticatedConnection(conn connection.EnhancedConnection, userID string, connMgr connection.ConnectionManager, messageService *chat.MessageService) {
	connType := conn.GetConnectionType()
	protocolType := conn.GetProtocolType()

	log.Printf("建立增强连接: 用户=%s, 类型=%s, 协议=%s", userID, connType, protocolType)

	// 优先处理 TCP-style 连接
	if connType == connection.ConnectionTypeTCPWS || connType == connection.ConnectionTypeTCP {
		connMgr.UnregisterConnection(userID, connection.ConnectionTypeWebSocket)
	}

	// 注册连接
	if err := connMgr.RegisterConnection(userID, conn); err != nil {
		log.Printf("Failed to register %s connection: %v", connType, err)
		conn.Close()
		return
	}

	// 延迟注销连接
	defer connMgr.UnregisterConnection(userID, connType)

	// 发送用户在线状态
	sendUserStatusUpdate(userID, true, messageService)

	// 启动消息处理
	go processEnhancedMessages(conn, userID, connMgr, messageService)

	// 等待连接关闭
	<-conn.GetDoneChan()

	// 发送用户离线状态
	sendUserStatusUpdate(userID, false, messageService)
	log.Printf("User %s's %s connection closed (protocol: %s)", userID, connType, protocolType)
}

// processEnhancedMessages 处理增强连接的消息
func processEnhancedMessages(conn connection.EnhancedConnection, userID string, connMgr connection.ConnectionManager, messageService *chat.MessageService) {
	// 根据连接类型处理消息
	switch enhancedConn := conn.(type) {
	case *connection.EnhancedWebSocketConnection:
		// 启动写入协程
		go enhancedConn.StartWriting()
		// 启动读取（阻塞）
		enhancedConn.StartReading(func(msg *protocol.Message) {
			handleEnhancedMessage(connMgr, messageService, userID, msg)
		})
	case *connection.EnhancedTCPConnection:
		// 启动写入协程
		go enhancedConn.StartWriting()
		// 启动读取（阻塞）
		enhancedConn.StartReading(func(msg *protocol.Message) {
			handleEnhancedMessage(connMgr, messageService, userID, msg)
		})
	default:
		log.Printf("未知的增强连接类型: %T", conn)
	}
}

// handleEnhancedMessage 处理增强消息
func handleEnhancedMessage(connMgr connection.ConnectionManager, messageService *chat.MessageService, userID string, message *protocol.Message) error {
	// 设置发送者ID和时间戳
	message.SenderID = userID
	if message.Timestamp == 0 {
		message.Timestamp = time.Now().Unix()
	}

	// 记录解析后的消息
	log.Printf("处理增强消息: %+v", message)

	// 检查消息接收者
	if message.RecipientID == "" && !message.IsGroup && message.Type != "ping" && message.Type != "pong" && message.Type != "status" {
		log.Printf("警告: 用户 %s 发送的消息没有接收者ID: %+v", userID, message)
		// 返回错误消息
		errorMsg := &protocol.Message{
			Type:        "error",
			SenderID:    "server",
			RecipientID: userID,
			Content:     "消息缺少接收者ID",
			Timestamp:   time.Now().Unix(),
		}
		return connMgr.SendMessage(errorMsg)
	}

	// 处理消息
	switch message.Type {
	case "ping":
		// 处理ping消息，发送pong响应
		log.Printf("收到用户 %s 的ping消息，发送pong响应", userID)
		pongMsg := &protocol.Message{
			Type:        "pong",
			SenderID:    "server",
			RecipientID: userID,
			Timestamp:   time.Now().Unix(),
		}
		return connMgr.SendMessage(pongMsg)

	case "pong":
		// 忽略pong消息
		log.Printf("收到用户 %s 的pong消息", userID)
		return nil

	case "status":
		// 处理状态更新消息
		log.Printf("处理用户 %s 的状态更新: %s", userID, message.Content)
		return messageService.BroadcastStatus(context.Background(), message)

	default:
		// 保存消息到数据库
		log.Printf("保存用户 %s 发送的消息到数据库", userID)

		// 确保消息有会话ID
		if message.ConversationID == "" {
			log.Printf("警告: 消息缺少会话ID，尝试生成临时会话ID")
			if message.RecipientID != "" {
				tempConvID := uuid.New().String()
				message.ConversationID = tempConvID
				log.Printf("为消息生成临时会话ID: %s (用户: %s -> %s)", tempConvID, userID, message.RecipientID)
			} else {
				log.Printf("无法为消息生成会话ID，缺少必要信息")
				return fmt.Errorf("消息缺少会话ID和接收者ID")
			}
		}

		err := messageService.SaveMessage(context.Background(), message)
		if err != nil {
			log.Printf("保存消息失败: %v", err)
			return err
		}

		// 发送消息
		log.Printf("转发消息从用户 %s 到用户 %s", userID, message.RecipientID)
		return connMgr.SendMessage(message)
	}
}

// EnhancedTCPServer 增强的 TCP 服务器，支持协议适配
type EnhancedTCPServer struct {
	addr           string
	listener       net.Listener
	connMgr        connection.ConnectionManager
	messageService *chat.MessageService
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewEnhancedTCPServer 创建新的增强 TCP 服务器
func NewEnhancedTCPServer(addr string, connMgr connection.ConnectionManager, messageService *chat.MessageService) *EnhancedTCPServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &EnhancedTCPServer{
		addr:           addr,
		connMgr:        connMgr,
		messageService: messageService,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动增强 TCP 服务器
func (s *EnhancedTCPServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("Enhanced TCP server listen failed: %w", err)
	}

	log.Printf("Enhanced TCP server started, listening at: %s", s.addr)

	go s.acceptConnections()

	return nil
}

// Stop 停止增强 TCP 服务器
func (s *EnhancedTCPServer) Stop() error {
	s.cancel()
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// acceptConnections 接受 TCP 连接
func (s *EnhancedTCPServer) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if s.ctx.Err() != nil {
					// Context canceled, normal exit
					return
				}
				log.Printf("Failed to accept enhanced TCP connection: %v", err)
				continue
			}

			go s.handleConnection(conn)
		}
	}
}

// handleConnection 处理 TCP 连接
func (s *EnhancedTCPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// 首先进行认证
	userID, err := authenticateTCPConn(conn)
	if err != nil {
		log.Printf("Enhanced TCP connection authentication failed: %v", err)
		return
	}

	log.Printf("User %s authenticated via enhanced TCP connection", userID)

	// 创建增强 TCP 连接对象
	tcpConn := connection.NewEnhancedTCPConnection(conn, userID, connection.ConnectionTypeTCP)

	// 处理连接
	handleEnhancedAuthenticatedConnection(tcpConn, userID, s.connMgr, s.messageService)
}
