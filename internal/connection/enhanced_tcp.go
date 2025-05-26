package connection

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	"cursorIM/internal/protocol"

	"github.com/google/uuid"
)

// EnhancedTCPConnection 增强的 TCP 连接，支持协议适配
type EnhancedTCPConnection struct {
	*ProtocolAwareConnection
	conn     net.Conn
	userID   string
	connType string
	send     chan *protocol.Message
	done     chan struct{}
	reader   *bufio.Reader
	writer   *bufio.Writer
}

// NewEnhancedTCPConnection 创建新的增强 TCP 连接
func NewEnhancedTCPConnection(conn net.Conn, userID string, connType string) *EnhancedTCPConnection {
	if connType == "" {
		connType = ConnectionTypeTCP
	}

	protocolAware := NewProtocolAwareConnection(connType)

	return &EnhancedTCPConnection{
		ProtocolAwareConnection: protocolAware,
		conn:                    conn,
		userID:                  userID,
		connType:                connType,
		send:                    make(chan *protocol.Message, 256),
		done:                    make(chan struct{}),
		reader:                  bufio.NewReader(conn),
		writer:                  bufio.NewWriter(conn),
	}
}

// SendMessage 发送消息到 TCP 客户端
func (c *EnhancedTCPConnection) SendMessage(message *protocol.Message) error {
	// 检查连接是否已关闭
	select {
	case <-c.done:
		return fmt.Errorf("连接已关闭")
	default:
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
func (c *EnhancedTCPConnection) SendMessageWithProtocol(message *protocol.Message, protocolType protocol.ProtocolType) error {
	// 序列化消息
	data, err := c.adapter.SerializeMessage(message, protocolType)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 设置写入超时
	c.conn.SetWriteDeadline(time.Now().Add(WriteWait))

	// 写入协议标识符（1字节）+ 消息长度（4字节）+ 消息数据
	var protocolFlag byte
	switch protocolType {
	case protocol.ProtocolTypeJSON:
		protocolFlag = 0x01
	case protocol.ProtocolTypeProtobuf:
		protocolFlag = 0x02
	default:
		return fmt.Errorf("不支持的协议类型: %s", protocolType)
	}

	// 写入协议标识符
	if err := c.writer.WriteByte(protocolFlag); err != nil {
		return fmt.Errorf("写入协议标识符失败: %w", err)
	}

	// 写入消息长度
	msgLen := uint32(len(data))
	if err := binary.Write(c.writer, binary.BigEndian, msgLen); err != nil {
		return fmt.Errorf("写入消息长度失败: %w", err)
	}

	// 写入消息数据
	if _, err := c.writer.Write(data); err != nil {
		return fmt.Errorf("写入消息数据失败: %w", err)
	}

	// 刷新缓冲区
	if err := c.writer.Flush(); err != nil {
		return fmt.Errorf("刷新写入缓冲区失败: %w", err)
	}

	return nil
}

// Close 关闭 TCP 连接
func (c *EnhancedTCPConnection) Close() error {
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
func (c *EnhancedTCPConnection) GetUserID() string {
	return c.userID
}

// GetConnectionType 获取连接类型
func (c *EnhancedTCPConnection) GetConnectionType() string {
	return c.connType
}

// GetDoneChan 获取完成通道
func (c *EnhancedTCPConnection) GetDoneChan() <-chan struct{} {
	return c.done
}

// GetSendChannel 获取发送通道
func (c *EnhancedTCPConnection) GetSendChannel() <-chan *protocol.Message {
	return c.send
}

// StartReading 开始从TCP读取消息
func (c *EnhancedTCPConnection) StartReading(msgHandler func(*protocol.Message)) {
	defer c.Close()

	log.Printf("用户 %s 的增强 TCP 连接已建立，协议类型: %s", c.userID, c.GetProtocolType())

	for {
		select {
		case <-c.done:
			return
		default:
			// 设置读取超时
			c.conn.SetReadDeadline(time.Now().Add(PongWait))

			// 读取协议标识符（1字节）
			protocolFlag, err := c.reader.ReadByte()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("TCP 读取协议标识符错误: %v", err)
				return
			}

			// 确定协议类型
			var protocolType protocol.ProtocolType
			switch protocolFlag {
			case 0x01:
				protocolType = protocol.ProtocolTypeJSON
			case 0x02:
				protocolType = protocol.ProtocolTypeProtobuf
			default:
				log.Printf("未知的协议标识符: 0x%02x", protocolFlag)
				continue
			}

			// 读取消息长度（4字节）
			var msgLen uint32
			if err := binary.Read(c.reader, binary.BigEndian, &msgLen); err != nil {
				log.Printf("TCP 读取消息长度错误: %v", err)
				return
			}

			// 检查消息长度是否合理
			if msgLen > MaxMessageSize {
				log.Printf("消息长度过大: %d", msgLen)
				continue
			}

			// 读取消息数据
			data := make([]byte, msgLen)
			if _, err := c.reader.Read(data); err != nil {
				log.Printf("TCP 读取消息数据错误: %v", err)
				return
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
}

// StartWriting 开始向TCP写入消息
func (c *EnhancedTCPConnection) StartWriting() {
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

			// 使用连接的默认协议类型发送ping消息
			if err := c.SendMessageWithProtocol(pingMsg, c.GetProtocolType()); err != nil {
				log.Printf("发送ping消息失败: %v", err)
				return
			}
		}
	}
}
