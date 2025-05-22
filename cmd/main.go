package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cursorIM/internal/chat"
	"cursorIM/internal/config"
	"cursorIM/internal/connection"
	"cursorIM/internal/database"
	"cursorIM/internal/redisclient"
	"cursorIM/internal/router"
	"cursorIM/internal/server"
	"cursorIM/internal/ws"

	"github.com/gin-gonic/gin"
)

func main() {
	// 读取配置
	if err := config.Init(); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	db, err := database.InitDB()
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("获取数据库连接失败: %v", err)
	}
	defer sqlDB.Close()

	log.Println("数据库初始化成功")

	// 从配置中获取 Redis 地址
	redisConfig := config.GlobalConfig.Redis
	redisAddr := fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port)
	log.Printf("连接Redis: %s, 数据库: %d", redisAddr, redisConfig.DB)

	// 初始化Redis
	if err := redisclient.InitRedis(redisAddr, redisConfig.Password, redisConfig.DB); err != nil {
		log.Printf("警告: Redis 初始化失败: %v", err)
		log.Printf("系统将在无Redis的情况下继续运行，但某些功能可能不可用")
	} else {
		log.Println("Redis 初始化成功")
	}

	// 创建消息服务
	messageService := chat.NewMessageService()

	// 创建Redis连接管理器（使用全局配置）
	connMgr := connection.NewRedisConnectionManager()

	// 设置消息服务的连接管理器
	messageService.SetConnectionManager(connMgr)

	// 启动连接管理器
	go connMgr.Run(context.Background())

	// 创建 TCP 服务器
	tcpServer := server.NewTCPServer(":8083", connMgr, messageService)
	if err := tcpServer.Start(); err != nil {
		log.Fatalf("启动 TCP 服务器失败: %v", err)
	}

	// 创建 WebSocket Hub
	hub := ws.NewHub()

	// 设置 Gin 路由
	r := router.SetupRouter(hub, connMgr, messageService)

	// 启动 HTTP 服务器 (WebSocket)
	httpServerPort := config.GlobalConfig.Server.Port
	httpServer := startHTTPServer(r, httpServerPort)

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	// 关闭 HTTP 服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP 服务器关闭失败: %v", err)
	}

	// 关闭 TCP 服务器
	if err := tcpServer.Stop(); err != nil {
		log.Fatalf("TCP 服务器关闭失败: %v", err)
	}

	// 关闭连接管理器
	if err := connMgr.Close(); err != nil {
		log.Fatalf("连接管理器关闭失败: %v", err)
	}

	log.Println("服务器已安全关闭")
}

// startHTTPServer 启动 HTTP 服务器
func startHTTPServer(r *gin.Engine, port int) *http.Server {
	// Fix: Convert port to string with proper format ":port"
	portStr := ":" + strconv.Itoa(port)

	srv := &http.Server{
		Addr:    portStr,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务器启动失败: %v", err)
		}
	}()

	log.Printf("HTTP 服务器已启动，监听端口 %d", port)
	return srv
}
