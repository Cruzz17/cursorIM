package model

import "time"

// Friend 表示好友关系
type Friend struct {
	ID        string `gorm:"primaryKey"`
	UserID    string `gorm:"not null;index"`
	FriendID  string `gorm:"not null;index"`
	Status    string `gorm:"not null;default:'accepted'"` // pending, accepted, rejected
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName 指定表名
func (Friend) TableName() string {
	return "friendships"
}
