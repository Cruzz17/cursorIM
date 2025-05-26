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

	"cursorIM/internal/config"
	"cursorIM/internal/connection"
	"cursorIM/internal/database"
	"cursorIM/internal/redisclient"
	"cursorIM/internal/router"
	"cursorIM/internal/server"
	"cursorIM/internal/service"

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

	// 创建优化的连接管理器（支持协议适配）
	connMgr := connection.NewOptimizedConnectionManager("server-1", "localhost:8082")

	// 启动连接管理器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go connMgr.Run(ctx)

	// 创建统一服务管理器
	serviceMgr := service.NewManager(context.Background(), connMgr)

	// 启动增强的 TCP 服务器（支持 Protobuf 协议）
	enhancedTCPServer := server.NewEnhancedTCPServer(":8083", connMgr, serviceMgr.GetChatService())
	if err := enhancedTCPServer.Start(); err != nil {
		log.Fatalf("启动增强 TCP 服务器失败: %v", err)
	}
	defer enhancedTCPServer.Stop()

	// 设置 Gin 路由
	r := router.SetupRouter(connMgr, serviceMgr.GetChatService())

	// 添加增强的 WebSocket 路由（支持协议适配）
	r.GET("/api/ws", server.EnhancedWebSocketHandler(connMgr, serviceMgr.GetChatService(), false))
	r.GET("/api/ws-tcp", server.EnhancedWebSocketHandler(connMgr, serviceMgr.GetChatService(), true))

	// 启动 HTTP/HTTPS 服务器 (WebSocket)
	httpServerPort := config.GlobalConfig.Server.Port
	httpServer := startHTTPServer(r, httpServerPort)

	// 打印协议支持信息
	log.Println("协议支持:")
	log.Println("  - Web端: JSON over WebSocket")
	log.Println("  - App端: Protobuf over TCP/WebSocket")
	log.Printf("  - WebSocket (JSON): ws://localhost:%d/api/ws", httpServerPort)
	log.Printf("  - WebSocket (Protobuf): ws://localhost:%d/api/ws-tcp", httpServerPort)
	log.Println("  - TCP (Protobuf): localhost:8083")

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	// 关闭 HTTP 服务器
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP 服务器关闭失败: %v", err)
	}

	// 关闭增强的 TCP 服务器
	if err := enhancedTCPServer.Stop(); err != nil {
		log.Fatalf("增强 TCP 服务器关闭失败: %v", err)
	}

	// 关闭服务管理器
	serviceMgr.Shutdown()

	// 关闭连接管理器
	if err := connMgr.Close(); err != nil {
		log.Fatalf("连接管理器关闭失败: %v", err)
	}

	log.Println("服务器已安全关闭")
}

// startHTTPServer 启动 HTTP/HTTPS 服务器
func startHTTPServer(r *gin.Engine, port int) *http.Server {
	portStr := ":" + strconv.Itoa(port)

	// 检查是否启用TLS
	enableTLS := os.Getenv("ENABLE_TLS") == "true"
	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")

	// 如果没有设置环境变量，使用默认值
	if certFile == "" {
		certFile = "./certs/server.crt"
	}
	if keyFile == "" {
		keyFile = "./certs/server.key"
	}

	// 创建TLS配置
	tlsConfig := server.NewTLSConfig(certFile, keyFile, enableTLS)

	// 验证证书（如果启用TLS）
	if enableTLS {
		if err := tlsConfig.ValidateCertificates(); err != nil {
			log.Printf("⚠️ TLS证书验证失败: %v", err)
			log.Printf("💡 提示: 运行 './scripts/generate_certs.sh' 生成开发证书")
			log.Printf("🔄 回退到HTTP模式...")
			enableTLS = false
			tlsConfig = server.NewTLSConfig("", "", false)
		}
	}

	srv := &http.Server{
		Addr:    portStr,
		Handler: r,
	}

	go func() {
		var err error
		if enableTLS {
			log.Printf("🔐 HTTPS服务器已启动，监听端口 %d", port)
			log.Printf("🌐 访问地址: https://localhost%s", portStr)
			log.Printf("📄 使用证书: %s", certFile)
			err = srv.ListenAndServeTLS(certFile, keyFile)
		} else {
			log.Printf("🌐 HTTP服务器已启动，监听端口 %d", port)
			log.Printf("🌐 访问地址: http://localhost%s", portStr)
			err = srv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	return srv
}
