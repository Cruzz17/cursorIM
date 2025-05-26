package chat

import (
	"context"
	"errors"
	"time"

	"cursorIM/internal/database"
	"cursorIM/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatService 处理会话和消息相关逻辑
type ChatService struct {
	db *gorm.DB
}

// NewChatService 创建聊天服务实例
func NewChatService() *ChatService {
	return &ChatService{
		db: database.GetDB(),
	}
}

// CreateConversation 创建新的会话
func (s *ChatService) CreateConversation(ctx context.Context, userID, recipientID string, isGroup bool, name string) (*ConversationResponse, error) {
	// 检查单聊是否已存在
	if !isGroup {
		var existingConvID string

		err := s.db.Raw(`
			SELECT c.id FROM conversations c
			JOIN participants p1 ON c.id = p1.conversation_id
			JOIN participants p2 ON c.id = p2.conversation_id
			WHERE c.is_group = false AND p1.user_id = ? AND p2.user_id = ?
		`, userID, recipientID).Scan(&existingConvID).Error

		if err == nil && existingConvID != "" {
			// 会话已存在，获取会话信息
			var conversation ConversationResponse

			err := s.db.Raw(`
				SELECT c.id, c.name, c.is_group as isGroup, 
				       COALESCE(m.content, '') as lastMessage,
				       0 as unread
				FROM conversations c
				LEFT JOIN messages m ON m.conversation_id = c.id
				WHERE c.id = ? AND (
					m.id = (
						SELECT msg.id FROM messages msg
						WHERE msg.conversation_id = c.id
						ORDER BY msg.created_at DESC
						LIMIT 1
					) OR m.id IS NULL
				)
			`, existingConvID).Scan(&conversation).Error

			if err == nil {
				// 处理会话名称
				if conversation.Name == "" || conversation.Name == userID {
					var recipient struct {
						Username string
						Nickname string
					}

					s.db.Raw(`SELECT username, nickname FROM users WHERE id = ?`, recipientID).Scan(&recipient)

					if recipient.Nickname != "" {
						conversation.Name = recipient.Nickname
					} else {
						conversation.Name = recipient.Username
					}
				}

				return &conversation, nil
			}
		}
	}

	// 创建新会话
	tx := s.db.Begin()

	// 1. 创建会话
	now := time.Now()
	conversation := model.Conversation{
		ID:   uuid.New().String(),
		Name: name,
		Type: func() int {
			if isGroup {
				return 1
			} else {
				return 0
			}
		}(),
		IsGroup:   isGroup,
		LastMsg:   "",
		LastTime:  now, // 确保初始化LastTime字段
		Unread:    0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := tx.Create(&conversation).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 2. 添加参与者
	participants := []model.Participant{
		{
			ID:             uuid.New().String(),
			ConversationID: conversation.ID,
			UserID:         userID,
			JoinedAt:       time.Now(),
			LastReadAt:     time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversation.ID,
			UserID:         recipientID,
			JoinedAt:       time.Now(),
			LastReadAt:     time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	for _, p := range participants {
		if err := tx.Create(&p).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// 获取对方用户信息（用于设置会话名称）
	convResponse := &ConversationResponse{
		ID:          conversation.ID,
		Name:        name,
		LastMessage: "",
		Unread:      0,
		IsGroup:     isGroup,
	}

	// 如果是单聊且没有指定名称，使用对方昵称或用户名
	if !isGroup && (name == "" || name == userID) {
		var recipient struct {
			Username string
			Nickname string
		}

		s.db.Raw(`SELECT username, nickname FROM users WHERE id = ?`, recipientID).Scan(&recipient)

		if recipient.Nickname != "" {
			convResponse.Name = recipient.Nickname
		} else {
			convResponse.Name = recipient.Username
		}
	}

	return convResponse, nil
}

// GetConversations 获取用户的所有会话
func (s *ChatService) GetConversations(ctx context.Context, userID string) ([]ConversationResponse, error) {
	var conversations []ConversationResponse

	// 查询用户参与的所有会话
	err := s.db.Raw(`
		SELECT c.id, c.name, c.is_group as isGroup, 
		       COALESCE(m.content, '') as lastMessage,
		       (SELECT COUNT(*) FROM messages msg 
		        WHERE msg.conversation_id = c.id 
		          AND msg.created_at > COALESCE(p.last_read_at, '1970-01-01')
		          AND msg.sender_id != ?) as unread
		FROM conversations c
		JOIN participants p ON c.id = p.conversation_id AND p.user_id = ?
		LEFT JOIN messages m ON m.conversation_id = c.id
		WHERE m.id = (
			SELECT msg.id FROM messages msg
			WHERE msg.conversation_id = c.id
			ORDER BY msg.created_at DESC
			LIMIT 1
		) OR m.id IS NULL
		ORDER BY COALESCE(m.created_at, c.created_at) DESC
	`, userID, userID).Scan(&conversations).Error

	if err != nil {
		return nil, err
	}

	// 处理会话名称 - 对于单聊，如果没有名称，使用对方的昵称
	for i, conv := range conversations {
		if !conv.IsGroup && (conv.Name == "" || conv.Name == userID) {
			// 查找对方用户信息
			var otherUser struct {
				ID       string
				Username string
				Nickname string
			}

			err := s.db.Raw(`
				SELECT u.id, u.username, u.nickname
				FROM users u
				JOIN participants p ON u.id = p.user_id
				WHERE p.conversation_id = ? AND p.user_id != ?
				LIMIT 1
			`, conv.ID, userID).Scan(&otherUser).Error

			if err == nil && otherUser.ID != "" {
				// 优先使用昵称，如果没有则使用用户名
				if otherUser.Nickname != "" {
					conversations[i].Name = otherUser.Nickname
				} else {
					conversations[i].Name = otherUser.Username
				}
			}
		}
	}

	return conversations, nil
}

// GetConversationByID 根据ID获取会话详情
func (s *ChatService) GetConversationByID(ctx context.Context, conversationID, userID string) (*ConversationResponse, error) {
	var conversation ConversationResponse

	err := s.db.Raw(`
		SELECT c.id, c.name, c.is_group as isGroup, 
		       COALESCE(m.content, '') as lastMessage,
		       (SELECT COUNT(*) FROM messages msg 
		        WHERE msg.conversation_id = c.id 
		          AND msg.created_at > COALESCE(p.last_read_at, '1970-01-01')
		          AND msg.sender_id != ?) as unread
		FROM conversations c
		JOIN participants p ON c.id = p.conversation_id AND p.user_id = ?
		LEFT JOIN messages m ON m.conversation_id = c.id
		WHERE c.id = ? AND (
			m.id = (
				SELECT msg.id FROM messages msg
				WHERE msg.conversation_id = c.id
				ORDER BY msg.created_at DESC
				LIMIT 1
			) OR m.id IS NULL
		)
	`, userID, userID, conversationID).Scan(&conversation).Error

	if err != nil {
		return nil, err
	}

	// 处理会话名称 - 对于单聊，如果没有名称，使用对方的昵称
	if !conversation.IsGroup && (conversation.Name == "" || conversation.Name == userID) {
		var otherUser struct {
			Username string
			Nickname string
		}

		err := s.db.Raw(`
			SELECT u.username, u.nickname
			FROM users u
			JOIN participants p ON u.id = p.user_id
			WHERE p.conversation_id = ? AND p.user_id != ?
			LIMIT 1
		`, conversationID, userID).Scan(&otherUser).Error

		if err == nil {
			if otherUser.Nickname != "" {
				conversation.Name = otherUser.Nickname
			} else {
				conversation.Name = otherUser.Username
			}
		}
	}

	return &conversation, nil
}

// GetParticipants 获取会话的所有参与者信息
func (s *ChatService) GetParticipants(ctx context.Context, conversationID string) ([]model.User, error) {
	var users []model.User

	err := s.db.Raw(`
		SELECT u.* FROM users u
		JOIN participants p ON u.id = p.user_id
		WHERE p.conversation_id = ?
	`, conversationID).Scan(&users).Error

	return users, err
}

// AddParticipant 添加用户到会话
func (s *ChatService) AddParticipant(ctx context.Context, conversationID, userID string) error {
	// 检查会话是否存在
	var count int64
	if err := s.db.Model(&model.Conversation{}).Where("id = ?", conversationID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("会话不存在")
	}

	// 检查用户是否已在会话中
	if err := s.db.Model(&model.Participant{}).Where("conversation_id = ? AND user_id = ?", conversationID, userID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("用户已在会话中")
	}

	// 添加用户到会话
	participant := model.Participant{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		UserID:         userID,
		JoinedAt:       time.Now(),
		LastReadAt:     time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	return s.db.Create(&participant).Error
}

// RemoveParticipant 从会话中移除用户
func (s *ChatService) RemoveParticipant(ctx context.Context, conversationID, userID string) error {
	return s.db.Where("conversation_id = ? AND user_id = ?", conversationID, userID).Delete(&model.Participant{}).Error
}
