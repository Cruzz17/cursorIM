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
	Online    bool      `gorm:"default:false" json:"online"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Friendship 好友关系
type Friendship struct {
	ID        string `gorm:"primaryKey;type:varchar(36)"`
	UserID    string `gorm:"type:varchar(36);index:idx_user_friend"`
	FriendID  string `gorm:"type:varchar(36);index:idx_user_friend"`
	Status    int    `gorm:"default:0"` // 0-待确认，1-已好友
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Group 群组表
type Group struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name      string    `gorm:"type:varchar(50);not null" json:"name"`
	OwnerID   string    `gorm:"type:varchar(36);not null" json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GroupMember 群成员表
type GroupMember struct {
	ID       string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	GroupID  string    `gorm:"type:varchar(36);index:idx_group_member" json:"group_id"`
	UserID   string    `gorm:"type:varchar(36);index:idx_group_member" json:"user_id"`
	Role     int       `gorm:"default:0" json:"role"` // 0-成员，1-管理员
	JoinedAt time.Time `json:"joined_at"`
}

// Conversation 会话
type Conversation struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name      string    `gorm:"type:varchar(100)" json:"name"` // 会话名称
	Type      int       `gorm:"default:0" json:"type"`         // 0-单聊，1-群聊
	IsGroup   bool      `gorm:"default:false" json:"is_group"` // 是否是群聊
	LastMsg   string    `gorm:"type:text" json:"last_msg"`
	LastTime  time.Time `json:"last_time"`
	Unread    int       `gorm:"default:0" json:"unread"`
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

// PrivateMessage 单聊消息表
type PrivateMessage struct {
	ID         string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	SenderID   string    `gorm:"type:varchar(36);index" json:"sender_id"`
	ReceiverID string    `gorm:"type:varchar(36);index" json:"receiver_id"`
	Type       string    `gorm:"type:varchar(10);default:'text'" json:"type"` // text/image/file
	Content    string    `gorm:"type:text" json:"content"`
	SentAt     time.Time `json:"sent_at"`
	Read       bool      `gorm:"default:false" json:"read"`
}

// GroupMessage 群聊消息表
type GroupMessage struct {
	ID       string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	GroupID  string    `gorm:"type:varchar(36);index" json:"group_id"`
	SenderID string    `gorm:"type:varchar(36);index" json:"sender_id"`
	Type     string    `gorm:"type:varchar(10);default:'text'" json:"type"` // text/image/file
	Content  string    `gorm:"type:text" json:"content"`
	SentAt   time.Time `json:"sent_at"`
}

// Message 消息
type Message struct {
	ID             string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	ConversationID string    `gorm:"type:varchar(100);index" json:"conversation_id"` // 增加长度以支持临时会话ID
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
		&Group{},
		&GroupMember{},
		&Conversation{},
		&Participant{},
		&PrivateMessage{},
		&GroupMessage{},
		&Message{},
	)
}
