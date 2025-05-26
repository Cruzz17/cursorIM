package protocol

import "time"

type Message struct {
	Version    string `json:"version"`     // 协议版本号
	Type       string `json:"type"`        // 消息类型（message/command/response）
	StatusCode int    `json:"status_code"` // 状态码
	ErrorCode  string `json:"error_code"`  // 业务错误码
	RequestID  string `json:"request_id"`  // 请求链路追踪ID

	ID             string    `json:"id"`
	SenderID       string    `json:"sender_id"`
	RecipientID    string    `json:"recipient_id"`
	Content        string    `json:"content"`
	Timestamp      int64     `json:"timestamp"`
	ConversationID string    `json:"conversation_id"`
	IsGroup        bool      `json:"is_group,omitempty"`
	GroupID        string    `json:"group_id,omitempty"` // 群组ID，用于群聊消息
	Status         string    `json:"status,omitempty"`
	CreatedAt      time.Time `json:"-"`
	UpdatedAt      time.Time `json:"-"`
	HandledByLocal bool      `json:"handledByLocal"`

	// 错误信息
	Error struct {
		Message string `json:"message"`
		Details any    `json:"details"`
	} `json:"error,omitempty"`

	// 扩展元数据
	Metadata map[string]string `json:"metadata,omitempty"`
}
