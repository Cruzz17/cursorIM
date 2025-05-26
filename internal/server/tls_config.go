package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// TLSConfig TLS配置
type TLSConfig struct {
	CertFile string `json:"cert_file"` // 证书文件路径
	KeyFile  string `json:"key_file"`  // 私钥文件路径
	Enabled  bool   `json:"enabled"`   // 是否启用TLS
}

// NewTLSConfig 创建TLS配置
func NewTLSConfig(certFile, keyFile string, enabled bool) *TLSConfig {
	return &TLSConfig{
		CertFile: certFile,
		KeyFile:  keyFile,
		Enabled:  enabled,
	}
}

// GetTLSConfig 获取标准TLS配置
func (c *TLSConfig) GetTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:               tls.VersionTLS12, // 最低TLS 1.2
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			// 推荐的安全加密套件
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}
}

// StartHTTPSServer 启动HTTPS服务器
func (c *TLSConfig) StartHTTPSServer(router *gin.Engine, addr string) error {
	if !c.Enabled {
		log.Printf("TLS未启用，启动HTTP服务器: %s", addr)
		return http.ListenAndServe(addr, router)
	}

	// 配置HTTPS服务器
	server := &http.Server{
		Addr:      addr,
		Handler:   router,
		TLSConfig: c.GetTLSConfig(),

		// 安全配置
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("启动HTTPS服务器: %s", addr)
	log.Printf("使用证书: %s", c.CertFile)

	return server.ListenAndServeTLS(c.CertFile, c.KeyFile)
}

// ValidateCertificates 验证证书文件
func (c *TLSConfig) ValidateCertificates() error {
	if !c.Enabled {
		return nil
	}

	_, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return fmt.Errorf("验证TLS证书失败: %w", err)
	}

	log.Printf("TLS证书验证成功: %s", c.CertFile)
	return nil
}

// GenerateSelfSignedCert 生成自签名证书（开发用）
func GenerateSelfSignedCert(certFile, keyFile string) error {
	// 注意：生产环境应使用CA签发的证书
	log.Println("⚠️ 警告: 生成自签名证书仅用于开发测试!")
	log.Println("生产环境请使用CA签发的证书")

	// 这里可以添加自签名证书生成逻辑
	// 或者提示用户使用 openssl 命令生成

	return fmt.Errorf("请手动生成证书或使用CA签发的证书")
}
