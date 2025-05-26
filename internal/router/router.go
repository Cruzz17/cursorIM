package router

import (
	"cursorIM/internal/chat"
	"cursorIM/internal/connection"
	"cursorIM/internal/group"
	"cursorIM/internal/middleware"
	"cursorIM/internal/server"
	"cursorIM/internal/user"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"bytes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// bodyLogWriter 用于捕获响应体
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// SetupRouter 配置所有路由
func SetupRouter(connMgr connection.ConnectionManager, messageService *chat.MessageService) *gin.Engine {
	r := gin.Default()

	// CORS 配置
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API请求日志中间件
	r.Use(func(c *gin.Context) {
		// 获取请求ID，方便跟踪
		requestID := uuid.New().String()
		c.Set("requestID", requestID)

		// 记录请求体
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = ioutil.ReadAll(c.Request.Body)
			// 设置回请求体，因为读取后需要重置
			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 捕获响应
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		// 请求开始时间
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 请求结束后记录
		latency := time.Since(startTime)

		// 记录请求和响应信息
		log.Printf("[%s] 请求: %s %s, 状态: %d, 延迟: %s",
			requestID, c.Request.Method, c.Request.URL.Path, c.Writer.Status(), latency)

		// 记录请求头
		log.Printf("[%s] 请求头: %v", requestID, c.Request.Header)

		// 记录请求体
		if len(requestBody) > 0 {
			log.Printf("[%s] 请求体: %s", requestID, string(requestBody))
		}

		// 记录响应体 (限制大小，避免日志过大)
		respBody := blw.body.String()
		if len(respBody) > 1000 {
			log.Printf("[%s] 响应体: %s... (截断，总长度: %d)", requestID, respBody[:1000], len(respBody))
		} else {
			log.Printf("[%s] 响应体: %s", requestID, respBody)
		}
	})

	// 静态文件服务 - 为测试页面提供支持
	r.StaticFile("/test_websocket.html", "./test_websocket.html")
	r.StaticFile("/debug_connections.html", "./debug_connections.html")

	// API 路由
	api := r.Group("/api")
	{
		// ----- 无需认证的路由 -----
		api.POST("/register", user.Register)
		api.POST("/login", user.Login)

		//心跳检测
		api.OPTIONS("/heartbeat", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})

		// WebSocket路由 - 直接在api组中，不经过JWT中间件
		api.GET("/ws", server.WebSocketHandler(connMgr, messageService, false)) // 标准WebSocket

		// ----- 需要认证的路由 -----
		auth := api.Group("/")
		auth.Use(middleware.JWT())
		{
			// ----- 用户相关 -----
			auth.GET("/user/info", user.GetUserInfo)

			// 用户搜索 - 支持多种路径
			userSearchRoutes := []string{"/user/search", "/users/search"}
			for _, route := range userSearchRoutes {
				auth.GET(route, user.SearchUsers)
			}

			// ----- 好友相关 -----

			// 添加好友 - 支持多种路径
			friendAddRoutes := []string{"/friend/add", "/friends"}
			for _, route := range friendAddRoutes {
				auth.POST(route, user.AddFriend)
			}

			// 获取好友列表 - 支持多种路径
			friendGetRoutes := []string{"/friends", "/friend/list"}
			for _, route := range friendGetRoutes {
				auth.GET(route, user.GetFriends)
			}

			// ----- 群组相关 -----
			// 创建群组
			auth.POST("/group/create", group.CreateGroup)

			// 邀请用户入群
			auth.POST("/group/:groupId/invite", group.InviteUser)

			// 退出群组
			auth.POST("/group/:groupId/exit", group.ExitGroup)

			// 获取群成员列表
			auth.GET("/group/:groupId/members", group.GetGroupMembers)

			// 获取用户所在群组列表
			auth.GET("/groups", group.GetUserGroups)

			// 更新群名称
			auth.PUT("/group/:groupId/name", group.UpdateGroupName)

			// 解散群组
			auth.DELETE("/group/:groupId", group.DeleteGroup)

			// ----- 会话相关 -----

			// 获取会话列表 - 支持多种路径
			conversationGetRoutes := []string{"/conversations", "/conversation/list"}
			for _, route := range conversationGetRoutes {
				auth.GET(route, chat.GetConversations)
			}

			// 创建会话 - 支持多种路径
			conversationCreateRoutes := []string{"/conversation", "/conversations"}
			for _, route := range conversationCreateRoutes {
				auth.POST(route, chat.CreateConversation)
			}

			// 获取单个会话 - 支持多种路径格式
			auth.GET("/conversation/:id", chat.GetConversation)
			auth.GET("/conversations/:id", chat.GetConversation)
			auth.GET("/conversations/:id/participants", chat.GetParticipants)

			// ----- 消息相关 -----
			auth.GET("/messages/:conversationId", chat.GetMessages)
			auth.POST("/messages/:id/read", chat.MarkMessagesAsRead)

			// 获取与特定用户的消息
			auth.GET("/messages/user/:user_id", func(c *gin.Context) {
				userID, _ := c.Get("userID")
				otherUserID := c.Param("user_id")
				log.Printf("获取用户 %s 和 %s 之间的消息", userID, otherUserID)

				msgService := chat.NewMessageService()
				messages, err := msgService.GetPrivateMessages(c.Request.Context(), userID.(string), otherUserID, 50)
				if err != nil {
					log.Printf("获取消息失败: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "获取消息失败"})
					return
				}

				log.Printf("找到用户 %s 和 %s 之间的 %d 条消息", userID, otherUserID, len(messages))
				c.JSON(http.StatusOK, messages)
			})

			// 获取群组消息
			auth.GET("/messages/group/:group_id", func(c *gin.Context) {
				groupID := c.Param("group_id")
				log.Printf("获取群组 %s 的消息", groupID)

				msgService := chat.NewMessageService()
				messages, err := msgService.GetGroupMessages(c.Request.Context(), groupID, 50)
				if err != nil {
					log.Printf("获取群组消息失败: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "获取群组消息失败"})
					return
				}

				log.Printf("找到群组 %s 的 %d 条消息", groupID, len(messages))
				c.JSON(http.StatusOK, messages)
			})

			// 心跳检测
			auth.GET("/heartbeat", user.Heartbeat)
		}
	}

	return r
}
