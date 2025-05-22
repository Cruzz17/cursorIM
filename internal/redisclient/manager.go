package redisclient

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

var (
	redisClient *redis.Client
)

// InitRedis 初始化Redis连接
func InitRedis(addr, password string, db int) error {
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
		return err
	}

	log.Println("Redis连接成功")
	return nil
}

// GetRedisClient 获取Redis客户端实例
func GetRedisClient() *redis.Client {
	return redisClient
}

// CloseRedis 关闭Redis连接
func CloseRedis() error {
	if redisClient != nil {
		return redisClient.Close()
	}
	return nil
}
