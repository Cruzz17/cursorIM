package connection

import (
	"fmt"
	"log"
	"time"

	"cursorIM/internal/protocol"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// EnhancedWebSocketConnection 增强的 WebSocket 连接，支持协议适配
type EnhancedWebSocketConnection struct {
	*ProtocolAwareConnection
	conn     *websocket.Conn
	userID   string
	connType string
	send     chan *protocol.Message
	done     chan struct{}
}

// NewEnhancedWebSocketConnection 创建新的增强 WebSocket 连接
func NewEnhancedWebSocketConnection(conn *websocket.Conn, userID string, connType string) *EnhancedWebSocketConnection {
	if connType == "" {
		connType = ConnectionTypeWebSocket
	}

	protocolAware := NewProtocolAwareConnection(connType)

	return &EnhancedWebSocketConnection{
		ProtocolAwareConnection: protocolAware,
		conn:                    conn,
		userID:                  userID,
		connType:                connType,
		send:                    make(chan *protocol.Message, 256),
		done:                    make(chan struct{}),
	}
}

// SendMessage 发送消息到 WebSocket 客户端
func (c *EnhancedWebSocketConnection) SendMessage(message *protocol.Message) error {
	// 检查连接是否已关闭
	select {
	case <-c.done:
		return fmt.Errorf("连接已关闭")
	default:
		// 连接仍然打开，继续发送
	}

	// 安全地尝试发送消息
	select {
	case c.send <- message:
		return nil
	case <-c.done:
		return fmt.Errorf("连接已关闭")
	default:
		return fmt.Errorf("发送缓冲区已满")
	}
}

// SendMessageWithProtocol 使用指定协议发送消息
func (c *EnhancedWebSocketConnection) SendMessageWithProtocol(message *protocol.Message, protocolType protocol.ProtocolType) error {
	// 序列化消息
	data, err := c.adapter.SerializeMessage(message, protocolType)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 设置写入超时
	c.conn.SetWriteDeadline(time.Now().Add(WriteWait))

	// 根据协议类型选择发送方式
	switch protocolType {
	case protocol.ProtocolTypeJSON:
		// JSON 使用文本消息
		return c.conn.WriteMessage(websocket.TextMessage, data)
	case protocol.ProtocolTypeProtobuf:
		// Protobuf 使用二进制消息
		return c.conn.WriteMessage(websocket.BinaryMessage, data)
	default:
		return fmt.Errorf("不支持的协议类型: %s", protocolType)
	}
}

// Close 关闭 WebSocket 连接
func (c *EnhancedWebSocketConnection) Close() error {
	select {
	case <-c.done:
		return nil
	default:
		close(c.done)
	}

	close(c.send)
	return c.conn.Close()
}

// GetUserID 获取用户 ID
func (c *EnhancedWebSocketConnection) GetUserID() string {
	return c.userID
}

// GetConnectionType 获取连接类型
func (c *EnhancedWebSocketConnection) GetConnectionType() string {
	return c.connType
}

// GetDoneChan 获取完成通道
func (c *EnhancedWebSocketConnection) GetDoneChan() <-chan struct{} {
	return c.done
}

// GetSendChannel 获取发送通道
func (c *EnhancedWebSocketConnection) GetSendChannel() <-chan *protocol.Message {
	return c.send
}

// StartReading 开始从WebSocket读取消息
func (c *EnhancedWebSocketConnection) StartReading(msgHandler func(*protocol.Message)) {
	defer c.Close()

	// 设置读取限制和超时
	c.conn.SetReadLimit(MaxMessageSize * 2)
	c.conn.SetReadDeadline(time.Now().Add(PongWait * 2))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(PongWait * 2))
		log.Printf("用户 %s 接收到pong响应，重置读取超时", c.userID)
		return nil
	})

	log.Printf("用户 %s 的增强 WebSocket 连接已建立，协议类型: %s", c.userID, c.GetProtocolType())

	for {
		// 读取消息
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("用户 %s 的 WebSocket读取错误: %v", c.userID, err)
			} else {
				log.Printf("用户 %s 的 WebSocket连接关闭: %v", c.userID, err)
			}
			break
		}

		// 根据消息类型确定协议
		var protocolType protocol.ProtocolType
		switch messageType {
		case websocket.TextMessage:
			protocolType = protocol.ProtocolTypeJSON
		case websocket.BinaryMessage:
			protocolType = protocol.ProtocolTypeProtobuf
		default:
			log.Printf("不支持的 WebSocket 消息类型: %d", messageType)
			continue
		}

		// 反序列化消息
		message, err := c.adapter.DeserializeMessage(data, protocolType)
		if err != nil {
			log.Printf("反序列化消息失败: %v", err)
			continue
		}

		// 打印收到的消息
		log.Printf("用户 %s 收到消息 (协议: %s): Type=%s, To=%s",
			c.userID, protocolType, message.Type, message.RecipientID)

		// 设置发送者ID和时间戳
		message.SenderID = c.userID
		if message.Timestamp == 0 {
			message.Timestamp = time.Now().Unix()
		}

		// 确保有会话ID
		if message.ConversationID == "" && message.Type == "message" {
			if message.RecipientID != "" {
				tempConvID := uuid.New().String()
				message.ConversationID = tempConvID
				log.Printf("为消息生成临时会话ID: %s", tempConvID)
			}
		}

		// 处理ping消息
		if message.Type == "ping" {
			pongMsg := &protocol.Message{
				ID:        uuid.New().String(),
				Type:      "pong",
				SenderID:  "server",
				Timestamp: time.Now().Unix(),
			}
			if err := c.SendMessage(pongMsg); err != nil {
				log.Printf("用户 %s 发送pong消息失败: %v", c.userID, err)
			}
			continue
		}

		// 检查消息接收者
		if message.RecipientID == "" && message.Type != "status" {
			log.Printf("警告: 用户 %s 发送的消息没有接收者ID", c.userID)
			if message.Type == "message" {
				errorMsg := &protocol.Message{
					ID:          uuid.New().String(),
					Type:        "error",
					SenderID:    "server",
					RecipientID: c.userID,
					Content:     "消息缺少接收者ID",
					Timestamp:   time.Now().Unix(),
				}
				c.SendMessage(errorMsg)
			}
			continue
		}

		// 将消息传递给处理函数
		msgHandler(message)
	}
}

// StartWriting 开始向WebSocket写入消息
func (c *EnhancedWebSocketConnection) StartWriting() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-c.done:
			return
		case message, ok := <-c.send:
			if !ok {
				// 发送通道已关闭
				c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 使用连接的默认协议类型发送消息
			if err := c.SendMessageWithProtocol(message, c.GetProtocolType()); err != nil {
				log.Printf("发送消息失败: %v", err)
				return
			}

			log.Printf("✅ 成功发送消息到用户 %s (协议: %s)", c.userID, c.GetProtocolType())

		case <-ticker.C:
			// 检查连接是否已关闭
			select {
			case <-c.done:
				return
			default:
			}

			// 发送ping消息
			pingMsg := &protocol.Message{
				Type: "ping",
				ID:   uuid.New().String(),
			}

			// 使用JSON发送ping消息（兼容性更好）
			if err := c.SendMessageWithProtocol(pingMsg, protocol.ProtocolTypeJSON); err != nil {
				log.Printf("发送ping消息失败: %v", err)
				return
			}
		}
	}
}
