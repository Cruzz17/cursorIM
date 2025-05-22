package connection

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"cursorIM/internal/protocol"
)

// TCPConnection 实现 TCP 连接
type TCPConnection struct {
	conn   net.Conn
	userID string
	send   chan *protocol.Message
	done   chan struct{}
}

// NewTCPConnection 创建新的 TCP 连接
func NewTCPConnection(conn net.Conn, userID string) *TCPConnection {
	return &TCPConnection{
		conn:   conn,
		userID: userID,
		send:   make(chan *protocol.Message, 256),
		done:   make(chan struct{}),
	}
}

// SendMessage 发送消息到 TCP 客户端
func (c *TCPConnection) SendMessage(message *protocol.Message) error {
	select {
	case c.send <- message:
		return nil
	default:
		return ErrConnectionBufferFull
	}
}

// Close 关闭 TCP 连接
func (c *TCPConnection) Close() error {
	select {
	case <-c.done:
		// 已经关闭
		return nil
	default:
		close(c.done)
	}

	close(c.send)
	return c.conn.Close()
}

// GetDoneChan 获取完成通道
func (c *TCPConnection) GetDoneChan() <-chan struct{} {
	return c.done
}

// GetUserID 获取用户 ID
func (c *TCPConnection) GetUserID() string {
	return c.userID
}

// GetConnectionType 获取连接类型
func (c *TCPConnection) GetConnectionType() string {
	return ConnectionTypeTCP
}

// GetSendChannel 获取发送通道
func (c *TCPConnection) GetSendChannel() <-chan *protocol.Message {
	return c.send
}

// StartReading 开始从 TCP 读取消息
func (c *TCPConnection) StartReading(msgHandler func(*protocol.Message)) {
	defer c.Close()

	buffer := make([]byte, 4096)
	messageBuffer := []byte{}

	for {
		select {
		case <-c.done:
			return
		default:
			// 设置读取超时
			c.conn.SetReadDeadline(time.Now().Add(PongWait))

			n, err := c.conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// 超时，发送心跳检测
					continue
				}

				log.Printf("TCP 读取错误: %v", err)
				return
			}

			// 追加到消息缓冲区
			messageBuffer = append(messageBuffer, buffer[:n]...)

			// 尝试解析完整消息
			messages, remainder := c.parseMessages(messageBuffer)
			messageBuffer = remainder

			// 处理解析出的所有消息
			for _, msg := range messages {
				// 设置发送者 ID 和时间戳
				msg.SenderID = c.userID
				if msg.Timestamp == 0 {
					msg.Timestamp = time.Now().Unix()
				}

				log.Printf("从用户 %s 接收到消息: Type=%s, To=%s, Content=%s",
					c.userID, msg.Type, msg.RecipientID, msg.Content)

				// 将消息传递给处理函数
				msgHandler(msg)
			}
		}
	}
}

// parseMessages 解析可能包含多个消息的数据
func (c *TCPConnection) parseMessages(data []byte) ([]*protocol.Message, []byte) {
	if len(data) == 0 {
		return nil, data
	}

	var messages []*protocol.Message
	var remainder = data

	// 尝试解析一个或多个JSON消息
	for len(remainder) > 0 {
		// 查找JSON边界
		var endIdx = len(remainder)
		bracketCount := 0
		foundComplete := false

		for i, b := range remainder {
			if b == '{' {
				bracketCount++
			} else if b == '}' {
				bracketCount--
				if bracketCount == 0 {
					endIdx = i + 1 // 包含结束括号
					foundComplete = true
					break
				}
			}
		}

		if !foundComplete {
			// 没有找到完整的JSON对象，保留剩余部分等待更多数据
			return messages, remainder
		}

		// 尝试解析这个可能的JSON对象
		var message protocol.Message
		if err := json.Unmarshal(remainder[:endIdx], &message); err == nil {
			messages = append(messages, &message)
			remainder = remainder[endIdx:]

			// 跳过任何空白字符
			for len(remainder) > 0 && (remainder[0] == ' ' || remainder[0] == '\n' || remainder[0] == '\r' || remainder[0] == '\t') {
				remainder = remainder[1:]
			}
		} else {
			// 解析错误，跳过这个字节并继续
			log.Printf("解析消息失败: %v", err)
			remainder = remainder[1:]
		}
	}

	return messages, remainder
}

// StartWriting 开始向 TCP 写入消息
func (c *TCPConnection) StartWriting() {
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
				return
			}

			// 将消息序列化为 JSON
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("消息序列化错误: %v", err)
				continue
			}

			// 设置写入超时
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))

			// 写入消息长度和内容
			_, err = c.conn.Write(data)
			if err != nil {
				log.Printf("TCP 写入错误: %v", err)
				return
			}

		case <-ticker.C:
			// 发送心跳消息
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			pingMessage := &protocol.Message{
				Type: "ping",
			}
			data, _ := json.Marshal(pingMessage)
			if _, err := c.conn.Write(data); err != nil {
				log.Printf("TCP 心跳错误: %v", err)
				return
			}
		}
	}
}
