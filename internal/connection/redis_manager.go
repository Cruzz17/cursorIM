package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"cursorIM/internal/database"
	"cursorIM/internal/model"
	"cursorIM/internal/protocol"
	"cursorIM/internal/redisclient"
	"cursorIM/internal/status"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// RedisConnectionManager 使用 Redis 实现的连接管理器
type RedisConnectionManager struct {
	redisClient          *redis.Client
	redisEnabled         bool
	connections          map[string]map[string]Connection // 用户ID -> 连接ID -> 连接
	connectionsByType    map[string]map[string]Connection // 连接类型 -> 用户ID -> 连接 (保留最新连接引用)
	messageQueueChan     chan *protocol.Message
	connectionUpdateChan chan struct{}
	statusManager        *status.Manager // 状态管理器
	mutex                sync.RWMutex
	ctx                  context.Context
	cancel               context.CancelFunc
}

// NewRedisConnectionManager 创建新的 Redis 连接管理器
func NewRedisConnectionManager() *RedisConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())

	// 使用统一的Redis客户端
	redisClient := redisclient.GetRedisClient()
	redisEnabled := redisclient.IsRedisEnabled()

	// 创建状态管理器
	statusMgr := status.NewManager(ctx)

	if !redisEnabled {
		log.Println("[Redis] running in memory-only mode")
	} else {
		log.Printf("[Redis] connection established successfully")
	}

	return &RedisConnectionManager{
		redisClient:          redisClient,
		redisEnabled:         redisEnabled,
		connections:          make(map[string]map[string]Connection),
		connectionsByType:    make(map[string]map[string]Connection),
		messageQueueChan:     make(chan *protocol.Message, 1000),
		connectionUpdateChan: make(chan struct{}, 100),
		statusManager:        statusMgr,
		ctx:                  ctx,
		cancel:               cancel,
	}
}

// RegisterConnection 注册一个新的连接
func (m *RedisConnectionManager) RegisterConnection(userID string, conn Connection) error {
	connType := conn.GetConnectionType()

	// 生成连接ID
	connID := fmt.Sprintf("%s_%s_%d", userID, connType, time.Now().UnixNano())

	// 更新本地连接映射
	m.mutex.Lock()

	// 初始化用户连接映射
	if _, ok := m.connections[userID]; !ok {
		m.connections[userID] = make(map[string]Connection)
	}
	m.connections[userID][connID] = conn

	// 初始化类型连接映射（保留最新连接引用）
	if _, ok := m.connectionsByType[connType]; !ok {
		m.connectionsByType[connType] = make(map[string]Connection)
	}
	m.connectionsByType[connType][userID] = conn

	m.mutex.Unlock()

	// 如果 Redis 启用，更新 Redis 中的连接信息
	if m.redisEnabled {
		connInfo := map[string]interface{}{
			"user_id":         userID,
			"connection_type": connType,
			"last_active":     time.Now().Unix(),
		}

		// 序列化连接信息
		connInfoBytes, err := json.Marshal(connInfo)
		if err != nil {
			return fmt.Errorf("序列化连接信息失败: %w", err)
		}

		// 存储到 Redis
		key := fmt.Sprintf("conn:%s:%s", userID, connType)
		err = m.redisClient.Set(m.ctx, key, connInfoBytes, 30*time.Minute).Err()
		if err != nil {
			log.Printf("存储连接信息到 Redis 失败: %v", err)
			// 不返回错误，继续使用内存模式
		} else {
			// 添加到用户连接集合
			err = m.redisClient.SAdd(m.ctx, fmt.Sprintf("user_conns:%s", userID), connType).Err()
			if err != nil {
				log.Printf("添加连接类型到用户集合失败: %v", err)
			}

			// 添加到在线用户集合
			err = m.redisClient.SAdd(m.ctx, fmt.Sprintf("online_users:%s", connType), userID).Err()
			if err != nil {
				log.Printf("添加用户到在线集合失败: %v", err)
			}
		}
	}

	// 更新用户状态为在线
	if err := m.statusManager.UpdateUserStatus(userID, connType, true); err != nil {
		log.Printf("更新用户 %s 的在线状态失败: %v", userID, err)
	}

	// 触发连接更新
	select {
	case m.connectionUpdateChan <- struct{}{}:
	default:
	}

	log.Printf("用户 %s 的 %s 连接已注册", userID, connType)

	// 用户上线后，发送离线消息
	go m.sendOfflineMessages(userID)

	return nil
}

