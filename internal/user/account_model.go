package user

import (
	"time"
)

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Nickname  string `json:"nickname" binding:"required"`
	AvatarURL string `json:"avatar_url"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	UserID string `json:"user_id"`
	Token  string `json:"token"`
}

// UserResponse 用户信息响应
type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

// AddFriendRequest 添加好友请求
type AddFriendRequest struct {
	FriendID string `json:"friendId" binding:"required"`
}
