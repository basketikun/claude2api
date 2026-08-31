package service

import (
	"os"
	"testing"
)

func testClaude(t *testing.T) *ClaudeAI {
	t.Helper()
	key := os.Getenv("CLAUDE_SESSION_KEY")
	if key == "" {
		t.Skip("set CLAUDE_SESSION_KEY to run upstream integration tests")
	}
	client := NewClaudeAI(key, os.Getenv("CLAUDE_PROXY"), key)
	if err := client.WarmUp(); err != nil {
		t.Fatalf("WarmUp 失败: %v", err)
	}
	return client
}

func TestGetUserInfo(t *testing.T) {
	userInfo, err := testClaude(t).GetUserInfo()
	if err != nil {
		t.Fatalf("GetUserInfo 失败: %v", err)
	}
	t.Logf("user_info: %+v", userInfo)
}

func TestSendMessage(t *testing.T) {
	claudeAI := testClaude(t)
	_, err := claudeAI.GetUserInfo()
	if err != nil {
		t.Fatalf("GetUserInfo 失败: %v", err)
	}

	for i := 0; i < 3; i++ {
		convID, err := claudeAI.CreateConversation("claude-sonnet-5", false)
		if err != nil {
			t.Fatalf("CreateConversation 失败: %v", err)
		}
		var reply string
		status, err := claudeAI.SendMessage(convID, "claude-sonnet-5", Prompt{Text: "只回复 OK"}, nil, nil, func(s string) { reply += s })
		if err != nil {
			t.Fatalf("第 %d 次 SendMessage 失败: status=%d err=%v", i+1, status, err)
		}
		t.Logf("第 %d 次 status=%d reply=%s", i+1, status, reply)
	}
}

func TestUploadFile(t *testing.T) {
	claudeAI := testClaude(t)
	_, err := claudeAI.GetUserInfo()
	if err != nil {
		t.Fatalf("GetUserInfo 失败: %v", err)
	}

	image := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

	fileUUIDs, err := claudeAI.UploadFile([]string{image})
	if err != nil {
		t.Fatalf("UploadFile 失败: %v", err)
	}
	if len(fileUUIDs) == 0 {
		t.Fatalf("UploadFile 未返回 file_uuid")
	}
	t.Logf("file_uuids: %v", fileUUIDs)
}
