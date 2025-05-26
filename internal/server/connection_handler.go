package server

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"cursorIM/internal/chat"
	"cursorIM/internal/connection"
	"cursorIM/internal/middleware"
	"cursorIM/internal/protocol"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Global WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins, should be stricter in production
	},
}

// WebSocketHandler handles WebSocket connections
func WebSocketHandler(connMgr connection.ConnectionManager, messageService *chat.MessageService, tcpStyle bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var userID string
		var err error

		if tcpStyle {
			// TCP-style WebSocket needs authentication after connection
			ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				log.Printf("Failed to upgrade WebSocket connection: %v", err)
				return
			}

			// Handle authentication immediately
			userID, err = authenticateTCPStyleWS(ws)
			if err != nil {
				log.Printf("TCP-style WebSocket authentication failed: %v", err)
				ws.Close()
				return
			}

			log.Printf("User %s authenticated via TCP-style WebSocket", userID)

			// Handle the TCP-style WebSocket connection
			conn := connection.NewWebSocketConnection(ws, userID, connection.ConnectionTypeTCPWS)
			handleAuthenticatedConnection(conn, userID, connMgr, messageService)
		} else {
			// Standard WebSocket authenticates first
			token := c.Query("token")
			if token == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
				return
			}

			// Validate token
			userID, err = middleware.ValidateToken(token)
			if err != nil {
				log.Printf("WebSocket connection failed - invalid token: %v", err)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
				return
			}

			log.Printf("User %s attempting to establish standard WebSocket connection", userID)

			// Upgrade HTTP connection to WebSocket
			ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				log.Printf("Failed to upgrade WebSocket connection: %v", err)
				return
			}

			// Handle the standard WebSocket connection
			conn := connection.NewWebSocketConnection(ws, userID, connection.ConnectionTypeWebSocket)
			handleAuthenticatedConnection(conn, userID, connMgr, messageService)
		}
	}
}

// authenticateTCPStyleWS handles TCP-style WebSocket authentication
func authenticateTCPStyleWS(ws *websocket.Conn) (string, error) {
	// Wait for authentication message
	ws.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, authMsg, err := ws.ReadMessage()
	if err != nil {
		return "", err
	}

	// Parse authentication message (format: AUTH {token})
	authStr := string(authMsg)
	authStr = strings.TrimSpace(authStr)
	parts := strings.SplitN(authStr, " ", 2)
	if len(parts) != 2 || parts[0] != "AUTH" {
		ws.WriteMessage(websocket.TextMessage, []byte("ERROR Invalid authentication format\n"))
		return "", fmt.Errorf("invalid authentication format")
	}

	token := parts[1]

	// Validate token
	userID, err := middleware.ValidateToken(token)
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("ERROR Authentication failed\n"))
		return "", err
	}

	// Send authentication success message
	if err := ws.WriteMessage(websocket.TextMessage, []byte("OK\n")); err != nil {
		return "", err
	}

	// Clear read deadline
	ws.SetReadDeadline(time.Time{})

	return userID, nil
}

// authenticateTCPConn handles TCP connection authentication
func authenticateTCPConn(conn net.Conn) (string, error) {
	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetReadDeadline(time.Time{}) // Clear timeout

	// Read authentication info
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read authentication info: %w", err)
	}

	// Parse authentication info
	line = strings.TrimSpace(line)
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 || parts[0] != "AUTH" {
		// Send authentication failure message
		conn.Write([]byte("ERROR Invalid authentication format\n"))
		return "", fmt.Errorf("invalid authentication format")
	}

	token := parts[1]

	// Validate token
	userID, err := middleware.ValidateToken(token)
	if err != nil {
		// Send authentication failure message
		conn.Write([]byte("ERROR Authentication failed\n"))
		return "", fmt.Errorf("invalid token: %w", err)
	}

	// Send authentication success message
	conn.Write([]byte("OK\n"))

	return userID, nil
}

// handleAuthenticatedConnection handles authenticated connections (both TCP and WebSocket)
func handleAuthenticatedConnection(conn connection.Connection, userID string, connMgr connection.ConnectionManager, messageService *chat.MessageService) {
	connType := conn.GetConnectionType()

	// Prioritize TCP-style connections by unregistering standard WebSocket
	if connType == connection.ConnectionTypeTCPWS || connType == connection.ConnectionTypeTCP {
		connMgr.UnregisterConnection(userID, connection.ConnectionTypeWebSocket)
	}

	// Register connection
	if err := connMgr.RegisterConnection(userID, conn); err != nil {
		log.Printf("Failed to register %s connection: %v", connType, err)
		conn.Close()
		return
	}

	// Deferred unregistration
	defer connMgr.UnregisterConnection(userID, connType)

	// Send user online status
	sendUserStatusUpdate(userID, true, messageService)

	// Start message processing
	go processMessages(conn, userID, connMgr, messageService)

	// Wait for connection to close
	<-conn.GetDoneChan()

	// Send user offline status
	sendUserStatusUpdate(userID, false, messageService)
	log.Printf("User %s's %s connection closed", userID, connType)
}

