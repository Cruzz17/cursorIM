package status

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/goccy/go-json"
)

// UserStatus 表示用户状态
type UserStatus struct {
	UserID      string    `json:"user_id"`
	Online      bool      `json:"online"`
	LastActive  time.Time `json:"last_active"`
	Connections struct {
		HTTP      bool `json:"http"`
		WebSocket bool `json:"websocket"`
	} `json:"connections"`
}

const (
	// 用户状态相关常量
	UserStatusOnline   = "online"    // 用户在线
	UserStatusOffline  = "offline"   // 用户离线
	UserStatusHTTPOnly = "http_only" // 用户仅HTTP连接

	// 状态过期时间
	StatusExpirationTime = 600 // 10分钟，单位秒
)

// StatusService 统一用户状态与连接管理服务
type StatusService struct {
	redisClient *redis.Client
	ctx         context.Context
}

// UpdateUserStatus 更新用户状态（整合连接类型管理）
func (s *StatusService) UpdateUserStatus(ctx context.Context, userID string, connectionType string) error {
	now := time.Now()
	statusKey := fmt.Sprintf("user:%s:status", userID)

	// 获取当前状态
	var status UserStatus
	statusData, err := s.redisClient.Get(ctx, statusKey).Bytes()
	if err != nil && err.Error() != "redisclient: nil" {
		return fmt.Errorf("获取用户状态失败: %w", err)
	}

	// 初始化新状态
	if err != nil || len(statusData) == 0 {
		status = UserStatus{
			UserID:     userID,
			Online:     true,
			LastActive: now,
		}
	} else {
		if err := json.Unmarshal(statusData, &status); err != nil {
			return fmt.Errorf("解析用户状态失败: %w", err)
		}
		status.LastActive = now
		status.Online = true
	}

	// 更新连接类型
	if connectionType == "http" {
		status.Connections.HTTP = true
	} else if connectionType == "websocket" {
		status.Connections.WebSocket = true
	}
	// 序列化并保存状态
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("序列化用户状态失败: %w", err)
	}

	// 设置状态，10分钟过期
	err = s.redisClient.Set(ctx, statusKey, data, 10*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("保存用户状态失败: %w", err)
	}

	// 将用户添加到在线用户集合
	err = s.redisClient.SAdd(ctx, "online_users", userID).Err()
	if err != nil {
		log.Printf("添加用户到在线集合失败: %v", err)
	}

	log.Printf("用户 %s 的 %s 连接状态已更新", userID, connectionType)
	return nil
}

// GetUserStatus 获取用户状态
func (s *StatusService) GetUserStatus(ctx context.Context, userID string) (string, error) {
	// 检查用户最后活跃时间
	lastActiveKey := fmt.Sprintf("user:%s:last_active", userID)
	lastActive, err := s.redisClient.Get(ctx, lastActiveKey).Int64()
	if err != nil {
		// 如果找不到键，用户可能离线
		return UserStatusOffline, nil
	}

	// 检查是否过期
	now := time.Now().Unix()
	if now-lastActive > StatusExpirationTime {
		return UserStatusOffline, nil
	}

	// 检查连接类型
	connKey := fmt.Sprintf("user:%s:connections", userID)
	hasWebSocket, err := s.redisClient.HExists(ctx, connKey, "websocket").Result()
	if err != nil {
		return UserStatusHTTPOnly, nil
	}

	if hasWebSocket {
		return UserStatusOnline, nil
	}

	return UserStatusHTTPOnly, nil
}

// CleanupExpiredStatuses 清理过期的用户状态
func (s *StatusService) CleanupExpiredStatuses(ctx context.Context) error {
	// 这个方法可以定期调用，清理过期的用户状态
	// 实际实现中可能需要使用Redis的扫描功能找出所有用户
	// 由于StatusExpirationTime已经在Redis键上设置了过期时间，大部分清理会自动完成

	log.Println("执行过期用户状态清理")
	return nil
}

// MarkUserOffline 标记用户为离线
func (s *StatusService) MarkUserOffline(ctx context.Context, userID string) error {
	// 清理用户连接状态
	connKey := fmt.Sprintf("user:%s:connections", userID)
	lastActiveKey := fmt.Sprintf("user:%s:last_active", userID)

	// 删除连接数据
	err := s.redisClient.Del(ctx, connKey).Err()
	if err != nil {
		return fmt.Errorf("删除用户连接状态失败: %w", err)
	}

	// 删除最后活跃时间
	err = s.redisClient.Del(ctx, lastActiveKey).Err()
	if err != nil {
		return fmt.Errorf("删除用户最后活跃时间失败: %w", err)
	}

	log.Printf("用户 %s 已被标记为离线", userID)
	return nil
}
