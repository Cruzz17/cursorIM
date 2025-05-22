package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`

	Database struct {
		MySQL struct {
			DSN string `yaml:"dsn"` // Data Source Name
		} `yaml:"mysql"`
	} `yaml:"database"`

	JWT struct {
		Secret string `yaml:"secret"`
		Expire int    `yaml:"expire"` // 过期时间（小时）
	} `yaml:"jwt"`

	Redis struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redisclient"`
}

// GlobalConfig 全局配置
var GlobalConfig = &Config{}

// Init 初始化配置
func Init() error {
	f, err := os.Open("config.yaml")
	if err != nil {
		// 如果配置文件不存在，创建默认配置
		log.Println("配置文件不存在，使用默认配置")
		GlobalConfig = &Config{}
		GlobalConfig.Server.Port = 8082
		GlobalConfig.Database.MySQL.DSN = "root:123456@tcp(127.0.0.1:3306)/im?charset=utf8mb4&parseTime=True&loc=Local"
		GlobalConfig.JWT.Secret = "default_secret_key_for_development"
		GlobalConfig.JWT.Expire = 24

		// 设置默认Redis配置
		GlobalConfig.Redis.Host = "127.0.0.1"
		GlobalConfig.Redis.Port = 6379
		GlobalConfig.Redis.Password = ""
		GlobalConfig.Redis.DB = 0

		return nil
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&GlobalConfig)
	if err != nil {
		return err
	}

	// 确保 JWT Secret 有值
	if GlobalConfig.JWT.Secret == "" {
		GlobalConfig.JWT.Secret = "default_secret_key_for_development"
	}

	// 确保过期时间有值
	if GlobalConfig.JWT.Expire <= 0 {
		GlobalConfig.JWT.Expire = 24
	}

	// 确保Redis配置有值
	if GlobalConfig.Redis.Host == "" {
		GlobalConfig.Redis.Host = "127.0.0.1"
	}
	if GlobalConfig.Redis.Port == 0 {
		GlobalConfig.Redis.Port = 6379
	}

	log.Printf("配置加载成功: Redis=%s:%d", GlobalConfig.Redis.Host, GlobalConfig.Redis.Port)
	return nil
}
