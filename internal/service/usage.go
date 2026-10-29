package service

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"claude2api/internal/repository"
	"claude2api/internal/utils"

	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

type usageReader struct {
	io.ReadCloser
	writer *io.PipeWriter
	done   chan struct{}
}

// CaptureAccountUsage 逐事件记录用量，原始响应字节保持不变。
func CaptureAccountUsage(body io.ReadCloser, record *repository.UsageRecord, inputBytes int) io.ReadCloser {
	reader, writer := io.Pipe()
	stream := &usageReader{ReadCloser: body, writer: writer, done: make(chan struct{})}
	go func() {
		defer close(stream.done)
		defer reader.Close()
		decoder := ssestream.NewDecoder(&http.Response{Body: reader})
		outputBytes, thinkingBytes := 0, 0
		completed, failed := false, false
		for decoder.Next() {
			if len(decoder.Event().Data) == 0 {
				continue
			}
			var event struct {
				Type         string                   `json:"type"`
				MessageLimit *repository.AccountUsage `json:"message_limit"`
				Message      struct {
					Model string `json:"model"`
				} `json:"message"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					Thinking    string `json:"thinking"`
					PartialJSON string `json:"partial_json"`
				} `json:"delta"`
			}
			if err := json.Unmarshal(decoder.Event().Data, &event); err != nil {
				slog.Warn("[账号用量] 解析失败", "email", record.Account, "response", utils.RedactMedia(string(decoder.Event().Data)), "err", err)
				failed = true
				continue
			}
			switch event.Type {
			case "message_start":
				record.Model = event.Message.Model
				record.InputTokens = utils.EstimateTokens(inputBytes)
			case "content_block_delta":
				switch event.Delta.Type {
				case "text_delta":
					outputBytes += len(event.Delta.Text)
				case "thinking_delta":
					thinkingBytes += len(event.Delta.Thinking)
				case "input_json_delta":
					outputBytes += len(event.Delta.PartialJSON)
				}
				if record.FirstTokenMs == 0 && outputBytes+thinkingBytes > 0 {
					record.FirstTokenMs = time.Since(record.CreatedAt).Milliseconds()
				}
			case "message_stop":
				completed = true
			case "error":
				failed = true
			}
			if event.Type == "message_limit" && record.Account != "" && event.MessageLimit != nil && len(event.MessageLimit.Windows) > 0 {
				if err := repository.UpdateAccountUsage(record.Account, event.MessageLimit, time.Now().UTC()); err != nil {
					slog.Warn("[账号用量] 保存失败", "email", record.Account, "usage", event.MessageLimit, "err", err)
				}
			}
		}
		if err := decoder.Err(); err != nil {
			failed = true
			slog.Warn("[账号用量] 读取失败", "email", record.Account, "err", err)
		}
		record.OutputTokens = utils.EstimateTokens(outputBytes)
		record.ThinkingTokens = utils.EstimateTokens(thinkingBytes)
		record.Success = completed && !failed
		record.DurationMs = time.Since(record.CreatedAt).Milliseconds()
		if record.Endpoint == "mirror" {
			if err := repository.InsertUsage(*record); err != nil {
				slog.Error("[用量统计] 保存镜像用量失败", "account", record.Account, "err", err)
			}
		}
	}()
	return stream
}

func (stream *usageReader) Read(p []byte) (int, error) {
	n, err := stream.ReadCloser.Read(p)
	if n > 0 {
		// 采集停止时仍继续透传，不能让用量记录中断对话。
		_, _ = stream.writer.Write(p[:n])
	}
	if err != nil {
		stream.writer.CloseWithError(err)
	}
	return n, err
}

func (stream *usageReader) Close() error {
	err := stream.ReadCloser.Close()
	stream.writer.Close()
	<-stream.done
	return err
}
