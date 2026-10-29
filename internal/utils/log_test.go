package utils

import (
	"strings"
	"testing"
)

func TestRequestLogOmitsMediaAndKeepsText(t *testing.T) {
	input := []byte(`{"messages":[{"content":[{"type":"text","text":"keep this text"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="}},{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}},{"type":"image_url","image_url":{"url":"aGVsbG8="}}]}]}`)
	result, err := RequestLog(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result), "aGVsbG8=") || !strings.Contains(string(result), "keep this text") || !strings.Contains(string(result), "image/png") {
		t.Fatalf("日志脱敏未保留文本或泄露媒体：%s", result)
	}
}
