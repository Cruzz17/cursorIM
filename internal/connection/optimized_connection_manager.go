package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"cursorIM/internal/protocol"
	"cursorIM/internal/redisclient"
	"cursorIM/internal/status"

	"github.com/go-redis/redis/v8"
)

// OptimizedConnectionManager 使用路由表机制的优化连接管理器
type OptimizedConnectionManager struct {
	redisClient      *redis.Client
	redisEnabled     bool
	connections      map[string]map[string]Connection // 用户ID -> 连接ID -> 连接
	messageQueueChan chan *protocol.Message
	statusManager    *status.Manager
	userRegistry     *UserConnectionRegistry // 用户连接路由表
	serverID         string                  // 当前服务器ID
	serverAddr       string                  // 当前服务器地址
	mutex            sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewOptimizedConnectionManager 创建优化的连接管理器
func NewOptimizedConnectionManager(serverID, serverAddr string) *OptimizedConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())

	redisClient := redisclient.GetRedisClient()
	redisEnabled := redisclient.IsRedisEnabled()

	statusMgr := status.NewManager(ctx)

	// 创建用户连接路由表
	userRegistry := NewUserConnectionRegistry(redisClient, serverID, serverAddr)

	if !redisEnabled {
		log.Println("[Optimized] running in memory-only mode")
	} else {
		log.Printf("[Optimized] connection established successfully")
	}

	manager := &OptimizedConnectionManager{
		redisClient:      redisClient,
		redisEnabled:     redisEnabled,
		connections:      make(map[string]map[string]Connection),
		messageQueueChan: make(chan *protocol.Message, 1000),
		statusManager:    statusMgr,
		userRegistry:     userRegistry,
		serverID:         serverID,
		serverAddr:       serverAddr,
		ctx:              ctx,
		cancel:           cancel,
	}

	// 启动路由表心跳
	if redisEnabled {
		userRegistry.StartHeartbeat()
	}

	return manager
}

// RegisterConnection 注册连接（优化版）
func (m *OptimizedConnectionManager) RegisterConnection(userID string, conn Connection) error {
	connType := conn.GetConnectionType()
	connID := fmt.Sprintf("%s_%s_%d", userID, connType, time.Now().UnixNano())

	// 更新本地连接映射
	m.mutex.Lock()
	if _, ok := m.connections[userID]; !ok {
		m.connections[userID] = make(map[string]Connection)
	}
	m.connections[userID][connID] = conn
	m.mutex.Unlock()

	// 注册到路由表
	if m.redisEnabled {
		if err := m.userRegistry.RegisterUser(userID, connType); err != nil {
			log.Printf("注册用户到路由表失败: %v", err)
		}
	}

	// 更新用户状态
	if err := m.statusManager.UpdateUserStatus(userID, connType, true); err != nil {
		log.Printf("更新用户 %s 的在线状态失败: %v", userID, err)
	}

	log.Printf("[Optimized] 用户 %s 的 %s 连接已注册到服务器 %s", userID, connType, m.serverID)

	// 发送离线消息
	go m.sendOfflineMessages(userID)

	return nil
}

// UnregisterConnection 注销连接（优化版）
func (m *OptimizedConnectionManager) UnregisterConnection(userID string, connType string) error {
	m.mutex.Lock()
	var connsToClose []Connection

	if userConns, ok := m.connections[userID]; ok {
		var connIDsToRemove []string
		for connID, conn := range userConns {
			if conn.GetConnectionType() == connType {
				connsToClose = append(connsToClose, conn)
				connIDsToRemove = append(connIDsToRemove, connID)
			}
		}

		for _, connID := range connIDsToRemove {
			delete(userConns, connID)
		}

		if len(userConns) == 0 {
			delete(m.connections, userID)
		}
	}
	m.mutex.Unlock()

	// 关闭连接
	for _, conn := range connsToClose {
		if conn != nil {
			_ = conn.Close()
		}
	}

	// 从路由表注销
	if m.redisEnabled {
		// 检查用户是否还有其他连接
		m.mutex.RLock()
		hasOtherConns := len(m.connections[userID]) > 0
		m.mutex.RUnlock()

		if !hasOtherConns {
			if err := m.userRegistry.UnregisterUser(userID); err != nil {
				log.Printf("从路由表注销用户失败: %v", err)
			}
		}
	}

	// 更新状态
	m.mutex.RLock()
	hasOtherConns := len(m.connections[userID]) > 0
	m.mutex.RUnlock()

	if !hasOtherConns {
		if err := m.statusManager.UpdateUserStatus(userID, connType, false); err != nil {
			log.Printf("更新用户 %s 的离线状态失败: %v", userID, err)
		}
	}

	log.Printf("[Optimized] 用户 %s 的 %s 连接已从服务器 %s 注销", userID, connType, m.serverID)
	return nil
}

