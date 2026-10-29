package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// TextUsage 只估算实际可见的文本，不代表上游计费 Token。
type TextUsage struct {
	InputTokens    int `json:"input_tokens"`
	OutputTokens   int `json:"output_tokens"`
	ThinkingTokens int `json:"thinking_tokens"`
}

type UsageRecord struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
	Account      string    `json:"account" gorm:"index"`
	Model        string    `json:"model" gorm:"index"`
	Endpoint     string    `json:"endpoint"`
	Success      bool      `json:"success"`
	DurationMs   int64     `json:"duration_ms"`
	FirstTokenMs int64     `json:"first_token_ms"`
	TextUsage
}

type UsageSummary struct {
	Requests          int64   `json:"requests"`
	Successes         int64   `json:"successes"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	ThinkingTokens    int64   `json:"thinking_tokens"`
	TotalTokens       int64   `json:"total_tokens"`
	AverageMs         float64 `json:"average_ms"`
	FirstTokenMs      float64 `json:"first_token_ms"`
	FirstTokenSamples int64   `json:"first_token_samples"`
	LastUsed          string  `json:"last_used"`
}

type UsageGroup struct {
	Name string `json:"name"`
	UsageSummary
}

type UsageOverview struct {
	Summary        UsageSummary `json:"summary"`
	Daily          []UsageGroup `json:"daily"`
	Groups         []UsageGroup `json:"groups"`
	Total          int64        `json:"total"`
	Endpoints      []UsageGroup `json:"endpoints"`
	Activity       []UsageGroup `json:"activity"`
	Accounts       int64        `json:"accounts"`
	ActiveAccounts int64        `json:"active_accounts"`
}

func InsertUsage(record UsageRecord) error {
	if db == nil {
		return nil
	}
	return db.Create(&record).Error
}

func LoadUsage(days, timezoneOffset int, account, group string, offset, limit int) (UsageOverview, error) {
	result := UsageOverview{Daily: []UsageGroup{}, Groups: []UsageGroup{}, Endpoints: []UsageGroup{}, Activity: []UsageGroup{}}
	base := db.Model(&UsageRecord{})
	if account != "" {
		base = base.Where("account = ?", account)
	}
	base = base.Session(&gorm.Session{})
	query := base
	zone := time.Duration(timezoneOffset) * time.Minute
	midnight := time.Now().UTC().Add(-zone).Truncate(24 * time.Hour).Add(zone)
	if days > 0 {
		query = query.Where("created_at >= ?", midnight.AddDate(0, 0, 1-days))
	}
	query = query.Session(&gorm.Session{})
	columns := "COUNT(*) AS requests, COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) AS successes, COALESCE(SUM(input_tokens), 0) AS input_tokens, COALESCE(SUM(output_tokens), 0) AS output_tokens, COALESCE(SUM(thinking_tokens), 0) AS thinking_tokens, COALESCE(SUM(input_tokens + output_tokens + thinking_tokens), 0) AS total_tokens, COALESCE(AVG(duration_ms), 0) AS average_ms, COALESCE(AVG(CASE WHEN first_token_ms > 0 THEN first_token_ms END), 0) AS first_token_ms, COUNT(CASE WHEN first_token_ms > 0 THEN 1 END) AS first_token_samples, COALESCE(MAX(created_at), '') AS last_used"
	if err := query.Select(columns).Scan(&result.Summary).Error; err != nil {
		return result, err
	}
	if err := query.Select("date(created_at, ?) AS name, "+columns, fmt.Sprintf("%+d minutes", -timezoneOffset)).Group("name").Order("name ASC").Scan(&result.Daily).Error; err != nil {
		return result, err
	}
	if err := query.Distinct(group).Count(&result.Total).Error; err != nil {
		return result, err
	}
	if err := query.Select(group + " AS name, " + columns).Group(group).Order("total_tokens DESC, name ASC").Offset(offset).Limit(limit).Scan(&result.Groups).Error; err != nil {
		return result, err
	}
	if err := query.Select("endpoint AS name, " + columns).Group("endpoint").Order("requests DESC").Scan(&result.Endpoints).Error; err != nil {
		return result, err
	}
	if err := base.Where("created_at >= ?", midnight.AddDate(0, 0, -179)).Select("date(created_at, ?) AS name, "+columns, fmt.Sprintf("%+d minutes", -timezoneOffset)).Group("name").Order("name ASC").Scan(&result.Activity).Error; err != nil {
		return result, err
	}
	if err := db.Model(&Account{}).Count(&result.Accounts).Error; err != nil {
		return result, err
	}
	err := db.Model(&Account{}).Where("status = ?", "active").Count(&result.ActiveAccounts).Error
	return result, err
}
