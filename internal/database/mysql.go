package database

import (
	"log"
	"time"

	"cursorIM/internal/config"
	"cursorIM/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB() (*gorm.DB, error) {
	// 设置日志
	newLogger := logger.New(
		log.New(log.Writer(), "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	// 连接数据库
	db, err := gorm.Open(mysql.Open(config.GlobalConfig.Database.MySQL.DSN), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, err
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 设置最大空闲连接数
	sqlDB.SetMaxIdleConns(10)
	// 设置最大打开连接数
	sqlDB.SetMaxOpenConns(100)
	// 设置连接最大生存时间
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移数据库结构
	if err := model.SetupDatabase(db); err != nil {
		return nil, err
	}

	DB = db
	return db, nil
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB
}
