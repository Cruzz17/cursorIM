package protocol

import "time"

type Message struct {
	ID             string    `json:"id"`
	SenderID       string    `json:"sender_id"`
	RecipientID    string    `json:"recipient_id"` // 确保这个字段存在并有正确的json标签
	Content        string    `json:"content"`
	Type           string    `json:"type"`
	Timestamp      int64     `json:"timestamp"`
	ConversationID string    `json:"conversation_id"`
	IsGroup        bool      `json:"is_group,omitempty"`
	Status         string    `json:"status,omitempty"`
	CreatedAt      time.Time `json:"-"`
	UpdatedAt      time.Time `json:"-"`
	HandledByLocal bool      `json:"handledByLocal"` // 标记消息是否已被本地节点处理
}