// SendMessage 发送消息（优化版 - 使用路由表）
func (m *OptimizedConnectionManager) SendMessage(message *protocol.Message) error {
	// 检查是否是本地用户
	if m.userRegistry.IsUserLocal(message.RecipientID) {
		// 本地用户，直接放入处理队列
		select {
		case m.messageQueueChan <- message:
			return nil
		default:
			return fmt.Errorf("消息队列已满")
		}
	}

	// 非本地用户，查找目标服务器
	if !m.redisEnabled {
		// 无Redis，存储为离线消息
		return m.storeOfflineMessage(message)
	}

	connInfo, err := m.userRegistry.FindUserServer(message.RecipientID)
	if err != nil {
		log.Printf("查找用户 %s 的服务器失败: %v", message.RecipientID, err)
		// 用户不在线，存储为离线消息
		return m.storeOfflineMessage(message)
	}

	// 发送到目标服务器
	return m.sendToTargetServer(message, connInfo.ServerID)
}

// sendToTargetServer 发送消息到目标服务器
func (m *OptimizedConnectionManager) sendToTargetServer(message *protocol.Message, targetServerID string) error {
	// 使用专用的服务器间通信频道
	channel := fmt.Sprintf("server_msg:%s", targetServerID)

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	err = m.redisClient.Publish(m.ctx, channel, msgBytes).Err()
	if err != nil {
		log.Printf("发送消息到服务器 %s 失败: %v", targetServerID, err)
		// 发送失败，存储为离线消息
		return m.storeOfflineMessage(message)
	}

	log.Printf("[Optimized] 消息已路由到服务器 %s", targetServerID)
	return nil
}

// processMessage 处理消息（优化版）
func (m *OptimizedConnectionManager) processMessage(message *protocol.Message) {
	recipientID := message.RecipientID
	senderID := message.SenderID

	log.Printf("[Optimized] 处理消息: %s -> %s", senderID, recipientID)

	// 检查接收者是否在本地
	m.mutex.RLock()
	userConns, ok := m.connections[recipientID]
	m.mutex.RUnlock()

	if !ok {
		log.Printf("警告: 接收者 %s 不在本服务器上", recipientID)
		// 这种情况不应该发生，因为路由表已经确保消息只发送到正确的服务器
		return
	}

	messageSent := false

	// 发送到本地连接
	var connTypes []string
	var conns []Connection

	m.mutex.RLock()
	for connType, conn := range userConns {
		connTypes = append(connTypes, connType)
		conns = append(conns, conn)
	}
	m.mutex.RUnlock()

	// 尝试发送消息
	for i, conn := range conns {
		err := conn.SendMessage(message)
		if err != nil {
			log.Printf("发送消息到用户 %s 的 %s 连接失败: %v", recipientID, connTypes[i], err)
			if err.Error() == "连接已关闭" {
				m.UnregisterConnection(recipientID, connTypes[i])
			}
		} else {
			log.Printf("[Optimized] 消息已发送到用户 %s", recipientID)
			messageSent = true
			break
		}
	}

	if !messageSent {
		log.Printf("发送消息失败，存储为离线消息")
		m.storeOfflineMessage(message)
	}
}

// Run 启动优化的连接管理器
func (m *OptimizedConnectionManager) Run(ctx context.Context) {
	defer m.Close()

	// 启动服务器间消息监听
	if m.redisEnabled {
		go m.startServerMessageListener()
	}

	// 处理消息队列
	for {
		select {
		case <-ctx.Done():
			log.Println("[Optimized] 连接管理器关闭中...")
			return
		case message := <-m.messageQueueChan:
			m.processMessage(message)
		}
	}
}

// startServerMessageListener 启动服务器间消息监听
func (m *OptimizedConnectionManager) startServerMessageListener() {
	// 订阅当前服务器的专用频道
	channel := fmt.Sprintf("server_msg:%s", m.serverID)
	pubsub := m.redisClient.Subscribe(m.ctx, channel)
	defer pubsub.Close()

	log.Printf("[Optimized] 开始监听服务器频道: %s", channel)

	ch := pubsub.Channel()
	for msg := range ch {
		var message protocol.Message
		if err := json.Unmarshal([]byte(msg.Payload), &message); err != nil {
			log.Printf("解析服务器间消息失败: %v", err)
			continue
		}

		// 将消息放入处理队列
		select {
		case m.messageQueueChan <- &message:
			log.Printf("[Optimized] 收到服务器间消息: %s -> %s", message.SenderID, message.RecipientID)
		default:
			log.Printf("消息队列已满，丢弃服务器间消息")
		}
	}
}

// 其他方法保持相同...
func (m *OptimizedConnectionManager) sendOfflineMessages(userID string) {
	// ... 同 redis_manager.go 中的实现
}

func (m *OptimizedConnectionManager) storeOfflineMessage(message *protocol.Message) error {
	// ... 同 redis_manager.go 中的实现
	return nil
}

func (m *OptimizedConnectionManager) GetOfflineMessages(userID string) ([]*protocol.Message, error) {
	// ... 同 redis_manager.go 中的实现
	return nil, nil
}

func (m *OptimizedConnectionManager) MarkOfflineMessagesAsSent(userID string, messages []*protocol.Message) error {
	// ... 同 redis_manager.go 中的实现
	return nil
}

func (m *OptimizedConnectionManager) Close() error {
	m.cancel()

	// 清理路由表
	if m.redisEnabled {
		m.userRegistry.CleanupServerUsers()
	}

	// 关闭所有连接
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, userConns := range m.connections {
		for _, conn := range userConns {
			conn.Close()
		}
	}

	m.connections = make(map[string]map[string]Connection)

	if m.redisClient != nil {
		return m.redisClient.Close()
	}

	return nil
}
