package ws

import (
	"context"
	"log"
	"time"

	"cursorIM/internal/chat"
	"cursorIM/internal/protocol"
)

// Hub 负责管理客户端连接和广播消息
type Hub struct {
	// 已注册的客户端映射，按用户ID索引
	clients map[string]*Client

	// 将消息发送到特定客户端
	send chan *protocol.Message

	// 客户端注册请求
	register chan *Client

	// 客户端取消注册请求
	unregister chan *Client

	// 消息服务
	messageService *chat.MessageService
}

// NewHub 创建一个新的Hub实例
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		send:       make(chan *protocol.Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run 开始Hub的消息处理循环
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client.UserID] = client
			log.Printf("Client registered: %s", client.UserID)
		case client := <-h.unregister:
			if _, ok := h.clients[client.UserID]; ok {
				delete(h.clients, client.UserID)
				close(client.send)
				log.Printf("Client unregistered: %s", client.UserID)
			}
		case message := <-h.send:
			// 保存消息到数据库
			go func(msg *protocol.Message) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				err := h.messageService.SaveMessage(ctx, msg)
				if err != nil {
					log.Printf("Error saving message: %v", err)
				}
			}(message)

			// 发送消息到接收者
			recipientID := message.RecipientID
			if client, ok := h.clients[recipientID]; ok {
				select {
				case client.send <- message:
					log.Printf("Message sent to %s", recipientID)
				default:
					log.Printf("Failed to send message to %s, client buffer full", recipientID)
				}
			} else {
				log.Printf("Recipient %s not connected", recipientID)
			}

			// 如果是群聊消息，发送给所有群成员
			if message.IsGroup {
				// 这里可以添加群聊消息分发逻辑
			}
		}
	}
}
