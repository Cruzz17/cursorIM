package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// UserConnectionRegistry 用户连接路由表
// 维护每个用户连接在哪台服务器上的映射关系
type UserConnectionRegistry struct {
	redisClient *redis.Client
	serverID    string          // 当前服务器的唯一标识
	localUsers  map[string]bool // 本地连接的用户集合
	mutex       sync.RWMutex
	ctx         context.Context
}

// UserConnectionInfo 用户连接信息
type UserConnectionInfo struct {
	UserID     string `json:"user_id"`
	ServerID   string `json:"server_id"`
	ConnType   string `json:"conn_type"`
	LastActive int64  `json:"last_active"`
	ServerAddr string `json:"server_addr"` // 服务器地址，用于直接通信
}

// NewUserConnectionRegistry 创建用户连接路由表
func NewUserConnectionRegistry(redisClient *redis.Client, serverID string, serverAddr string) *UserConnectionRegistry {
	return &UserConnectionRegistry{
		redisClient: redisClient,
		serverID:    serverID,
		localUsers:  make(map[string]bool),
		ctx:         context.Background(),
	}
}

// RegisterUser 注册用户连接
func (r *UserConnectionRegistry) RegisterUser(userID, connType string) error {
	r.mutex.Lock()
	r.localUsers[userID] = true
	r.mutex.Unlock()

	// 在Redis中注册用户连接信息
	connInfo := UserConnectionInfo{
		UserID:     userID,
		ServerID:   r.serverID,
		ConnType:   connType,
		LastActive: time.Now().Unix(),
	}

	data, err := json.Marshal(connInfo)
	if err != nil {
		return fmt.Errorf("序列化用户连接信息失败: %w", err)
	}

	// 存储到Redis，设置过期时间
	key := fmt.Sprintf("user_registry:%s", userID)
	err = r.redisClient.Set(r.ctx, key, data, 5*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("注册用户连接到Redis失败: %w", err)
	}

	// 同时添加到服务器用户集合中，便于服务器下线时批量清理
	serverUsersKey := fmt.Sprintf("server_users:%s", r.serverID)
	err = r.redisClient.SAdd(r.ctx, serverUsersKey, userID).Err()
	if err != nil {
		log.Printf("添加用户到服务器集合失败: %v", err)
	}

	log.Printf("用户 %s 已注册到服务器 %s", userID, r.serverID)
	return nil
}

// UnregisterUser 注销用户连接
func (r *UserConnectionRegistry) UnregisterUser(userID string) error {
	r.mutex.Lock()
	delete(r.localUsers, userID)
	r.mutex.Unlock()

	// 从Redis中删除用户连接信息
	key := fmt.Sprintf("user_registry:%s", userID)
	err := r.redisClient.Del(r.ctx, key).Err()
	if err != nil {
		log.Printf("从Redis删除用户连接信息失败: %v", err)
	}

	// 从服务器用户集合中删除
	serverUsersKey := fmt.Sprintf("server_users:%s", r.serverID)
	err = r.redisClient.SRem(r.ctx, serverUsersKey, userID).Err()
	if err != nil {
		log.Printf("从服务器集合删除用户失败: %v", err)
	}

	log.Printf("用户 %s 已从服务器 %s 注销", userID, r.serverID)
	return nil
}

// FindUserServer 查找用户所在的服务器
func (r *UserConnectionRegistry) FindUserServer(userID string) (*UserConnectionInfo, error) {
	// 首先检查是否在本地
	r.mutex.RLock()
	isLocal := r.localUsers[userID]
	r.mutex.RUnlock()

	if isLocal {
		return &UserConnectionInfo{
			UserID:   userID,
			ServerID: r.serverID,
		}, nil
	}

	// 从Redis查询用户连接信息
	key := fmt.Sprintf("user_registry:%s", userID)
	data, err := r.redisClient.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("用户 %s 不在线", userID)
		}
		return nil, fmt.Errorf("查询用户连接信息失败: %w", err)
	}

	var connInfo UserConnectionInfo
	err = json.Unmarshal([]byte(data), &connInfo)
	if err != nil {
		return nil, fmt.Errorf("解析用户连接信息失败: %w", err)
	}

	return &connInfo, nil
}

// IsUserLocal 检查用户是否在本地连接
func (r *UserConnectionRegistry) IsUserLocal(userID string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.localUsers[userID]
}

// GetLocalUsers 获取本地所有用户
func (r *UserConnectionRegistry) GetLocalUsers() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	users := make([]string, 0, len(r.localUsers))
	for userID := range r.localUsers {
		users = append(users, userID)
	}
	return users
}

// StartHeartbeat 启动心跳机制，定期更新用户连接状态
func (r *UserConnectionRegistry) StartHeartbeat() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			r.updateHeartbeat()
		}
	}()
}

// updateHeartbeat 更新心跳
func (r *UserConnectionRegistry) updateHeartbeat() {
	r.mutex.RLock()
	localUsers := make([]string, 0, len(r.localUsers))
	for userID := range r.localUsers {
		localUsers = append(localUsers, userID)
	}
	r.mutex.RUnlock()

	// 批量更新本地用户的心跳时间
	for _, userID := range localUsers {
		key := fmt.Sprintf("user_registry:%s", userID)
		// 刷新过期时间
		r.redisClient.Expire(r.ctx, key, 5*time.Minute)
	}

	if len(localUsers) > 0 {
		log.Printf("更新了 %d 个用户的心跳", len(localUsers))
	}
}

// CleanupServerUsers 清理服务器下线时的用户数据
func (r *UserConnectionRegistry) CleanupServerUsers() error {
	serverUsersKey := fmt.Sprintf("server_users:%s", r.serverID)

	// 获取该服务器的所有用户
	users, err := r.redisClient.SMembers(r.ctx, serverUsersKey).Result()
	if err != nil {
		return fmt.Errorf("获取服务器用户列表失败: %w", err)
	}

	// 删除所有用户的连接信息
	for _, userID := range users {
		userKey := fmt.Sprintf("user_registry:%s", userID)
		r.redisClient.Del(r.ctx, userKey)
	}

	// 删除服务器用户集合
	r.redisClient.Del(r.ctx, serverUsersKey)

	log.Printf("服务器 %s 下线，清理了 %d 个用户的连接信息", r.serverID, len(users))
	return nil
}
