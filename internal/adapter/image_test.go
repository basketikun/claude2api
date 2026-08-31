package adapter

import "testing"

func TestImageInput(t *testing.T) {
	if raw, err := decodeImageBase64("aGk"); err != nil || string(raw) != "hi" {
		t.Fatalf("无填充 Base64 解码失败: %q %v", raw, err)
	}
	if _, err := publicImageURL("http://127.0.0.1/image.png"); err == nil {
		t.Fatal("不应允许内网图片 URL")
	}
}
