package chat

// ConversationResponse 会话响应模型
type ConversationResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	LastMessage string `json:"lastMessage"`
	Unread      int    `json:"unread"`
	IsGroup     bool   `json:"isGroup"`
}