// processMessages processes messages from a connection
func processMessages(conn connection.Connection, userID string, connMgr connection.ConnectionManager, messageService *chat.MessageService) {
	// Handle incoming messages based on connection type
	switch wsConn := conn.(type) {
	case *connection.WebSocketConnection:
		// Start writing in a separate goroutine so it doesn't block reading
		go wsConn.StartWriting()

		// StartReading blocks, so we call it last
		wsConn.StartReading(func(msg *protocol.Message) {
			handleMessage(connMgr, messageService, userID, msg)
		})
	case *connection.TCPConnection:
		tcpConn := conn.(*connection.TCPConnection)

		// Start writing in a separate goroutine so it doesn't block reading
		go tcpConn.StartWriting()

		// StartReading blocks, so we call it last
		tcpConn.StartReading(func(msg *protocol.Message) {
			handleMessage(connMgr, messageService, userID, msg)
		})
	}
}

// handleMessage 处理收到的消息
func handleMessage(connMgr connection.ConnectionManager, messageService *chat.MessageService, userID string, message *protocol.Message) error {
	// 设置发送者ID和时间戳
	message.SenderID = userID
	if message.Timestamp == 0 {
		message.Timestamp = time.Now().Unix()
	}

	// 记录解析后的消息
	log.Printf("处理消息: %+v", message)

	// 检查消息接收者
	if message.RecipientID == "" && !message.IsGroup && message.Type != "ping" && message.Type != "pong" && message.Type != "status" {
		log.Printf("警告: 用户 %s 发送的消息没有接收者ID: %+v", userID, message)
		// 返回错误但不中断处理
		errorMsg := &protocol.Message{
			Type:        "error",
			SenderID:    "server",
			RecipientID: userID,
			Content:     "消息缺少接收者ID",
			Timestamp:   time.Now().Unix(),
		}
		// 尝试发送错误消息给发送者
		if err := connMgr.SendMessage(errorMsg); err != nil {
			log.Printf("向用户 %s 发送错误消息失败: %v", userID, err)
		}
		return fmt.Errorf("消息缺少接收者ID")
	}

	// 处理消息
	if message.Type == "ping" {
		// 处理ping消息，发送pong响应
		log.Printf("收到用户 %s 的ping消息，发送pong响应", userID)
		pongMsg := &protocol.Message{
			Type:        "pong",
			SenderID:    "server",
			RecipientID: userID, // 确保pong消息发回给发送ping的用户
			Timestamp:   time.Now().Unix(),
		}

		return connMgr.SendMessage(pongMsg)
	} else if message.Type == "pong" {
		// 忽略pong消息
		log.Printf("收到用户 %s 的pong消息", userID)
		return nil
	} else if message.Type == "status" {
		// 处理状态更新消息
		log.Printf("处理用户 %s 的状态更新: %s", userID, message.Content)
		return messageService.BroadcastStatus(context.Background(), message)
	} else {
		// 保存消息到数据库
		log.Printf("保存用户 %s 发送的消息到数据库", userID)

		// 确保消息有会话ID
		if message.ConversationID == "" {
			log.Printf("警告: 消息缺少会话ID，尝试生成临时会话ID")
			// 为一对一消息生成临时会话ID
			if message.RecipientID != "" {
				// 生成一个更短的临时会话ID (使用UUID)
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

// sendUserStatusUpdate sends a user status update
func sendUserStatusUpdate(userID string, online bool, messageService *chat.MessageService) {
	status := "online"
	if !online {
		status = "offline"
	}

	// Create status notification message
	statusMsg := &protocol.Message{
		Type:        "status",
		SenderID:    userID,
		RecipientID: "system", // System message
		Content:     status,
		Timestamp:   time.Now().Unix(),
	}

	// Broadcast status change
	if err := messageService.BroadcastStatus(context.Background(), statusMsg); err != nil {
		log.Printf("Failed to broadcast status notification: %v", err)
	}
}

// TCPServer handles TCP connections
type TCPServer struct {
	addr           string
	listener       net.Listener
	connMgr        connection.ConnectionManager
	messageService *chat.MessageService
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewTCPServer creates a new TCP server
func NewTCPServer(addr string, connMgr connection.ConnectionManager, messageService *chat.MessageService) *TCPServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &TCPServer{
		addr:           addr,
		connMgr:        connMgr,
		messageService: messageService,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start starts the TCP server
func (s *TCPServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("TCP server listen failed: %w", err)
	}

	log.Printf("TCP server started, listening at: %s", s.addr)

	go s.acceptConnections()

	return nil
}

// Stop stops the TCP server
func (s *TCPServer) Stop() error {
	s.cancel()
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// acceptConnections accepts TCP connections
func (s *TCPServer) acceptConnections() {
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
				log.Printf("Failed to accept TCP connection: %v", err)
				continue
			}

			go s.handleConnection(conn)
		}
	}
}

// handleConnection handles a TCP connection
func (s *TCPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// First step: authentication
	userID, err := authenticateTCPConn(conn)
	if err != nil {
		log.Printf("TCP connection authentication failed: %v", err)
		return
	}

	log.Printf("User %s authenticated via TCP connection", userID)

	// Create TCP connection object
	tcpConn := connection.NewTCPConnection(conn, userID)

	// Handle the connection
	handleAuthenticatedConnection(tcpConn, userID, s.connMgr, s.messageService)
}
