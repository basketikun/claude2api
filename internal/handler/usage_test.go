package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"claude2api/internal/config"
	"claude2api/internal/repository"

	"github.com/gin-gonic/gin"
)

func TestUsageOverview(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.example.yaml", []byte("admin_password: local-test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config.Load()
	if err := repository.InitDB(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := repository.CloseDB(); err != nil {
			t.Error(err)
		}
	})
	midnight := time.Now().UTC().Add(8 * time.Hour).Truncate(24 * time.Hour).Add(-8 * time.Hour)
	for _, record := range []repository.UsageRecord{
		{Account: "first@example.com", Model: "claude-sonnet-4-6", Endpoint: "mirror", CreatedAt: midnight.Add(time.Hour), Success: true, DurationMs: 100, FirstTokenMs: 20, TextUsage: repository.TextUsage{InputTokens: 8, OutputTokens: 2, ThinkingTokens: 1}},
		{Account: "second@example.com", Model: "claude-sonnet-4-6", Endpoint: "messages", CreatedAt: midnight.Add(-time.Minute), Success: true, DurationMs: 200, FirstTokenMs: 40, TextUsage: repository.TextUsage{InputTokens: 4, OutputTokens: 3}},
		{Account: "first@example.com", Model: "claude-sonnet-4-6", Endpoint: "messages", CreatedAt: midnight.Add(2 * time.Hour), Success: false, DurationMs: 30},
	} {
		if err := repository.InsertUsage(record); err != nil {
			t.Fatal(err)
		}
	}
	repository.InsertAPILog(repository.APILog{Account: "first@example.com", InputTokens: 8, OutputTokens: 2})
	repository.TrimAPILogs(0)
	router := gin.New()
	router.GET("/api/usage", AdminUsage)
	for _, query := range []struct {
		parameters               string
		requests, tokens, groups int64
		rows                     int
	}{
		{"days=0&group=account&offset=0&limit=1", 3, 18, 2, 1},
		{"days=1&group=model&offset=0&limit=10", 2, 11, 1, 1},
		{"days=0&group=model&account=second%40example.com&offset=0&limit=10", 1, 7, 1, 1},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/usage?timezone_offset=-480&"+query.parameters, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("用量读取失败：%d %s", response.Code, response.Body.String())
		}
		var result struct {
			Usage  repository.UsageOverview `json:"usage"`
			Source string                   `json:"token_source"`
			Cache  *int                     `json:"cache_tokens"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Usage.Summary.Requests != query.requests || result.Usage.Summary.TotalTokens != query.tokens || result.Usage.Total != query.groups || len(result.Usage.Groups) != query.rows || result.Source != "estimated" || result.Cache != nil {
			t.Fatalf("聚合结果错误：%s", response.Body.String())
		}
	}
}
