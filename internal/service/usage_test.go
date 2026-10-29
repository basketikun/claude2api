package service

import (
	"bytes"
	"io"
	"os"
	"testing"
	"testing/iotest"
	"time"

	"claude2api/internal/repository"
)

func TestCapturedCompletionUsage(t *testing.T) {
	body, err := os.ReadFile("testdata/completion.sse")
	if err != nil {
		t.Fatal(err)
	}
	for _, complete := range []bool{true, false} {
		input := body
		if !complete {
			input = body[:bytes.Index(body, []byte("event: message_stop"))]
		}
		record := &repository.UsageRecord{CreatedAt: time.Now().UTC()}
		stream := CaptureAccountUsage(io.NopCloser(iotest.OneByteReader(bytes.NewReader(input))), record, 28)
		output, err := io.ReadAll(stream)
		if err != nil {
			t.Fatal(err)
		}
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(input, output) {
			t.Fatal("采集改变了对话流")
		}
		if record.InputTokens != 7 || record.OutputTokens != 4 || record.ThinkingTokens != 0 || record.Model != "claude-sonnet-4-6" || record.Success != complete {
			t.Fatalf("采集结果不符合真实样本，complete=%v record=%+v", complete, record)
		}
	}
}
