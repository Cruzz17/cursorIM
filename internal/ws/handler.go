package ws

import (
	"cursorIM/internal/protocol"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// HandleWebSocket 处理WebSocket连接请求
func HandleWebSocket(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文中获取用户ID
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
			return
		}

		// 升级HTTP连接为WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Error upgrading to websocket: %v", err)
			return
		}

		// 创建新的客户端
		client := &Client{
			Hub:    hub,
			conn:   conn,
			send:   make(chan *protocol.Message, 256),
			UserID: userID.(string),
		}

		// 注册客户端
		client.Hub.register <- client

		// 启动处理例程
		go client.writePump()
		go client.readPump()
	}
}

// ServeWs 处理WebSocket连接
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, userID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{
		Hub:    hub,
		conn:   conn,
		send:   make(chan *protocol.Message, 256),
		UserID: userID,
	}
	client.Hub.register <- client

	// 允许收集未使用的连接
	go client.writePump()
	go client.readPump()
}
