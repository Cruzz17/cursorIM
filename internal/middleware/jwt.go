package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cursorIM/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWT 中间件验证 token
func JWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证token"})
			c.Abort()
			return
		}

		// 验证 token 格式
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token格式"})
			c.Abort()
			return
		}

		// 验证 token
		userID, err := ValidateToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token"})
			c.Abort()
			return
		}

		// 将用户ID存储在上下文中
		c.Set("userID", userID)
		c.Next()
	}
}

// ValidateToken 验证JWT token，返回用户ID
func ValidateToken(tokenString string) (string, error) {
	// 解析token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}

		return []byte(config.GlobalConfig.JWT.Secret), nil
	})

	if err != nil {
		return "", err
	}

	// 验证token是否有效
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// 检查token是否过期
		exp, ok := claims["exp"].(float64)
		if !ok {
			return "", errors.New("无效的过期时间")
		}

		if time.Unix(int64(exp), 0).Before(time.Now()) {
			return "", errors.New("token已过期")
		}

		// 获取用户ID
		userID, ok := claims["user_id"].(string)
		if !ok {
			return "", errors.New("无效的用户ID")
		}

		return userID, nil
	}

	return "", errors.New("无效的token")
}

// GenerateToken 生成 JWT token
func GenerateToken(userID string) (string, error) {
	// 设置过期时间
	expire := time.Now().Add(time.Hour * 24) // 令牌有效期24小时

	// 创建声明
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expire.Unix(),
		"iat":     time.Now().Unix(),
	}

	// 创建token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名token
	return token.SignedString([]byte(config.GlobalConfig.JWT.Secret))
}
