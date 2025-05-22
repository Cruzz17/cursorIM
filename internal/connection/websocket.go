package connection

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cursorIM/internal/protocol"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// WebSocketConnection 实现 WebSocket 连接
type WebSocketConnection struct {
	conn     *websocket.Conn
	userID   string
	connType string
	send     chan *protocol.Message
	done     chan struct{}
}

// NewWebSocketConnection 创建新的 WebSocket 连接
func NewWebSocketConnection(conn *websocket.Conn, userID string, connType string) *WebSocketConnection {
	// 如果未指定连接类型，使用默认的WebSocket类型
	if connType == "" {
		connType = ConnectionTypeWebSocket
	}

	return &WebSocketConnection{
		conn:     conn,
		userID:   userID,
		connType: connType,
		send:     make(chan *protocol.Message, 256),
		done:     make(chan struct{}),
	}
}

// SendMessage 发送消息到 WebSocket 客户端
func (c *WebSocketConnection) SendMessage(message *protocol.Message) error {
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

// Close 关闭 WebSocket 连接
func (c *WebSocketConnection) Close() error {
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
func (c *WebSocketConnection) GetUserID() string {
	return c.userID
}

// GetConnectionType 获取连接类型
func (c *WebSocketConnection) GetConnectionType() string {
	return c.connType
}

// GetDoneChan 获取完成通道
func (c *WebSocketConnection) GetDoneChan() <-chan struct{} {
	return c.done
}

// GetSendChannel 获取发送通道
func (c *WebSocketConnection) GetSendChannel() <-chan *protocol.Message {
	return c.send
}

// StartReading 开始从WebSocket读取消息
func (c *WebSocketConnection) StartReading(msgHandler func(*protocol.Message)) {
	defer c.Close()

	// 设置更长的读取超时和更宽松的缓冲区
	c.conn.SetReadLimit(MaxMessageSize * 2)
	c.conn.SetReadDeadline(time.Now().Add(PongWait * 2)) // 增加超时时间
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(PongWait * 2)) // 增加超时时间
		log.Printf("用户 %s 接收到pong响应，重置读取超时", c.userID)
		return nil
	})

	// 记录连接已建立
	log.Printf("用户 %s 的 WebSocket 连接已成功建立并开始读取消息", c.userID)

	for {
		var message protocol.Message
		err := c.conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("用户 %s 的 WebSocket读取错误: %v", c.userID, err)
			} else {
				log.Printf("用户 %s 的 WebSocket连接关闭: %v", c.userID, err)
			}
			break
		}

		// 打印完整收到的消息内容，便于调试
		messageBytes, _ := json.Marshal(message)
		log.Printf("用户 %s 收到消息: %s", c.userID, string(messageBytes))

		// 设置发送者ID和时间戳
		message.SenderID = c.userID
		if message.Timestamp == 0 {
			message.Timestamp = time.Now().Unix()
		}

		// 确保有会话ID
		if message.ConversationID == "" && message.Type == "message" {
			log.Printf("警告: 消息缺少会话ID，可能无法正确处理")
			// 尝试使用接收者ID作为会话ID的情况下
			if message.RecipientID != "" {
				participantIDs := []string{c.userID, message.RecipientID}
				// 确保ID排序是一致的
				if c.userID > message.RecipientID {
					participantIDs[0] = message.RecipientID
					participantIDs[1] = c.userID
				}
				// 生成一个临时的会话ID
				message.ConversationID = fmt.Sprintf("temp_conv_%s_%s", participantIDs[0], participantIDs[1])
				log.Printf("为消息生成临时会话ID: %s", message.ConversationID)
			}
		}

		// 如果是ping消息，直接回复pong而不转发
		if message.Type == "ping" {
			pongMsg := &protocol.Message{
				ID:        uuid.New().String(),
				Type:      "pong",
				SenderID:  "server",
				Timestamp: time.Now().Unix(),
			}
			if err := c.SendMessage(pongMsg); err != nil {
				log.Printf("用户 %s 发送pong消息失败: %v", c.userID, err)
			} else {
				log.Printf("成功响应用户 %s 的ping消息", c.userID)
			}
			continue
		}

		// 确保消息有接收者ID
		if message.RecipientID == "" && message.Type != "status" {
			log.Printf("警告: 用户 %s 发送的消息没有接收者ID，无法处理", c.userID)
			log.Printf("消息内容: %+v", message)

			// 如果是普通消息但没有接收者，尝试返回错误给客户端
			if message.Type == "message" {
				errorMsg := &protocol.Message{
					ID:          uuid.New().String(),
					Type:        "error",
					SenderID:    "server",
					RecipientID: c.userID,
					Content:     "消息缺少接收者ID",
					Timestamp:   time.Now().Unix(),
				}
				if err := c.SendMessage(errorMsg); err != nil {
					log.Printf("向用户 %s 发送错误消息失败: %v", c.userID, err)
				}
			}
			continue
		}

		// 将消息传递给处理函数
		log.Printf("用户 %s 发送消息给 %s，类型: %s, 会话: %s",
			c.userID, message.RecipientID, message.Type, message.ConversationID)
		msgHandler(&message)
	}
}

// StartWriting 开始向WebSocket写入消息
func (c *WebSocketConnection) StartWriting() {
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
				// 发送通道已关闭，尝试优雅地关闭连接
				c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
				err := c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					log.Printf("发送关闭消息失败: %v", err)
				}
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			// 增加失败重试
			var err error
			for i := 0; i < 3; i++ { // 最多重试3次
				err = c.conn.WriteJSON(message)
				if err == nil {
					break
				}
				log.Printf("WebSocket写入失败(尝试 %d/3): %v", i+1, err)

				// 检查连接是否已关闭
				if websocket.IsCloseError(err) || websocket.IsUnexpectedCloseError(err) {
					log.Printf("连接已关闭，停止重试")
					return
				}

				time.Sleep(time.Millisecond * 100) // 短暂延迟后重试
			}

			if err != nil {
				log.Printf("WebSocket写入最终失败: %v", err)
				return
			}

		case <-ticker.C:
			// 检查连接是否已关闭
			select {
			case <-c.done:
				return
			default:
				// 连接仍然打开，发送ping消息
			}

			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			// 发送ping消息而不是ping帧，便于调试
			pingMsg := &protocol.Message{
				Type: "ping",
				ID:   uuid.New().String(),
			}

			data, _ := json.Marshal(pingMsg)
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("发送ping消息失败: %v", err)
				return
			}
		}
	}
}
