package repository

import (
	"time"

	"gorm.io/gorm/clause"
)

// Account 是一条账号记录。
type Account struct {
	ID               uint              `json:"-" gorm:"primaryKey"`
	Email            string            `json:"email" gorm:"uniqueIndex;not null"`
	OrgUUID          string            `json:"org_uuid" gorm:"column:org_uuid"`
	Cookies          map[string]string `json:"-" gorm:"serializer:json"`
	Status           string            `json:"status,omitempty"`
	SubscriptionPlan string            `json:"subscription_plan"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	Usage            *AccountUsage     `json:"usage" gorm:"serializer:json;type:text"`
	UsageUpdatedAt   *time.Time        `json:"usage_updated_at"`
	Models           []AccountModel    `json:"models" gorm:"serializer:json;type:text"`
	ModelsUpdatedAt  *time.Time        `json:"models_updated_at"`
}

type AccountModel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Thinking      bool   `json:"thinking"`
	ContextWindow int    `json:"context_window"`
}

// AccountUsage 保存最近一次对话返回的额度窗口。
type AccountUsage struct {
	Windows map[string]UsageWindow `json:"windows"`
}

type UsageWindow struct {
	Status      string  `json:"status"`
	ResetsAt    int64   `json:"resets_at"`
	Utilization float64 `json:"utilization"`
}

// LoadAccounts 读取全部账号。
func LoadAccounts() []Account {
	var out []Account
	if err := db.Order("id ASC").Find(&out).Error; err != nil {
		return []Account{}
	}
	return out
}

// AccountByEmail 按邮箱取账号。
func AccountByEmail(email string) *Account {
	if email == "" {
		return nil
	}
	var a Account
	if db.Where("email = ?", email).First(&a).Error != nil {
		return nil
	}
	return &a
}

// UpsertAccount 写入或更新账号。
func UpsertAccount(a *Account) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"org_uuid", "cookies", "status", "subscription_plan", "updated_at", "models", "models_updated_at"}),
	}).Create(a).Error
}

// UpdateAccount 修改指定账号。
func UpdateAccount(email string, mutate func(*Account)) bool {
	a := AccountByEmail(email)
	if a == nil {
		return false
	}
	mutate(a)
	return db.Omit("Usage", "UsageUpdatedAt").Save(a).Error == nil
}

// UpdateAccountUsage 只更新额度，避免与账号刷新互相覆盖。
func UpdateAccountUsage(email string, usage *AccountUsage, capturedAt time.Time) error {
	return db.Model(&Account{}).
		Where("email = ? AND (usage_updated_at IS NULL OR usage_updated_at < ?)", email, capturedAt).
		Select("Usage", "UsageUpdatedAt").
		UpdateColumns(&Account{Usage: usage, UsageUpdatedAt: &capturedAt}).Error
}

// DeleteAccount 删除账号。
func DeleteAccount(email string) int {
	res := db.Where("email = ?", email).Delete(&Account{})
	if res.Error != nil {
		return 0
	}
	return int(res.RowsAffected)
}

// DeleteAccountsByStatus 按状态删除账号。
func DeleteAccountsByStatus(statuses []string) []string {
	removed := make([]string, 0)
	for _, a := range LoadAccounts() {
		for _, s := range statuses {
			if a.Status == s {
				if db.Where("email = ?", a.Email).Delete(&Account{}).Error == nil {
					removed = append(removed, a.Email)
				}
				break
			}
		}
	}
	return removed
}
