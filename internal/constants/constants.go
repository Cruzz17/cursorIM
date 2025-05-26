package constants

// 用户状态常量
const (
	UserStatusOnline   = "online"    // 用户在线
	UserStatusOffline  = "offline"   // 用户离线
	UserStatusHTTPOnly = "http_only" // 用户仅HTTP连接
)

// 连接类型常量
const (
	ConnectionTypeHTTP      = "http"
	ConnectionTypeWebSocket = "websocket"
	ConnectionTypeTCP       = "tcp"
)

// 消息类型常量
const (
	MessageTypeText   = "text"
	MessageTypeImage  = "image"
	MessageTypeFile   = "file"
	MessageTypePing   = "ping"
	MessageTypePong   = "pong"
	MessageTypeStatus = "status"
)

// 会话类型常量
const (
	ConversationTypePrivate = 0 // 单聊
	ConversationTypeGroup   = 1 // 群聊
)

// 群组角色常量
const (
	GroupRoleMember = 0 // 普通成员
	GroupRoleAdmin  = 1 // 管理员/群主
)

// 好友状态常量
const (
	FriendshipStatusPending  = 0 // 待确认
	FriendshipStatusAccepted = 1 // 已接受
)

// 时间常量
const (
	StatusExpirationTime = 600 // 10分钟，单位秒
	TokenExpirationTime  = 24  // 24小时
)

// Redis键前缀
const (
	RedisKeyUserStatus      = "user:%s:status"
	RedisKeyUserConnections = "user:%s:connections"
	RedisKeyUserLastActive  = "user:%s:last_active"
	RedisKeyOnlineUsers     = "online_users"
	RedisKeyConnection      = "conn:%s:%s" // conn:userID:connectionType
)

// HTTP状态码
const (
	StatusOK                  = 200
	StatusBadRequest          = 400
	StatusUnauthorized        = 401
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusInternalServerError = 500
)

// 错误信息
const (
	ErrInvalidParams     = "参数无效"
	ErrUnauthorized      = "未授权"
	ErrUserNotFound      = "用户不存在"
	ErrPasswordIncorrect = "密码错误"
	ErrUserExists        = "用户已存在"
	ErrFriendExists      = "已经是好友"
	ErrGroupNotFound     = "群组不存在"
	ErrInsufficientPerm  = "权限不足"
)
