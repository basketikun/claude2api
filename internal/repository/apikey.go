package repository

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// APIKey 是访问密钥。
type APIKey struct {
	ID        int64  `json:"id" gorm:"primaryKey"`
	Key       string `json:"key,omitempty" gorm:"-"`
	KeyHash   string `json:"-" gorm:"uniqueIndex;column:key_hash"`
	KeyPrefix string `json:"prefix" gorm:"not null;default:''"`
	Name      string `json:"name" gorm:"not null;default:''"`
	Enabled   bool   `json:"enabled" gorm:"not null;default:true"`
	CreatedAt string `json:"created_at" gorm:"not null;default:''"`
}

// GenerateAPIKey 生成随机密钥。
func GenerateAPIKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(b), nil
}

// ListAPIKeys 返回全部密钥。
func ListAPIKeys() []APIKey {
	var out []APIKey
	if err := db.Order("id ASC").Find(&out).Error; err != nil {
		return []APIKey{}
	}
	for i := range out {
		out[i].Key = ""
	}
	return out
}

// CreateAPIKey 新建密钥。
func CreateAPIKey(name, value string) (APIKey, error) {
	if value == "" {
		var err error
		if value, err = GenerateAPIKey(); err != nil {
			return APIKey{}, err
		}
	}
	k := APIKey{KeyHash: hashKey(value), KeyPrefix: prefixKey(value), Name: name, Enabled: true, CreatedAt: nowUTC()}
	if err := db.Create(&k).Error; err != nil {
		return APIKey{}, err
	}
	k.Key = value
	return k, nil
}

// DeleteAPIKey 删除密钥。
func DeleteAPIKey(id int64) int {
	res := db.Delete(&APIKey{}, id)
	if res.Error != nil {
		return 0
	}
	return int(res.RowsAffected)
}

// ValidateAPIKey 校验密钥。
func ValidateAPIKey(value string) bool {
	if value == "" {
		return false
	}
	var cnt int64
	if err := db.Model(&APIKey{}).Where("key_hash = ? AND enabled = 1", hashKey(value)).Count(&cnt).Error; err != nil {
		return false
	}
	return cnt > 0
}

func hashKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func prefixKey(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:8] + "…"
}
