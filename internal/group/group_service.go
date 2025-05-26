package group

import (
	"context"
	"cursorIM/internal/database"
	"cursorIM/internal/model"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GroupService struct {
	db *gorm.DB
}

func NewGroupService() *GroupService {
	return &GroupService{
		db: database.GetDB(),
	}
}

// CreateGroup 创建群组
func (s *GroupService) CreateGroup(ctx context.Context, ownerID, name string) (*model.Group, error) {
	group := &model.Group{
		ID:        uuid.New().String(),
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	// 创建群组
	if err := tx.Create(group).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 添加群主为成员
	member := &model.GroupMember{
		ID:       uuid.New().String(),
		GroupID:  group.ID,
		UserID:   ownerID,
		Role:     1, // 管理员角色
		JoinedAt: time.Now(),
	}

	if err := tx.Create(member).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()
	return group, nil
}

// InviteUser 邀请用户入群
func (s *GroupService) InviteUser(ctx context.Context, groupID, userID, inviterID string) error {
	// 检查群组是否存在
	var group model.Group
	if err := s.db.First(&group, "id = ?", groupID).Error; err != nil {
		return errors.New("群组不存在")
	}

	// 检查邀请者权限（只有群主和管理员可以邀请）
	var inviterMember model.GroupMember
	if err := s.db.First(&inviterMember, "group_id = ? AND user_id = ?", groupID, inviterID).Error; err != nil {
		return errors.New("您不是群成员")
	}

	if inviterMember.Role == 0 {
		return errors.New("权限不足")
	}

	// 检查用户是否已经是群成员
	var existMember model.GroupMember
	if err := s.db.First(&existMember, "group_id = ? AND user_id = ?", groupID, userID).Error; err == nil {
		return errors.New("用户已经是群成员")
	}

	// 添加新成员
	member := &model.GroupMember{
		ID:       uuid.New().String(),
		GroupID:  groupID,
		UserID:   userID,
		Role:     0, // 普通成员
		JoinedAt: time.Now(),
	}

	return s.db.Create(member).Error
}

// ExitGroup 退出群组
func (s *GroupService) ExitGroup(ctx context.Context, groupID, userID string) error {
	// 检查是否是群成员
	var member model.GroupMember
	if err := s.db.First(&member, "group_id = ? AND user_id = ?", groupID, userID).Error; err != nil {
		return errors.New("您不是群成员")
	}

	// 检查是否是群主
	var group model.Group
	if err := s.db.First(&group, "id = ?", groupID).Error; err != nil {
		return errors.New("群组不存在")
	}

	if group.OwnerID == userID {
		return errors.New("群主不能退出群组，请先转让群主身份")
	}

	// 删除群成员记录
	return s.db.Delete(&member).Error
}

// GetGroupMembers 获取群成员列表
func (s *GroupService) GetGroupMembers(ctx context.Context, groupID string) ([]model.User, error) {
	var users []model.User
	err := s.db.Table("users").
		Joins("JOIN group_members ON users.id = group_members.user_id").
		Where("group_members.group_id = ?", groupID).
		Find(&users).Error

	return users, err
}

// GetUserGroups 获取用户所在的群组列表
func (s *GroupService) GetUserGroups(ctx context.Context, userID string) ([]model.Group, error) {
	var groups []model.Group
	err := s.db.Table("groups").
		Joins("JOIN group_members ON groups.id = group_members.group_id").
		Where("group_members.user_id = ?", userID).
		Find(&groups).Error

	return groups, err
}

// UpdateGroupName 更新群名称
func (s *GroupService) UpdateGroupName(ctx context.Context, groupID, userID, newName string) error {
	// 检查权限（只有群主和管理员可以修改）
	var member model.GroupMember
	if err := s.db.First(&member, "group_id = ? AND user_id = ?", groupID, userID).Error; err != nil {
		return errors.New("您不是群成员")
	}

	if member.Role == 0 {
		return errors.New("权限不足")
	}

	return s.db.Model(&model.Group{}).
		Where("id = ?", groupID).
		Update("name", newName).Error
}

// DeleteGroup 解散群组（仅群主可操作）
func (s *GroupService) DeleteGroup(ctx context.Context, groupID, userID string) error {
	// 检查是否是群主
	var group model.Group
	if err := s.db.First(&group, "id = ?", groupID).Error; err != nil {
		return errors.New("群组不存在")
	}

	if group.OwnerID != userID {
		return errors.New("只有群主可以解散群组")
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// 删除所有群成员
	if err := tx.Delete(&model.GroupMember{}, "group_id = ?", groupID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 删除群组
	if err := tx.Delete(&group).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}
