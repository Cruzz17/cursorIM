package chat

import (
	"context"
	"fmt"
	"log"
	"time"

	"cursorIM/internal/database"
	"cursorIM/internal/model"
	"cursorIM/internal/protocol"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessageService struct {
	db            *gorm.DB
	notifyChannel chan *protocol.Message
	connManager   interface{} // We'll use this to access the connection manager
}

func NewMessageService() *MessageService {
	return &MessageService{
		db:            database.GetDB(),
		notifyChannel: make(chan *protocol.Message, 100),
	}
}

// SetConnectionManager sets the connection manager for message routing
func (s *MessageService) SetConnectionManager(manager interface{}) {
	s.connManager = manager

	// Start the notification processor
	go s.processNotifications()
}

// processNotifications handles outgoing status notifications
func (s *MessageService) processNotifications() {
	for msg := range s.notifyChannel {
		// If we have a connection manager that supports SendMessage, use it
		if cm, ok := s.connManager.(interface{ SendMessage(*protocol.Message) error }); ok {
			if err := cm.SendMessage(msg); err != nil {
				log.Printf("发送通知消息失败: %v", err)
			}
		} else {
			log.Printf("通知消息无法发送，连接管理器未设置或不支持SendMessage")
		}
	}
}

// SaveMessage 保存一条消息到数据库
func (s *MessageService) SaveMessage(ctx context.Context, message *protocol.Message) error {
	// 不保存心跳消息
	if message.Type == "ping" || message.Type == "pong" {
		return nil
	}

	// 确保消息有唯一ID
	if message.ID == "" {
		message.ID = uuid.New().String()
	}

	// 判断是群聊还是单聊消息
	if message.IsGroup {
		// 保存为群聊消息
		return s.saveGroupMessage(ctx, message)
	} else {
		// 保存为单聊消息
		return s.savePrivateMessage(ctx, message)
	}
}

// savePrivateMessage 保存单聊消息
func (s *MessageService) savePrivateMessage(ctx context.Context, message *protocol.Message) error {
	// 确保必要字段不为空
	if message.RecipientID == "" && message.Type != "status" {
		return fmt.Errorf("单聊消息接收者ID不能为空")
	}

	// 设置默认状态
	status := message.Status
	if status == "" {
		status = "sent"
	}

	// 创建单聊消息记录
	privateMsg := model.PrivateMessage{
		ID:         message.ID,
		SenderID:   message.SenderID,
		ReceiverID: message.RecipientID,
		Type:       message.Type,
		Content:    message.Content,
		SentAt:     time.Now(),
		Read:       false,
	}

	// 保存消息
	err := s.db.Create(&privateMsg).Error
	if err != nil {
		log.Printf("保存单聊消息到数据库失败: %v", err)
		return err
	}

	// 同时保存到通用消息表（兼容现有逻辑）
	dbMessage := model.Message{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		SenderID:       message.SenderID,
		RecipientID:    message.RecipientID,
		Content:        message.Content,
		ContentType:    message.Type,
		Status:         status,
		Timestamp:      message.Timestamp,
		IsGroup:        false,
		Type:           message.Type,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = s.db.Create(&dbMessage).Error
	if err != nil {
		log.Printf("保存消息到通用表失败: %v", err)
		return err
	}

	log.Printf("单聊消息已成功保存: ID=%s, 发送者=%s, 接收者=%s, 类型=%s",
		privateMsg.ID, privateMsg.SenderID, privateMsg.ReceiverID, privateMsg.Type)

	return nil
}

// saveGroupMessage 保存群聊消息
func (s *MessageService) saveGroupMessage(ctx context.Context, message *protocol.Message) error {
	// 创建群聊消息记录
	groupMsg := model.GroupMessage{
		ID:       message.ID,
		GroupID:  message.RecipientID, // 对于群聊，RecipientID是GroupID
		SenderID: message.SenderID,
		Type:     message.Type,
		Content:  message.Content,
		SentAt:   time.Now(),
	}

	// 保存消息
	err := s.db.Create(&groupMsg).Error
	if err != nil {
		log.Printf("保存群聊消息到数据库失败: %v", err)
		return err
	}

	// 同时保存到通用消息表（兼容现有逻辑）
	dbMessage := model.Message{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		SenderID:       message.SenderID,
		RecipientID:    message.RecipientID,
		Content:        message.Content,
		ContentType:    message.Type,
		Status:         "sent",
		Timestamp:      message.Timestamp,
		IsGroup:        true,
		Type:           message.Type,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = s.db.Create(&dbMessage).Error
	if err != nil {
		log.Printf("保存群聊消息到通用表失败: %v", err)
		return err
	}

	log.Printf("群聊消息已成功保存: ID=%s, 群组=%s, 发送者=%s, 类型=%s",
		groupMsg.ID, groupMsg.GroupID, groupMsg.SenderID, groupMsg.Type)

	return nil
}

// GetPrivateMessages 获取两个用户之间的单聊消息
func (s *MessageService) GetPrivateMessages(ctx context.Context, userID, otherUserID string, limit int) ([]*protocol.Message, error) {
	var dbMessages []model.PrivateMessage

	// 查询两个用户之间的消息
	err := s.db.Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
		userID, otherUserID, otherUserID, userID).
		Order("sent_at DESC").
		Limit(limit).
		Find(&dbMessages).Error

	if err != nil {
		return nil, err
	}

	// 转换为协议消息
	var messages []*protocol.Message
	for i := len(dbMessages) - 1; i >= 0; i-- { // 反转顺序，最早的消息在前
		msg := dbMessages[i]
		messages = append(messages, &protocol.Message{
			ID:          msg.ID,
			SenderID:    msg.SenderID,
			RecipientID: msg.ReceiverID,
			Content:     msg.Content,
			Type:        msg.Type,
			Timestamp:   msg.SentAt.Unix(),
			IsGroup:     false,
		})
	}

	return messages, nil
}

// GetGroupMessages 获取群组消息历史
func (s *MessageService) GetGroupMessages(ctx context.Context, groupID string, limit int) ([]*protocol.Message, error) {
	var dbMessages []model.GroupMessage

	// 查询群组消息
	err := s.db.Where("group_id = ?", groupID).
		Order("sent_at DESC").
		Limit(limit).
		Find(&dbMessages).Error

	if err != nil {
		return nil, err
	}

	// 转换为协议消息
	var messages []*protocol.Message
	for i := len(dbMessages) - 1; i >= 0; i-- { // 反转顺序，最早的消息在前
		msg := dbMessages[i]
		messages = append(messages, &protocol.Message{
			ID:          msg.ID,
			SenderID:    msg.SenderID,
			RecipientID: msg.GroupID,
			Content:     msg.Content,
			Type:        msg.Type,
			Timestamp:   msg.SentAt.Unix(),
			IsGroup:     true,
		})
	}

	return messages, nil
}

// BroadcastToGroup 向群组广播消息
func (s *MessageService) BroadcastToGroup(ctx context.Context, message *protocol.Message) error {
	groupID := message.RecipientID

	// 获取群组成员
	var members []model.GroupMember
	err := s.db.Where("group_id = ?", groupID).Find(&members).Error
	if err != nil {
		return fmt.Errorf("获取群组成员失败: %w", err)
	}

	// 向每个成员发送消息（除了发送者）
	for _, member := range members {
		if member.UserID != message.SenderID {
			groupMsg := &protocol.Message{
				ID:          message.ID,
				Type:        message.Type,
				SenderID:    message.SenderID,
				RecipientID: member.UserID,
				Content:     message.Content,
				Timestamp:   message.Timestamp,
				IsGroup:     true,
				GroupID:     groupID,
			}

			// 发送通知
			select {
			case s.notifyChannel <- groupMsg:
				// 消息已发送到通知频道
			default:
				log.Printf("通知频道已满，丢弃给用户 %s 的群聊消息", member.UserID)
			}
		}
	}

	return nil
}

// GetMessagesByConversation 获取特定会话的消息历史
func (s *MessageService) GetMessagesByConversation(ctx context.Context, conversationID string, limit int64) ([]*protocol.Message, error) {
	var dbMessages []model.Message

	// 查询消息
	err := s.db.Where("conversation_id = ?", conversationID).
		Order("timestamp desc").
		Limit(int(limit)).
		Find(&dbMessages).Error

	if err != nil {
		return nil, err
	}

	// 转换为协议消息
	var messages []*protocol.Message
	for i := len(dbMessages) - 1; i >= 0; i-- { // 反转顺序
		msg := dbMessages[i]
		messages = append(messages, &protocol.Message{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			Content:        msg.Content,
			Type:           msg.ContentType,
			Timestamp:      msg.Timestamp,
			Status:         msg.Status,
			RecipientID:    msg.RecipientID,
		})
	}

	return messages, nil
}

// GetMessages 获取两个用户之间的消息历史
func (s *MessageService) GetMessages(ctx context.Context, userID string, otherUserID string, limit int64) ([]*protocol.Message, error) {
	// 首先找到这两个用户之间的会话
	var conversationID string
	err := s.db.Raw(`
		SELECT c.id FROM conversations c
		JOIN participants p1 ON c.id = p1.conversation_id
		JOIN participants p2 ON c.id = p2.conversation_id
		WHERE c.is_group = ? AND p1.user_id = ? AND p2.user_id = ?
	`, false, userID, otherUserID).Scan(&conversationID).Error

	if err != nil {
		return nil, err
	}

	if conversationID == "" {
		// 没有会话，返回空消息列表
		return []*protocol.Message{}, nil
	}

	// 获取会话中的消息
	return s.GetMessagesByConversation(ctx, conversationID, limit)
}

// MarkMessagesAsRead 将消息标记为已读
func (s *MessageService) MarkMessagesAsRead(ctx context.Context, conversationID string, userID string) error {
	// 更新参与者的最后读取时间
	return s.db.Model(&model.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("last_read_at", time.Now()).Error
}

// BroadcastStatus broadcasts user status changes
func (s *MessageService) BroadcastStatus(ctx context.Context, message *protocol.Message) error {
	// Get user's friends to notify them of status change
	userID := message.SenderID
	friends, err := s.getUserFriends(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user friends: %w", err)
	}

	// Create a copy of the message for each friend
	for _, friendID := range friends {
		statusMsg := &protocol.Message{
			Type:        "status",
			SenderID:    userID,
			RecipientID: friendID,
			Content:     message.Content,
			Timestamp:   message.Timestamp,
		}

		// Send through notification channel
		select {
		case s.notifyChannel <- statusMsg:
			// Message sent to channel
		default:
			log.Printf("Status notification channel full, dropping status update for user %s", friendID)
		}
	}

	return nil
}

// getUserFriends gets a user's friends
func (s *MessageService) getUserFriends(ctx context.Context, userID string) ([]string, error) {
	var friends []model.Friendship
	if err := database.GetDB().Where("user_id = ?", userID).Find(&friends).Error; err != nil {
		return nil, err
	}

	var friendIDs []string
	for _, friend := range friends {
		friendIDs = append(friendIDs, friend.FriendID)
	}

	return friendIDs, nil
}
