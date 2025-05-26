package group

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CreateGroupRequest 创建群组请求
type CreateGroupRequest struct {
	Name string `json:"name" binding:"required"`
}

// InviteUserRequest 邀请用户请求
type InviteUserRequest struct {
	UserID string `json:"userId" binding:"required"`
}

// UpdateGroupNameRequest 更新群名请求
type UpdateGroupNameRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateGroup 创建群组
func CreateGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service := NewGroupService()
	group, err := service.CreateGroup(c.Request.Context(), userID.(string), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "群组创建成功",
		"group":   group,
	})
}

// InviteUser 邀请用户入群
func InviteUser(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	groupID := c.Param("groupId")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组ID不能为空"})
		return
	}

	var req InviteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service := NewGroupService()
	err := service.InviteUser(c.Request.Context(), groupID, req.UserID, userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "邀请成功"})
}

// ExitGroup 退出群组
func ExitGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	groupID := c.Param("groupId")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组ID不能为空"})
		return
	}

	service := NewGroupService()
	err := service.ExitGroup(c.Request.Context(), groupID, userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "退出群组成功"})
}

// GetGroupMembers 获取群成员列表
func GetGroupMembers(c *gin.Context) {
	groupID := c.Param("groupId")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组ID不能为空"})
		return
	}

	service := NewGroupService()
	members, err := service.GetGroupMembers(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"members": members,
	})
}

// GetUserGroups 获取用户所在的群组列表
func GetUserGroups(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	service := NewGroupService()
	groups, err := service.GetUserGroups(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
	})
}

// UpdateGroupName 更新群名称
func UpdateGroupName(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	groupID := c.Param("groupId")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组ID不能为空"})
		return
	}

	var req UpdateGroupNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service := NewGroupService()
	err := service.UpdateGroupName(c.Request.Context(), groupID, userID.(string), req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "群名称更新成功"})
}

// DeleteGroup 解散群组
func DeleteGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	groupID := c.Param("groupId")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组ID不能为空"})
		return
	}

	service := NewGroupService()
	err := service.DeleteGroup(c.Request.Context(), groupID, userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "群组解散成功"})
}
