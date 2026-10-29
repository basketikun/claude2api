package service

import (
	"encoding/json"
	"os"
	"testing"

	"claude2api/internal/repository"
)

func TestModelsFromCapturedAccount(t *testing.T) {
	body, err := os.ReadFile("testdata/account_models.json")
	if err != nil {
		t.Fatal(err)
	}
	var account ClaudeAccount
	if err := json.Unmarshal(body, &account); err != nil {
		t.Fatal(err)
	}
	info := account.UserInfo()
	if len(info.Models) != 13 {
		t.Fatalf("可用模型数量 = %d，期望 13", len(info.Models))
	}
	stored := &repository.Account{Models: info.Models}
	for _, model := range []string{"claude-sonnet-5-5", "claude-sonnet-4-6-thinking", "claude-3-opus-20240229"} {
		if !AccountSupportsModel(stored, model) {
			t.Errorf("遗漏可用模型 %s", model)
		}
	}
	for _, model := range []string{"claude-opus-4-5-20251101", "claude-3-opus-20240229-thinking", "unknown"} {
		if AccountSupportsModel(stored, model) {
			t.Errorf("错误开放模型 %s", model)
		}
	}
}
