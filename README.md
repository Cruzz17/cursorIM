# CursorIM - 即时通讯系统

一个基于Go语言开发的功能完整的即时通讯系统，支持单聊、群聊、好友管理等核心功能。

## 功能特性

### 核心功能
- ✅ 用户注册、登录、认证
- ✅ 好友管理（添加好友、好友列表）
- ✅ 群组管理（创建群组、邀请成员、退出群组、解散群组）
- ✅ 单聊消息（实时发送、历史记录）
- ✅ 群聊消息（实时广播、历史记录）
- ✅ WebSocket实时通信
- ✅ 消息持久化存储
- ✅ 用户在线状态管理

### 技术架构
- **后端**: Go + Gin + GORM + WebSocket
- **数据库**: MySQL
- **缓存**: Redis
- **前端**: React + TypeScript + Antd
- **简单前端**: HTML + CSS + JavaScript
- **认证**: JWT Token
- **协议支持**: 
  - JSON over WebSocket (Web端)
  - Protobuf over TCP/WebSocket (App端)
  - 自动协议检测和适配

## 数据库设计

### 核心表结构

#### 用户表 (users)
```sql
- id: 用户唯一ID (UUID)
- username: 用户名 (唯一)
- password: 密码 (加密)
- nickname: 昵称
- avatar_url: 头像URL
- online: 在线状态
- created_at: 创建时间
- updated_at: 更新时间
```

#### 好友关系表 (friendships)
```sql
- id: 关系ID
- user_id: 用户ID
- friend_id: 好友ID
- status: 状态 (0-待确认，1-已好友)
- created_at: 创建时间
```

#### 群组表 (groups)
```sql
- id: 群组ID (UUID)
- name: 群组名称
- owner_id: 群主ID
- created_at: 创建时间
```

#### 群成员表 (group_members)
```sql
- id: 成员关系ID
- group_id: 群组ID
- user_id: 用户ID
- role: 角色 (0-成员，1-管理员)
- joined_at: 加入时间
```

#### 单聊消息表 (private_messages)
```sql
- id: 消息ID (UUID)
- sender_id: 发送者ID
- receiver_id: 接收者ID
- type: 消息类型 (text/image/file)
- content: 消息内容
- sent_at: 发送时间
- read: 是否已读
```

#### 群聊消息表 (group_messages)
```sql
- id: 消息ID (UUID)
- group_id: 群组ID
- sender_id: 发送者ID
- type: 消息类型
- content: 消息内容
- sent_at: 发送时间
```

## API接口文档

### 认证相关
- `POST /api/register` - 用户注册
- `POST /api/login` - 用户登录
- `GET /api/user/info` - 获取用户信息

### 好友管理
- `POST /api/friend/add` - 添加好友
- `GET /api/friends` - 获取好友列表
- `GET /api/user/search` - 搜索用户

### 群组管理
- `POST /api/group/create` - 创建群组
- `POST /api/group/:groupId/invite` - 邀请用户入群
- `POST /api/group/:groupId/exit` - 退出群组
- `GET /api/group/:groupId/members` - 获取群成员
- `GET /api/groups` - 获取用户群组列表
- `PUT /api/group/:groupId/name` - 更新群名称
- `DELETE /api/group/:groupId` - 解散群组

### 消息相关
- `GET /api/messages/user/:user_id` - 获取与指定用户的聊天记录
- `GET /api/messages/group/:group_id` - 获取群组聊天记录
- `WebSocket /api/ws` - 实时消息通信

## 快速开始

### 环境要求
- Go 1.19+
- Node.js 16+
- MySQL 5.7+
- Redis 6.0+

### 安装步骤

1. **克隆项目**
```bash
git clone <repository-url>
cd cursorIM
```

2. **安装依赖**
```bash
go mod download
```

3. **配置数据库**
编辑 `config.yaml` 文件：
```yaml
server:
  port: 8082

database:
  mysql:
    dsn: "root:password@tcp(127.0.0.1:3306)/im?charset=utf8mb4&parseTime=True&loc=Local"

jwt:
  secret: "your-secret-key"
  expire: 24

redis:
  host: "127.0.0.1"
  port: 6379
  password: ""
  db: 0
```

4. **创建数据库**
```sql
CREATE DATABASE im CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

5. **启动服务**

**方法一：使用全栈启动脚本（推荐）**
```bash
chmod +x start-fullstack.sh
./start-fullstack.sh
```

**方法二：分别启动前后端**

启动后端：
```bash
go run cmd/main.go
```

启动前端（新窗口）：
```bash
cd web/im-web
npm install
npm start
```

6. **访问前端**
- **主前端（推荐）**: `http://localhost:3000`
- **简单前端**: `http://localhost:8082/web/index.html`

## 使用说明

### 用户注册和登录
1. 访问前端页面
2. 点击"注册"按钮创建新账户
3. 使用用户名和密码登录

### 好友管理
1. 登录后在好友列表区域可以看到现有好友
2. 使用搜索功能查找其他用户并添加好友

### 群组功能
1. 在群组页面点击"创建群组"按钮创建新群组
2. 群主可以邀请好友加入群组
3. 群主可以修改群组名称和解散群组
4. 普通成员可以退出群组
5. 查看群组成员列表和权限

### 聊天功能
1. 在会话列表中点击好友或群组开始聊天
2. 支持单聊和群聊两种模式
3. 实时消息发送和接收
4. 消息历史记录保存和查看
5. 消息状态显示（发送中、已发送、已读）
6. 连接断开自动重连功能

### 前端界面
- **主前端**: 基于React + Antd的现代化界面
  - 响应式设计，支持移动端
  - 好友管理页面
  - 群组管理页面
  - 实时聊天界面
  - 统一的导航栏和用户菜单
- **简单前端**: 基础HTML页面，适合快速测试

## 协议支持

本系统支持双协议架构，自动根据客户端类型选择最优协议：

### 协议类型
- **JSON over WebSocket**: Web端使用，兼容性好，易于调试
- **Protobuf over TCP/WebSocket**: App端使用，高性能，消息体积小

### 连接端点
- **Web端 (JSON)**: `ws://localhost:8082/api/ws?token=<JWT_TOKEN>`
- **App端 (Protobuf WebSocket)**: `ws://localhost:8082/api/ws-tcp`
- **App端 (Protobuf TCP)**: `localhost:8083`

### 协议测试
```bash
# 编译服务器
go build -o bin/server cmd/main.go

# 运行协议测试
./test_protocols.sh

# 手动测试 Protobuf 客户端
go run test/protobuf_client.go <JWT_TOKEN>
```

详细协议文档请参考：[PROTOCOL_GUIDE.md](PROTOCOL_GUIDE.md)

## WebSocket消息协议

### 消息格式
```