// sendOfflineMessages 发送离线消息
func (m *RedisConnectionManager) sendOfflineMessages(userID string) {
	// 从数据库获取离线消息
	offlineMessages, err := m.GetOfflineMessages(userID)
	if err != nil {
		log.Printf("获取用户 %s 的离线消息失败: %v", userID, err)
		return
	}

	if len(offlineMessages) == 0 {
		log.Printf("用户 %s 没有离线消息", userID)
		return
	}

	log.Printf("为用户 %s 发送 %d 条离线消息", userID, len(offlineMessages))

	// 发送离线消息
	for _, msg := range offlineMessages {
		err := m.SendMessage(msg)
		if err != nil {
			log.Printf("发送离线消息失败: %v", err)
			continue
		}
	}

	// 标记消息为已发送
	err = m.MarkOfflineMessagesAsSent(userID, offlineMessages)
	if err != nil {
		log.Printf("标记离线消息为已发送失败: %v", err)
	} else {
		log.Printf("用户 %s 的 %d 条离线消息已标记为已发送", userID, len(offlineMessages))
	}
}

// UnregisterConnection 注销一个连接
func (m *RedisConnectionManager) UnregisterConnection(userID string, connType string) error {
	// 更新本地连接映射
	m.mutex.Lock()
	var connsToClose []Connection

	if userConns, ok := m.connections[userID]; ok {
		// 遍历用户的所有连接，找到匹配类型的连接
		var connIDsToRemove []string
		for connID, conn := range userConns {
			if conn.GetConnectionType() == connType {
				connsToClose = append(connsToClose, conn)
				connIDsToRemove = append(connIDsToRemove, connID)
			}
		}

		// 删除找到的连接
		for _, connID := range connIDsToRemove {
			delete(userConns, connID)
		}

		if len(userConns) == 0 {
			delete(m.connections, userID)
		}
	}

	// 从类型映射中删除
	if typeConns, ok := m.connectionsByType[connType]; ok {
		delete(typeConns, userID)
	}
	m.mutex.Unlock()

	// 在锁外安全地关闭连接
	for _, conn := range connsToClose {
		if conn != nil {
			// 忽略关闭错误，因为连接可能已经关闭
			_ = conn.Close()
		}
	}

	// 如果 Redis 启用，从 Redis 中删除连接信息
	if m.redisEnabled {
		// 从 Redis 中删除连接信息
		key := fmt.Sprintf("conn:%s:%s", userID, connType)
		err := m.redisClient.Del(m.ctx, key).Err()
		if err != nil {
			log.Printf("从 Redis 删除连接信息失败: %v", err)
		}

		// 从用户连接集合中删除
		err = m.redisClient.SRem(m.ctx, fmt.Sprintf("user_conns:%s", userID), connType).Err()
		if err != nil {
			log.Printf("从用户集合删除连接类型失败: %v", err)
		}

		// 从在线用户集合中删除
		err = m.redisClient.SRem(m.ctx, fmt.Sprintf("online_users:%s", connType), userID).Err()
		if err != nil {
			log.Printf("从在线集合删除用户失败: %v", err)
		}
	}

	// 检查用户是否还有其他连接
	m.mutex.RLock()
	hasOtherConns := len(m.connections[userID]) > 0
	m.mutex.RUnlock()

	// 如果没有其他连接，更新用户状态为离线
	if !hasOtherConns {
		if err := m.statusManager.UpdateUserStatus(userID, connType, false); err != nil {
			log.Printf("更新用户 %s 的离线状态失败: %v", userID, err)
		}
	}

	// 触发连接更新
	select {
	case m.connectionUpdateChan <- struct{}{}:
	default:
	}

	log.Printf("用户 %s 的 %s 连接已注销", userID, connType)
	return nil
}

