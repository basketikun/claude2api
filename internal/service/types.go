package service

import (
	"encoding/json"
	"slices"

	"claude2api/internal/repository"
)

// UserInfo 是账号信息。
type UserInfo struct {
	Email   string
	OrgUUID string
	Models  []repository.AccountModel
}

type ClaudeAccount struct {
	EmailAddress string `json:"email_address"`
	Memberships  []struct {
		Organization struct {
			UUID   string `json:"uuid"`
			Models []struct {
				ID           string   `json:"model"`
				Name         string   `json:"name"`
				Inactive     bool     `json:"inactive"`
				PaprikaModes []string `json:"paprika_modes"`
				HardLimit    int      `json:"hard_limit"`
			} `json:"claude_ai_bootstrap_models_config"`
		} `json:"organization"`
	} `json:"memberships"`
}

func (account ClaudeAccount) UserInfo() *UserInfo {
	info := &UserInfo{Email: account.EmailAddress, Models: []repository.AccountModel{}}
	if len(account.Memberships) == 0 {
		return info
	}
	organization := account.Memberships[0].Organization
	info.OrgUUID = organization.UUID
	for _, model := range organization.Models {
		if !model.Inactive {
			info.Models = append(info.Models, repository.AccountModel{ID: model.ID, Name: model.Name, Thinking: slices.Contains(model.PaprikaModes, "extended"), ContextWindow: model.HardLimit})
		}
	}
	return info
}

// SseEvent 是 completion SSE 事件。
type SseEvent struct {
	Type         string `json:"type"`
	ContentBlock struct {
		Type string `json:"type"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Prompt 是协议适配层交给上游实现的输入。
type Prompt struct {
	Text        string
	Images      []string
	RawRequest  json.RawMessage
	ForceInline bool
	MaxTokens   int
	Temperature *float64
	TopP        *float64
	Stop        []string
}

type CompletionResult struct {
	Account    string
	StatusCode int
	Usage      *repository.UsageRecord
}

type CompletionError struct {
	StatusCode int
	Err        error
}

func (e *CompletionError) Error() string { return e.Err.Error() }
func (e *CompletionError) Unwrap() error { return e.Err }
