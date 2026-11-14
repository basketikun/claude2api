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

func CloseDB() error {
	connection, err := db.DB()
	if err != nil {
		return err
	}
	return connection.Close()
}

// InitDB 初始化数据库。
func InitDB() error {
	if err := os.MkdirAll(config.DataDir(), 0o700); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	_ = os.Chmod(config.DataDir(), 0o700)
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
	if err := os.Chmod(config.DBPath(), 0o600); err != nil {
		return fmt.Errorf("收紧数据库权限失败: %w", err)
	}
	if err := conn.AutoMigrate(&Account{}, &APIKey{}, &APILog{}, &UsageRecord{}); err != nil {
		return fmt.Errorf("建表失败: %w", err)
	}
	db = conn
	if err := migrateAPIKeys(); err != nil {
		return err
	}
	return nil
}

func migrateAPIKeys() error {
	var legacy []struct {
		ID  int64
		Key string
	}
	if err := db.Table("api_keys").Select("id, key").Find(&legacy).Error; err != nil {
		// New databases have no legacy key column.
		return nil
	}
	for _, row := range legacy {
		if row.Key == "" {
			continue
		}
		if err := db.Model(&APIKey{}).Where("id = ?", row.ID).Updates(map[string]any{"key_hash": hashKey(row.Key), "key_prefix": prefixKey(row.Key)}).Error; err != nil {
			return fmt.Errorf("迁移 API 密钥失败: %w", err)
		}
	}
	return db.Exec("UPDATE api_keys SET key = ''").Error
}

// nowUTC 返回 UTC 时间戳。
func nowUTC() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}