// SendMessage 发送消息
func (m *RedisConnectionManager) SendMessage(message *protocol.Message) error {
	// 将消息放入本地队列
	select {
	case m.messageQueueChan <- message:
		// 消息已成功放入本地队列
		// log.Printf("消息已放入本地队列: %s -> %s", message.SenderID, message.RecipientID) // 可选日志
	default:
		log.Printf("警告: 消息队列已满，丢弃消息: %s -> %s", message.SenderID, message.RecipientID)
		return fmt.Errorf("消息队列已满")
	}

	// 新增：如果 Redis 启用，发布到 Redis，以便其他节点也能收到并处理
	// 只有当消息有明确的接收者时才需要发布到特定频道
	if m.redisEnabled && message.RecipientID != "" {
		// 构造成针对特定用户的频道
		channel := fmt.Sprintf("message_to:%s", message.RecipientID)
		msgBytes, err := json.Marshal(message)
		if err != nil {
			// 序列化失败是严重错误，但为了不阻塞发送，只记录日志
			log.Printf("序列化消息失败 for Redis publish: %v", err)
			// 不返回错误，继续流程
		} else {
			// 使用 Publish 将消息发送到 Redis 频道
			// PUBLISH 命令是 fire-and-forget，不关心是否有订阅者
			// 仅当消息未在本地处理时才发布到Redis
			if !message.HandledByLocal {
				err := m.redisClient.Publish(m.ctx, channel, msgBytes).Err()
				if err != nil {
					// 发布失败通常是 Redis 问题，记录日志
					log.Printf("发布消息到 Redis 频道 %s 失败: %v", channel, err)
					// 不返回错误，继续流程
				} else {
					// log.Printf("消息已发布到 Redis 频道 %s", channel) // 可选日志
				}
			}
		}
	} else if m.redisEnabled && message.RecipientID == "" {
		// 对于没有接收者的消息（如系统消息？），可能不需要发布，或者发布到广播频道
		// 根据你的协议设计决定是否需要处理
		// log.Printf("消息没有接收者ID，不发布到特定Redis频道")
	}

	return nil // 本地队列接收成功，返回nil
}

