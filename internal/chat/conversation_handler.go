package chat

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetConversations 获取用户的所有会话
func GetConversations(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	chatService := NewChatService()
	conversations, err := chatService.GetConversations(c.Request.Context(), userID.(string))
	if err != nil {
		log.Printf("获取会话列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取会话列表失败"})
		return
	}

	c.JSON(http.StatusOK, conversations)
}

// CreateConversation 创建新的会话
func CreateConversation(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req struct {
		RecipientID string `json:"RecipientID" binding:"required"`
		IsGroup     bool   `json:"is_group"`
		Name        string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("创建会话: 用户=%s, 接收者=%s, 是否群聊=%t",
		userID, req.RecipientID, req.IsGroup)

	chatService := NewChatService()
	conversation, err := chatService.CreateConversation(
		c.Request.Context(),
		userID.(string),
		req.RecipientID,
		req.IsGroup,
		req.Name,
	)

	if err != nil {
		log.Printf("创建会话失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建会话失败"})
		return
	}

	log.Printf("会话创建成功: %s", conversation.ID)
	c.JSON(http.StatusOK, conversation)
}

// GetConversation 获取单个会话详情
func GetConversation(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话ID不能为空"})
		return
	}

	chatService := NewChatService()
	conversation, err := chatService.GetConversationByID(c.Request.Context(), conversationID, userID.(string))
	if err != nil {
		log.Printf("获取会话详情失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取会话详情失败"})
		return
	}

	c.JSON(http.StatusOK, conversation)
}

// GetMessages 获取会话的消息历史
func GetMessages(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	log.Printf("userID:%s", userID)

	conversationID := c.Param("conversationId")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话ID不能为空"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		limit = 50
	}

	messageService := NewMessageService()
	messages, err := messageService.GetMessagesByConversation(c.Request.Context(), conversationID, limit)
	if err != nil {
		log.Printf("获取消息失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取消息失败"})
		return
	}

	c.JSON(http.StatusOK, messages)
}

// GetParticipants 获取会话参与者
func GetParticipants(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话ID不能为空"})
		return
	}

	chatService := NewChatService()

	// 确认用户是参与者
	conversation, err := chatService.GetConversationByID(c.Request.Context(), conversationID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在或无权访问"})
		c.JSON(123, conversation)
		return
	}

	participants, err := chatService.GetParticipants(c.Request.Context(), conversationID)
	if err != nil {
		log.Printf("获取参与者失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取参与者失败"})
		return
	}

	c.JSON(http.StatusOK, participants)
}

// MarkMessagesAsRead 标记消息为已读
func MarkMessagesAsRead(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话ID不能为空"})
		return
	}

	messageService := NewMessageService()
	if err := messageService.MarkMessagesAsRead(c.Request.Context(), conversationID, userID.(string)); err != nil {
		log.Printf("标记消息为已读失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "标记消息为已读失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "消息已标记为已读"})
}
