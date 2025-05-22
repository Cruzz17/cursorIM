package user

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"cursorIM/internal/redisclient"

	"github.com/gin-gonic/gin"
)

// Register 处理用户注册
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	svc := NewAccountService()
	userID, err := svc.Register(c.Request.Context(), &req)
	if err != nil {
		log.Printf("注册错误: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "用户注册成功",
		"user_id": userID,
	})
}

// Login 处理用户登录
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("登录请求绑定错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("尝试登录用户: %s", req.Username)
	svc := NewAccountService()
	response, err := svc.Login(c.Request.Context(), &req)
	if err != nil {
		log.Printf("%s 登录失败: %v", req.Username, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	log.Printf("用户 %s 登录成功", req.Username)
	c.JSON(http.StatusOK, gin.H{"token": response.Token})
}

// GetUserInfo 获取用户信息
func GetUserInfo(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	svc := NewAccountService()
	user, err := svc.GetUserByID(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// SearchUsers 搜索用户
func SearchUsers(c *gin.Context) {
	// 支持 q 或 username 参数
	query := c.Query("q")
	if query == "" {
		query = c.Query("username")
	}

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索查询不能为空"})
		return
	}

	log.Printf("搜索用户，查询: %s", query)
	svc := NewAccountService()
	users, err := svc.SearchUsers(c.Request.Context(), query)
	if err != nil {
		log.Printf("搜索用户出错: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索用户失败"})
		return
	}

	log.Printf("找到 %d 个匹配查询 '%s' 的用户", len(users), query)
	c.JSON(http.StatusOK, users)
}

// AddFriend 处理添加好友请求
func AddFriend(c *gin.Context) {
	// 这里支持两种字段名：friendId 和 FriendID
	var req struct {
		FriendID string `json:"FriendID"`
		FriendId string `json:"friendId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 确定要使用的好友ID
	friendID := req.FriendID
	if friendID == "" {
		friendID = req.FriendId
	}

	if friendID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "好友ID不能为空"})
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	log.Printf("用户 %s 尝试添加好友 %s", userID, friendID)
	svc := NewAccountService()
	err := svc.AddFriend(c.Request.Context(), userID.(string), friendID)
	if err != nil {
		log.Printf("添加好友失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("好友 %s 已成功添加到用户 %s", friendID, userID)
	c.JSON(http.StatusOK, gin.H{"message": "好友添加成功"})
}

// GetFriends 获取好友列表
func GetFriends(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	log.Printf("获取用户 %s 的好友列表", userID)
	svc := NewAccountService()
	friends, err := svc.GetFriends(c.Request.Context(), userID.(string))
	if err != nil {
		log.Printf("获取好友列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取好友列表失败"})
		return
	}

	log.Printf("为用户 %s 找到 %d 个好友", userID, len(friends))
	c.JSON(http.StatusOK, friends)
}

// Heartbeat 处理心跳请求，用于检测用户在线状态
func Heartbeat(c *gin.Context) {
	// 从上下文获取用户ID
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	// 获取客户端IP和用户代理
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// 创建设备信息
	deviceInfo := map[string]string{
		"user_agent": userAgent,
		"ip":         clientIP,
	}

	// 构建连接信息
	connectionInfo := map[string]interface{}{
		"http": map[string]interface{}{
			"last_heartbeat": time.Now().Unix(),
			"status":         "online",
			"device_info":    deviceInfo,
		},
		"websocket": false, // 默认HTTP连接没有同时建立WebSocket
	}

	// 获取Redis客户端
	rdb := redisclient.GetRedisClient()
	ctx := context.Background()

	// 检查用户是否有WebSocket连接
	wsKey := fmt.Sprintf("conn:%s:websocket", userID)
	wsExists, err := rdb.Exists(ctx, wsKey).Result()
	if err == nil && wsExists > 0 {
		connectionInfo["websocket"] = true
	}

	// 序列化连接信息
	jsonData, err := json.Marshal(connectionInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化数据失败"})
		return
	}

	// 更新Redis中的连接信息
	userConnKey := fmt.Sprintf("user:%s:connections", userID)
	lastActiveKey := fmt.Sprintf("user:%s:last_active", userID)

	// 使用管道批量操作
	pipe := rdb.Pipeline()
	pipe.Set(ctx, userConnKey, jsonData, 10*time.Minute) // 10分钟过期
	pipe.Set(ctx, lastActiveKey, time.Now().Unix(), 10*time.Minute)
	pipe.SAdd(ctx, "online_users", userID) // 添加到在线用户集合
	_, err = pipe.Exec(ctx)

	if err != nil {
		log.Printf("更新用户连接信息失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新状态失败"})
		return
	}

	// 获取当前系统时间戳
	currentTime := time.Now().Unix()

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"status": func() string {
			if wsExists > 0 {
				return "online"
			}
			return "http_only"
		}(),
		"timestamp":   currentTime,
		"user_id":     userID,
		"connections": connectionInfo,
	})
}
