# CursorIM 项目重构指南

## 重构概述

本次重构主要解决了以下问题：
1. **编译错误修复** - 解决了结构体字段不匹配和语法错误
2. **代码重复消除** - 删除了重复的模块和代码
3. **模块结构优化** - 重新组织了项目结构，提高可维护性
4. **统一服务管理** - 引入了服务管理器模式

## 主要改动

### 1. 模型层优化 (`internal/model/model.go`)

**修改**:
- 为 `Conversation` 结构体添加了 `Name` 和 `IsGroup` 字段
- 统一了会话模型，支持单聊和群聊

**影响**:
- 解决了 `chat_service.go` 中的编译错误
- 提高了数据模型的完整性

### 2. 状态管理重构 (`internal/status/`)

**修改**:
- 删除了重复的 `status_service.go`
- 重写了 `manager.go`，修复语法错误
- 统一了用户状态管理逻辑

**优势**:
- 消除了代码重复
- 提供了更清晰的状态管理API
- 支持多种连接类型（HTTP、WebSocket、TCP）

### 3. 服务管理器引入 (`internal/service/`)

**新增**:
- `service_manager.go` - 统一服务管理器

**功能**:
- 协调各个模块之间的交互
- 提供统一的服务获取接口
- 简化了依赖注入

**使用示例**:
```go
// 创建服务管理器
serviceMgr := service.NewManager(ctx, connMgr)

// 获取各种服务
chatService := serviceMgr.GetChatService()
accountService := serviceMgr.GetAccountService()
groupService := serviceMgr.GetGroupService()
statusManager := serviceMgr.GetStatusManager()
```

### 4. 常量统一管理 (`internal/constants/`)

**新增**:
- `constants.go` - 全局常量定义

**包含**:
- 用户状态常量
- 连接类型常量
- 消息类型常量
- 会话类型常量
- 群组角色常量
- Redis键前缀
- 错误信息常量

**优势**:
- 减少硬编码
- 提高代码可维护性
- 便于统一修改

### 5. 主程序重构 (`cmd/main.go`)

**修改**:
- 使用统一服务管理器
- 简化了依赖管理
- 改进了关闭逻辑

**对比**:
```go
// 重构前
messageService := chat.NewMessageService()
messageService.SetConnectionManager(connMgr)

// 重构后
serviceMgr := service.NewManager(context.Background(), connMgr)
messageService := serviceMgr.GetChatService()
```

### 6. 前端架构完善

**新增组件**:
- `GroupsList.tsx` - 群组管理界面
- `MainLayout.tsx` - 统一布局组件
- `ConversationList.tsx` - 改进的会话列表

**功能增强**:
- 群组创建和管理
- 统一导航栏
- 响应式设计
- 更好的用户体验

## 架构改进

### 重构前的问题

1. **模块耦合度高**: 各模块直接相互依赖
2. **代码重复**: status模块有两个重复的文件
3. **常量分散**: 魔法数字和字符串散布各处
4. **依赖管理复杂**: main.go中手动管理所有依赖

### 重构后的优势

1. **清晰的分层架构**:
   ```
   main.go
     ↓
   service.Manager (统一服务管理)
     ↓
   chat/user/group/status (业务模块)
     ↓
   model/database (数据层)
   ```

2. **模块解耦**: 通过服务管理器进行依赖注入
3. **代码复用**: 统一的常量和工具函数
4. **易于测试**: 清晰的接口和依赖注入
5. **易于扩展**: 新模块可以轻松集成到服务管理器

## 使用指南

### 启动应用

**方法一：全栈启动（推荐）**
```bash
chmod +x start-fullstack.sh
./start-fullstack.sh
```

**方法二：分别启动**
```bash
# 后端
go run cmd/main.go

# 前端（新窗口）
cd web/im-web
npm install
npm start
```

### 添加新服务

1. 在相应目录创建服务文件
2. 在 `service_manager.go` 中添加服务
3. 更新构造函数和getter方法

示例：
```go
// 1. 在 internal/newmodule/ 创建服务
type NewService struct {}
func NewNewService() *NewService { return &NewService{} }

// 2. 在服务管理器中添加
type Manager struct {
    // ... 其他字段
    newService *newmodule.NewService
}

// 3. 更新构造函数
func NewManager(ctx context.Context, connMgr connection.ConnectionManager) *Manager {
    return &Manager{
        // ... 其他服务
        newService: newmodule.NewNewService(),
    }
}

// 4. 添加getter方法
func (m *Manager) GetNewService() *newmodule.NewService {
    return m.newService
}
```

## 性能优化

### Redis使用优化
- 使用Pipeline批量操作
- 合理设置过期时间
- 统一Redis键命名规范

### 内存管理
- 定期清理过期状态缓存
- 使用对象池减少GC压力
- 合理的连接池大小设置

### 并发优化
- 读写锁保护共享状态
- 协程池管理并发数量
- 合理的超时设置

## 后续计划

1. **监控和日志**:
   - 添加Prometheus指标
   - 结构化日志输出
   - 性能监控面板

2. **测试覆盖**:
   - 单元测试
   - 集成测试
   - 压力测试

3. **部署优化**:
   - Docker容器化
   - Kubernetes部署
   - CI/CD流水线

4. **功能扩展**:
   - 文件上传
   - 音视频通话
   - 消息推送

## 注意事项

1. **数据库迁移**: 如果已有数据，需要执行数据库迁移脚本
2. **配置更新**: 确保 `config.yaml` 包含所有必要配置
3. **Redis依赖**: 某些功能需要Redis支持，建议部署Redis服务
4. **前端依赖**: React前端需要Node.js 16+环境

## 问题排查

### 常见问题

1. **编译错误**: 运行 `go mod tidy` 更新依赖
2. **Redis连接失败**: 检查Redis服务状态和配置
3. **数据库连接失败**: 确认数据库服务和配置正确
4. **前端启动失败**: 确认Node.js版本和依赖安装

### 调试建议

1. 启用详细日志输出
2. 检查各服务的健康状态
3. 使用调试工具分析性能瓶颈
4. 查看错误日志和堆栈跟踪

---

## 总结

本次重构大幅提升了项目的代码质量和可维护性，解决了原有的编译错误和代码重复问题。通过引入服务管理器模式和统一的常量管理，项目架构更加清晰，扩展性更强。同时，完善的前端界面为用户提供了更好的使用体验。 