// processMessage 处理单个消息
func (m *RedisConnectionManager) processMessage(message *protocol.Message) {
	recipientID := message.RecipientID
	senderID := message.SenderID

	// 更详细的日志记录
	// 标记消息已本地处理
	message.HandledByLocal = true

	log.Printf("处理消息: SenderID=%s, RecipientID=%s, Type=%s, Content=%s",
		senderID, recipientID, message.Type, message.Content)

	// 检查接收者是否为空
	if recipientID == "" {
		// 对于没有接收者ID的消息，可能是系统消息或广播消息
		if message.Type == "status" || message.Type == "broadcast" {
			log.Printf("处理系统消息或广播消息: Type=%s, SenderID=%s", message.Type, senderID)
			// 这里可以添加广播逻辑
			return
		}
		log.Printf("警告: 接收者ID为空，无法处理普通消息 (发送者: %s, 类型: %s, 内容: %s)",
			senderID, message.Type, message.Content)
		return
	}

	log.Printf("处理从用户 %s 发送到用户 %s 的消息 (类型: %s)", senderID, recipientID, message.Type)

	// 检查接收者是否在本地连接
	m.mutex.RLock()
	userConns, ok := m.connections[recipientID]
	m.mutex.RUnlock()

	messageSent := false

	if ok {
		log.Printf("接收者 %s 有本地连接，尝试直接发送消息", recipientID)

		// 创建一个副本防止在迭代过程中修改map
		var connTypes []string
		var conns []Connection

		m.mutex.RLock()
		// 首先收集所有连接
		for connType, conn := range userConns {
			connTypes = append(connTypes, connType)
			conns = append(conns, conn)
		}
		m.mutex.RUnlock()

		// 首先尝试TCP连接
		for i, connType := range connTypes {
			if connType == ConnectionTypeTCP {
				err := conns[i].SendMessage(message)
				if err != nil {
					log.Printf("发送消息到用户 %s 的 TCP 连接失败: %v",
						recipientID, err)

					// 如果是"连接已关闭"错误，注销该连接
					if err.Error() == "连接已关闭" {
						m.UnregisterConnection(recipientID, connType)
					}
				} else {
					log.Printf("消息已通过 TCP 成功发送到用户 %s", recipientID)
					messageSent = true
					break
				}
			}
		}

		// 如果TCP发送失败或不存在TCP连接，尝试其他类型的连接
		if !messageSent {
			for i, connType := range connTypes {
				if connType == ConnectionTypeTCP {
					continue // 已经尝试过了
				}

				log.Printf("尝试通过 %s 连接发送消息到用户 %s", connType, recipientID)
				err := conns[i].SendMessage(message)
				if err != nil {
					log.Printf("发送消息到用户 %s 的 %s 连接失败: %v",
						recipientID, connType, err)

					// 如果是"连接已关闭"错误，注销该连接
					if err.Error() == "连接已关闭" {
						m.UnregisterConnection(recipientID, connType)
					}
				} else {
					log.Printf("消息已通过 %s 成功发送到用户 %s", connType, recipientID)
					messageSent = true
					break
				}
			}
		}
	}

	// 如果本地发送失败，存储为离线消息
	if !messageSent {
		log.Printf("接收者 %s 没有活跃连接或消息发送失败，存储为离线消息", recipientID)

		// 存储为离线消息
		if err := m.storeOfflineMessage(message); err != nil {
			log.Printf("存储离线消息失败: %v", err)
		} else {
			log.Printf("离线消息已成功存储，将在用户 %s 上线时发送", recipientID)
			messageSent = true
		}
	}

	// 处理群组消息
	if message.IsGroup {
		// TODO: 实现群聊消息转发
		log.Printf("群组消息转发功能尚未实现")
	}
}

// storeOfflineMessage 存储离线消息
func (m *RedisConnectionManager) storeOfflineMessage(message *protocol.Message) error {
	// 将消息标记为未发送状态
	message.Status = "unsent"

	// 确保消息有唯一ID
	if message.ID == "" {
		message.ID = uuid.New().String()
	}

	// 确保接收者ID不为空
	if message.RecipientID == "" {
		log.Printf("警告: 离线消息接收者ID为空，无法存储")
		return fmt.Errorf("接收者ID不能为空")
	}

	// 保存到数据库
	dbMessage := model.Message{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		SenderID:       message.SenderID,
		RecipientID:    message.RecipientID,
		Content:        message.Content,
		ContentType:    message.Type,
		Status:         message.Status,
		Timestamp:      message.Timestamp,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	log.Printf("存储离线消息: ID=%s, 发送者=%s, 接收者=%s",
		dbMessage.ID, dbMessage.SenderID, dbMessage.RecipientID)

	return database.GetDB().Create(&dbMessage).Error
}

// checkUserOnline 检查用户是否在线（在任何服务器上）
func (m *RedisConnectionManager) checkUserOnline(userID string) (bool, error) {
	return m.statusManager.IsUserOnline(userID)
}

// Run 启动连接管理器
func (m *RedisConnectionManager) Run(ctx context.Context) {
	defer m.Close()

	// 如果 Redis 启用，启动 Redis 消息订阅
	if m.redisEnabled {
		go m.startRedisSubscription()
	}

	// 定期更新连接心跳
	heartbeatTicker := time.NewTicker(1 * time.Minute)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("连接管理器关闭中...")
			return

		case message := <-m.messageQueueChan:
			m.processMessage(message)

		case <-m.connectionUpdateChan:
			// 连接更新逻辑，如必要时可以实现

		case <-heartbeatTicker.C:
			// 更新所有连接的心跳
			m.updateConnectionHeartbeats()
		}
	}
}

