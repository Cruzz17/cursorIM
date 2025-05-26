package status

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"cursorIM/internal/redisclient"

	"github.com/go-redis/redis/v8"
)

// UserStatus 表示用户状态
type UserStatus struct {
	UserID      string    `json:"user_id"`
	Online      bool      `json:"online"`
	LastActive  time.Time `json:"last_active"`
	Connections struct {
		HTTP      bool `json:"http"`
		WebSocket bool `json:"websocket"`
		TCP       bool `json:"tcp"`
	} `json:"connections"`
}

// Manager 统一用户状态管理
type Manager struct {
	redisClient  *redis.Client
	redisEnabled bool
	statusCache  map[string]*UserStatus // 本地状态缓存
	mutex        sync.RWMutex
	ctx          context.Context
}

// NewManager 创建状态管理器
func NewManager(ctx context.Context) *Manager {
	return &Manager{
		redisClient:  redisclient.GetRedisClient(),
		redisEnabled: redisclient.IsRedisEnabled(),
		statusCache:  make(map[string]*UserStatus),
		ctx:          ctx,
	}
}

// UpdateUserStatus 更新用户状态
func (m *Manager) UpdateUserStatus(userID string, connectionType string, online bool) error {
	now := time.Now()

	// 更新本地缓存
	m.mutex.Lock()
	var status *UserStatus
	if s, exists := m.statusCache[userID]; exists {
		status = s
		status.LastActive = now
		status.Online = online
	} else {
		status = &UserStatus{
			UserID:     userID,
			Online:     online,
			LastActive: now,
		}
		m.statusCache[userID] = status
	}

	// 更新连接类型
	switch connectionType {
	case "http":
		status.Connections.HTTP = online
	case "websocket":
		status.Connections.WebSocket = online
	case "tcp":
		status.Connections.TCP = online
	}
	m.mutex.Unlock()

	// 如果Redis可用，同步到Redis
	if m.redisEnabled {
		return m.syncToRedis(userID, status)
	}

	return nil
}

// syncToRedis 将状态同步到Redis
func (m *Manager) syncToRedis(userID string, status *UserStatus) error {
	statusKey := fmt.Sprintf("user:%s:status", userID)
	connKey := fmt.Sprintf("user:%s:connections", userID)
	lastActiveKey := fmt.Sprintf("user:%s:last_active", userID)

	// 序列化状态
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("序列化用户状态失败: %w", err)
	}

	// 使用Redis事务保证原子性
	pipe := m.redisClient.Pipeline()
	pipe.Set(m.ctx, statusKey, data, 10*time.Minute)
	pipe.Set(m.ctx, lastActiveKey, status.LastActive.Unix(), 10*time.Minute)

	// 设置连接状态
	if status.Connections.HTTP {
		pipe.HSet(m.ctx, connKey, "http", "1")
	} else {
		pipe.HDel(m.ctx, connKey, "http")
	}

	if status.Connections.WebSocket {
		pipe.HSet(m.ctx, connKey, "websocket", "1")
	} else {
		pipe.HDel(m.ctx, connKey, "websocket")
	}

	if status.Connections.TCP {
		pipe.HSet(m.ctx, connKey, "tcp", "1")
	} else {
		pipe.HDel(m.ctx, connKey, "tcp")
	}

	// 如果在线，添加到在线用户集合
	if status.Online {
		pipe.SAdd(m.ctx, "online_users", userID)
	} else {
		pipe.SRem(m.ctx, "online_users", userID)
	}

	// 执行事务
	_, err = pipe.Exec(m.ctx)
	if err != nil {
		return fmt.Errorf("同步用户状态到Redis失败: %w", err)
	}

	return nil
}

// GetUserStatus 获取用户状态
func (m *Manager) GetUserStatus(userID string) (*UserStatus, error) {
	// 先查本地缓存
	m.mutex.RLock()
	if status, ok := m.statusCache[userID]; ok {
		// 检查是否过期
		if time.Since(status.LastActive) < 10*time.Minute {
			result := *status // 复制一份
			m.mutex.RUnlock()
			return &result, nil
		}
	}
	m.mutex.RUnlock()

	// 查询Redis
	if m.redisEnabled {
		statusKey := fmt.Sprintf("user:%s:status", userID)
		data, err := m.redisClient.Get(m.ctx, statusKey).Bytes()
		if err == nil {
			var status UserStatus
			if err = json.Unmarshal(data, &status); err == nil {
				m.mutex.Lock()
				m.statusCache[userID] = &status // 更新本地缓存
				m.mutex.Unlock()
				return &status, nil
			}
		}
	}

	// 默认返回离线状态
	return &UserStatus{
		UserID:     userID,
		Online:     false,
		LastActive: time.Now().Add(-1 * time.Hour), // 1小时前
	}, nil
}

// IsUserOnline 检查用户是否在线
func (m *Manager) IsUserOnline(userID string) (bool, error) {
	status, err := m.GetUserStatus(userID)
	if err != nil {
		return false, err
	}
	return status.Online, nil
}

// MarkUserOffline 标记用户为离线
func (m *Manager) MarkUserOffline(userID string) error {
	return m.UpdateUserStatus(userID, "all", false)
}

// GetOnlineUsers 获取所有在线用户
func (m *Manager) GetOnlineUsers() ([]string, error) {
	if !m.redisEnabled {
		// 如果Redis不可用，从本地缓存获取
		m.mutex.RLock()
		defer m.mutex.RUnlock()

		var onlineUsers []string
		for userID, status := range m.statusCache {
			if status.Online && time.Since(status.LastActive) < 10*time.Minute {
				onlineUsers = append(onlineUsers, userID)
			}
		}
		return onlineUsers, nil
	}

	// 从Redis获取在线用户列表
	users, err := m.redisClient.SMembers(m.ctx, "online_users").Result()
	if err != nil {
		return nil, fmt.Errorf("获取在线用户列表失败: %w", err)
	}

	return users, nil
}

// CleanupExpiredStatuses 清理过期的用户状态
func (m *Manager) CleanupExpiredStatuses() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for userID, status := range m.statusCache {
		if now.Sub(status.LastActive) > 10*time.Minute {
			delete(m.statusCache, userID)
		}
	}

	return nil
}
