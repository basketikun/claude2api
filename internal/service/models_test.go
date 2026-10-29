package service

import (
	"encoding/json"
	"os"
	"testing"

	"claude2api/internal/repository"
)

func TestModelsFromCapturedFreeAccount(t *testing.T) {
	body, err := os.ReadFile("testdata/account_models.json")
	if err != nil {
		t.Fatal(err)
	}
	var account ClaudeAccount
	if err := json.Unmarshal(body, &account); err != nil {
		t.Fatal(err)
	}
	info := account.UserInfo()
	if info.SubscriptionPlan != "claude_free" {
		t.Fatalf("套餐识别错误：%s", info.SubscriptionPlan)
	}
	stored := &repository.Account{SubscriptionPlan: info.SubscriptionPlan, Models: info.Models}
	if models := PublicAccountView(stored).Models; len(models) != 5 {
		t.Fatalf("Free 套餐可用模型数量 = %d，期望 5", len(models))
	}
	for _, model := range []string{"claude-sonnet-5-5", "claude-sonnet-4-6-thinking", "claude-haiku-5-5", "claude-haiku-4-5-20251001-thinking"} {
		if !AccountSupportsModel(stored, model) {
			t.Errorf("遗漏可用模型 %s", model)
		}
	}
	for _, model := range []string{"claude-fable-5-1", "claude-fable-5-1-thinking", "claude-opus-5-5", "claude-opus-5-5-thinking", "claude-opus-4-5-20251101", "claude-3-opus-20240229", "claude-3-opus-20240229-thinking", "unknown"} {
		if AccountSupportsModel(stored, model) {
			t.Errorf("错误开放模型 %s", model)
		}
	}
	stored.SubscriptionPlan = ""
	if len(PublicAccountView(stored).Models) != 0 || AccountSupportsModel(stored, "claude-sonnet-4-6") {
		t.Fatal("未同步套餐的账号不能开放模型")
	}
}
