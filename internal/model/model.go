package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Username  string    `gorm:"type:varchar(50);uniqueIndex" json:"username"`
	Password  string    `gorm:"type:varchar(100)" json:"-"`
	Nickname  string    `gorm:"type:varchar(50)" json:"nickname"`
	AvatarURL string    `gorm:"type:varchar(255)" json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Friendship 好友关系
type Friendship struct {
	ID        string `gorm:"primaryKey;type:varchar(36)"`
	UserID    string `gorm:"type:varchar(36);index:idx_user_friend"`
	FriendID  string `gorm:"type:varchar(36);index:idx_user_friend"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Conversation 会话
type Conversation struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name      string    `gorm:"type:varchar(100)" json:"name"`
	IsGroup   bool      `json:"is_group"`
	JoinedAt  time.Time `json:"joined_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Participant 会话参与者
type Participant struct {
	ID             string `gorm:"primaryKey;type:varchar(36)"`
	UserID         string `gorm:"type:varchar(36);index:idx_conv_user"`
	ConversationID string `gorm:"type:varchar(36);index:idx_conv_user"`
	LastReadAt     time.Time
	JoinedAt       time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Message 消息
type Message struct {
	ID             string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	ConversationID string    `gorm:"type:varchar(36);index" json:"conversation_id"`
	SenderID       string    `gorm:"type:varchar(36);index" json:"sender_id"`
	Content        string    `gorm:"type:text" json:"content"`
	ContentType    string    `gorm:"type:varchar(20);default:'text'" json:"content_type"` // text, image, file
	Status         string    `gorm:"type:varchar(20);default:'sent'" json:"status"`       // sent, delivered, read
	Timestamp      int64     `json:"timestamp"`
	IsGroup        bool      `json:"is_group"`                                   // 是否是群组消息
	Type           string    `json:"type"`                                       // 文本、图片、文件等
	RecipientID    string    `gorm:"type:varchar(36);index" json:"recipient_id"` // 直接接收者ID
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SetupDatabase 初始化数据库表结构
func SetupDatabase(db *gorm.DB) error {
	// 自动迁移表结构
	return db.AutoMigrate(
		&User{},
		&Friendship{},
		&Conversation{},
		&Participant{},
		&Message{},
	)
}
