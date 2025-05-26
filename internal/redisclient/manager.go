package redisclient

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	// redisClient 是全局Redis客户端实例
	redisClient *redis.Client

	// 保护全局变量的互斥锁
	mutex sync.RWMutex

	// redisEnabled 标记Redis是否可用
	redisEnabled bool
)

// InitRedis 初始化Redis连接
func InitRedis(addr, password string, db int) error {
	mutex.Lock()
	defer mutex.Unlock()

	// 关闭之前的连接（如果存在）
	if redisClient != nil {
		redisClient.Close()
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Printf("Redis连接失败: %v", err)
		redisEnabled = false
		return err
	}

	log.Println("Redis连接成功")
	redisEnabled = true
	return nil
}

// GetRedisClient 获取Redis客户端实例
func GetRedisClient() *redis.Client {
	mutex.RLock()
	defer mutex.RUnlock()
	return redisClient
}

// IsRedisEnabled 检查Redis是否启用
func IsRedisEnabled() bool {
	mutex.RLock()
	defer mutex.RUnlock()
	return redisEnabled && redisClient != nil
}

// CloseRedis 关闭Redis连接
func CloseRedis() error {
	mutex.Lock()
	defer mutex.Unlock()

	if redisClient != nil {
		err := redisClient.Close()
		redisClient = nil
		redisEnabled = false
		return err
	}
	return nil
}
