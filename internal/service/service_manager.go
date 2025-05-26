package service

import (
	"context"
	"log"

	"cursorIM/internal/chat"
	"cursorIM/internal/connection"
	"cursorIM/internal/group"
	"cursorIM/internal/status"
	"cursorIM/internal/user"
)

// Manager 统一服务管理器
type Manager struct {
	ctx               context.Context
	chatService       *chat.MessageService
	accountService    *user.AccountService
	groupService      *group.GroupService
	statusManager     *status.Manager
	connectionManager connection.ConnectionManager
}

// NewManager 创建服务管理器
func NewManager(ctx context.Context, connMgr connection.ConnectionManager) *Manager {
	manager := &Manager{
		ctx:               ctx,
		connectionManager: connMgr,
		chatService:       chat.NewMessageService(),
		accountService:    user.NewAccountService(),
		groupService:      group.NewGroupService(),
		statusManager:     status.NewManager(ctx),
	}

	// 设置聊天服务的连接管理器
	manager.chatService.SetConnectionManager(connMgr)

	log.Println("服务管理器初始化完成")
	return manager
}

// GetChatService 获取聊天服务
func (m *Manager) GetChatService() *chat.MessageService {
	return m.chatService
}

// GetAccountService 获取账户服务
func (m *Manager) GetAccountService() *user.AccountService {
	return m.accountService
}

// GetGroupService 获取群组服务
func (m *Manager) GetGroupService() *group.GroupService {
	return m.groupService
}

// GetStatusManager 获取状态管理器
func (m *Manager) GetStatusManager() *status.Manager {
	return m.statusManager
}

// GetConnectionManager 获取连接管理器
func (m *Manager) GetConnectionManager() connection.ConnectionManager {
	return m.connectionManager
}

// Shutdown 关闭所有服务
func (m *Manager) Shutdown() {
	log.Println("正在关闭服务管理器...")

	// 清理状态缓存
	if err := m.statusManager.CleanupExpiredStatuses(); err != nil {
		log.Printf("清理状态缓存失败: %v", err)
	}

	log.Println("服务管理器已关闭")
}
