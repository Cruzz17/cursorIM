package user

import (
	"context"
	"errors"
	"log"
	"time"

	"cursorIM/internal/database"
	"cursorIM/internal/middleware"
	"cursorIM/internal/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AccountService struct {
	db *gorm.DB
}

func NewAccountService() *AccountService {
	return &AccountService{
		db: database.GetDB(),
	}
}

// Register 注册新用户
func (s *AccountService) Register(ctx context.Context, req *RegisterRequest) (string, error) {
	// 检查用户名是否已存在
	var count int64
	if err := s.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&count).Error; err != nil {
		return "", err
	}
	if count > 0 {
		return "", errors.New("用户名已存在")
	}

	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	// 创建新用户
	user := model.User{
		ID:        uuid.New().String(),
		Username:  req.Username,
		Password:  string(hashedPassword),
		Nickname:  req.Nickname,
		AvatarURL: req.AvatarURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 插入数据库
	if err := s.db.Create(&user).Error; err != nil {
		return "", err
	}

	return user.ID, nil
}

// Login 用户登录
func (s *AccountService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	log.Printf("尝试登录用户: %s", req.Username)

	// 查找用户
	var user model.User
	if err := s.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("用户不存在: %s", req.Username)
			return nil, errors.New("用户不存在")
		}
		log.Printf("查询用户时数据库错误: %v", err)
		return nil, err
	}

	// 验证密码
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		log.Printf("用户 %s 密码验证失败: %v", req.Username, err)
		return nil, errors.New("密码错误")
	}

	// 生成JWT令牌，这里不再需要传递secret
	token, err := middleware.GenerateToken(user.ID)
	if err != nil {
		log.Printf("生成令牌失败: %v", err)
		return nil, err
	}

	log.Printf("用户 %s (ID: %s) 登录成功", req.Username, user.ID)
	return &LoginResponse{
		UserID: user.ID,
		Token:  token,
	}, nil
}

// GetUserByID 通过ID获取用户
func (s *AccountService) GetUserByID(ctx context.Context, userID string) (*UserResponse, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}

	return &UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		AvatarURL: user.AvatarURL,
		CreatedAt: user.CreatedAt,
	}, nil
}

// SearchUsers 搜索用户
func (s *AccountService) SearchUsers(ctx context.Context, query string) ([]*UserResponse, error) {
	log.Printf("执行用户搜索，查询: '%s'", query)

	var users []model.User
	// 使用更宽松的搜索条件，同时搜索用户名、昵称或ID的部分匹配
	result := s.db.Where("username LIKE ? OR nickname LIKE ? OR id LIKE ?",
		"%"+query+"%", "%"+query+"%", "%"+query+"%").Find(&users)

	if result.Error != nil {
		log.Printf("搜索用户时出错: %v", result.Error)
		return nil, result.Error
	}

	log.Printf("找到 %d 个用户", len(users))

	var response []*UserResponse
	for _, user := range users {
		response = append(response, &UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Nickname:  user.Nickname,
			AvatarURL: user.AvatarURL,
			CreatedAt: user.CreatedAt,
		})
	}

	return response, nil
}

// AddFriend 添加好友
func (s *AccountService) AddFriend(ctx context.Context, userID, friendID string) error {
	// 检查好友是否存在
	var friend model.User
	if err := s.db.Where("id = ?", friendID).First(&friend).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("好友不存在")
		}
		return err
	}

	// 检查是否已经是好友
	var count int64
	if err := s.db.Model(&model.Friendship{}).Where("user_id = ? AND friend_id = ?", userID, friendID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("已经是好友")
	}

	// 开始事务
	tx := s.db.Begin()

	// 添加好友关系
	friendship := model.Friendship{
		ID:        uuid.New().String(),
		UserID:    userID,
		FriendID:  friendID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := tx.Create(&friendship).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 添加反向好友关系
	reverseFriendship := model.Friendship{
		ID:        uuid.New().String(),
		UserID:    friendID,
		FriendID:  userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := tx.Create(&reverseFriendship).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 提交事务
	return tx.Commit().Error
}

// GetFriends 获取好友列表
func (s *AccountService) GetFriends(ctx context.Context, userID string) ([]*UserResponse, error) {
	var friends []*UserResponse

	// 查询SQL，通过JOIN获取好友信息
	rows, err := s.db.Raw(`
		SELECT u.id, u.username, u.nickname, u.avatar_url, u.created_at
		FROM users u
		JOIN friendships f ON u.id = f.friend_id
		WHERE f.user_id = ?
	`, userID).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user UserResponse
		var createdAt time.Time
		if err := rows.Scan(&user.ID, &user.Username, &user.Nickname, &user.AvatarURL, &createdAt); err != nil {
			return nil, err
		}
		user.CreatedAt = createdAt
		friends = append(friends, &user)
	}

	return friends, nil
}
