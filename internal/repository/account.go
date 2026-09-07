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
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	
	// Session limits tracking
	SessionLimits    *SessionLimits    `json:"session_limits,omitempty" gorm:"serializer:json"`
	LastLimitUpdate  *time.Time        `json:"last_limit_update,omitempty"`
}

// SessionLimits holds Claude session limit information
type SessionLimits struct {
	Type                    string                 `json:"type"`
	ResetsAt                *int64                 `json:"resets_at,omitempty"`
	Remaining               *int                   `json:"remaining,omitempty"`
	PerModelLimit           *int                   `json:"per_model_limit,omitempty"`
	RepresentativeClaim     string                 `json:"representative_claim,omitempty"`
	OverageDisabledReason   string                 `json:"overage_disabled_reason,omitempty"`
	OverageInUse            bool                   `json:"overage_in_use"`
	Windows                 map[string]LimitWindow `json:"windows,omitempty"`
	Resolved                *ResolvedLimit         `json:"resolved,omitempty"`
}

// LimitWindow represents a time window for limits
type LimitWindow struct {
	Status       string  `json:"status"`
	ResetsAt     int64   `json:"resets_at"`
	Utilization  float64 `json:"utilization"`
}

// ResolvedLimit represents the resolved limit information
type ResolvedLimit struct {
	Status          string  `json:"status"`
	Limit           *Limit  `json:"limit,omitempty"`
	Spend           *string `json:"spend,omitempty"`
	DisabledReason  string  `json:"disabled_reason,omitempty"`
	Notice          *string `json:"notice,omitempty"`
}

// Limit represents a specific limit
type Limit struct {
	Kind      string  `json:"kind"`
	Group     string  `json:"group"`
	Percent   int     `json:"percent"`
	Severity  string  `json:"severity"`
	ResetsAt  string  `json:"resets_at"`
	Scope     *string `json:"scope,omitempty"`
	IsActive  bool    `json:"is_active"`
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
		UpdateAll: true,
	}).Create(a).Error
}

// UpdateAccount 修改指定账号。
func UpdateAccount(email string, mutate func(*Account)) bool {
	a := AccountByEmail(email)
	if a == nil {
		return false
	}
	mutate(a)
	return db.Save(a).Error == nil
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

// UpdateAccountSessionLimits updates session limit information for an account
func UpdateAccountSessionLimits(email string, limits *SessionLimits) bool {
	now := time.Now()
	return UpdateAccount(email, func(a *Account) {
		a.SessionLimits = limits
		a.LastLimitUpdate = &now
	})
}
