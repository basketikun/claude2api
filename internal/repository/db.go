package repository

import (
	"fmt"
	"os"
	"time"

	"claude2api/internal/config"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

// InitDB 初始化数据库。
func InitDB() error {
	if err := os.MkdirAll(config.DataDir(), 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	// 等待并发写，启用 WAL。
	dsn := config.DBPath() + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	// SQLite 写操作串行，单连接更稳。
	if sqlDB, err := conn.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := conn.AutoMigrate(&Account{}, &APIKey{}, &APILog{}); err != nil {
		return fmt.Errorf("建表失败: %w", err)
	}
	db = conn
	return nil
}

// nowUTC 返回 UTC 时间戳。
func nowUTC() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}