// startRedisSubscription 启动 Redis 消息订阅
func (m *RedisConnectionManager) startRedisSubscription() {
	// 订阅所有用户的消息
	pubsub := m.redisClient.PSubscribe(m.ctx, "message_to:*")
	defer pubsub.Close()

	// 处理接收到的消息
	ch := pubsub.Channel()
	for msg := range ch {
		// 解析消息内容
		var message protocol.Message
		if err := json.Unmarshal([]byte(msg.Payload), &message); err != nil {
			log.Printf("解析 Redis 消息失败: %v", err)
			continue
		}

		// 检查是否是当前服务器应该处理的消息
		channel := msg.Channel
		userID := channel[len("message_to:"):]

		m.mutex.RLock()
		_, hasUser := m.connections[userID]
		m.mutex.RUnlock()

		if hasUser {
			// 将消息放入处理队列
			select {
			case m.messageQueueChan <- &message:
				log.Printf("从 Redis 接收到消息，已加入处理队列")
			default:
				log.Printf("消息队列已满，无法处理从 Redis 接收到的消息")
			}
		}
	}
}

// updateConnectionHeartbeats 更新所有连接的心跳
func (m *RedisConnectionManager) updateConnectionHeartbeats() {
	if !m.redisEnabled {
		return
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for userID, userConns := range m.connections {
		for connType := range userConns {
			key := fmt.Sprintf("conn:%s:%s", userID, connType)

			// 更新过期时间
			err := m.redisClient.Expire(m.ctx, key, 30*time.Minute).Err()
			if err != nil {
				log.Printf("更新用户 %s 的 %s 连接心跳失败: %v", userID, connType, err)
			}
		}
	}
}

// Close 关闭连接管理器
func (m *RedisConnectionManager) Close() error {
	m.cancel()

	// 关闭所有连接
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, userConns := range m.connections {
		for _, conn := range userConns {
			conn.Close()
		}
	}

	// 清空连接映射
	m.connections = make(map[string]map[string]Connection)
	m.connectionsByType = make(map[string]map[string]Connection)

	// 关闭 Redis 连接
	if m.redisClient != nil {
		return m.redisClient.Close()
	}

	return nil
}

// GetOfflineMessages 获取离线消息
func (m *RedisConnectionManager) GetOfflineMessages(userID string) ([]*protocol.Message, error) {
	var messages []*protocol.Message

	// 查询数据库获取离线消息
	var dbMessages []model.Message
	err := database.GetDB().Where("recipient_id = ? AND status = ?", userID, "unsent").
		Order("timestamp asc").
		Find(&dbMessages).Error

	if err != nil {
		return nil, fmt.Errorf("查询离线消息失败: %w", err)
	}

	log.Printf("找到用户 %s 的 %d 条离线消息", userID, len(dbMessages))

	// 转换为协议消息
	for _, msg := range dbMessages {
		messages = append(messages, &protocol.Message{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			RecipientID:    userID,
			Content:        msg.Content,
			Type:           msg.ContentType,
			Timestamp:      msg.Timestamp,
			Status:         msg.Status,
		})
	}

	return messages, nil
}

// MarkOfflineMessagesAsSent 标记离线消息为已发送
func (m *RedisConnectionManager) MarkOfflineMessagesAsSent(userID string, messages []*protocol.Message) error {
	if len(messages) == 0 {
		return nil
	}

	var ids []string
	for _, msg := range messages {
		ids = append(ids, msg.ID)
	}

	// 更新消息状态
	return database.GetDB().Model(&model.Message{}).
		Where("id IN ?", ids).
		Update("status", "sent").Error
}
