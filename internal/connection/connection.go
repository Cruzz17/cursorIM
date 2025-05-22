package connection

import (
	"context"
	"cursorIM/internal/protocol"
	"fmt"
	"time"
)

// Connection 表示一个与客户端的连接，可以是 WebSocket 或 TCP
type Connection interface {
	// SendMessage 发送消息到客户端
	SendMessage(message *protocol.Message) error

	// Close 关闭连接
	Close() error

	// GetUserID 获取用户 ID
	GetUserID() string

	// GetConnectionType 获取连接类型
	GetConnectionType() string

	// GetDoneChan 获取完成通道
	GetDoneChan() <-chan struct{}

	// GetSendChannel 获取发送通道
	GetSendChannel() <-chan *protocol.Message
}

// ConnectionType constants - keeping these in one place
const (
	ConnectionTypeWebSocket = "websocket"
	ConnectionTypeTCP       = "tcp"
	ConnectionTypeTCPWS     = "tcp_ws"
)

// Connection timeout and heartbeat constants
const (
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second
	PingPeriod     = (PongWait * 9) / 10
	MaxMessageSize = 10000
)

// Common error definitions
var (
	ErrConnectionBufferFull = fmt.Errorf("connection buffer full")
)

// ConnectionManager 负责管理所有连接和消息转发
type ConnectionManager interface {
	// RegisterConnection 注册一个新的连接
	RegisterConnection(userID string, conn Connection) error

	// UnregisterConnection 注销一个连接
	UnregisterConnection(userID string, connType string) error

	// SendMessage 发送消息
	SendMessage(message *protocol.Message) error

	// Run 启动连接管理器
	Run(ctx context.Context)

	// Close 关闭连接管理器
	Close() error
}
