package service

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"claude2api/internal/repository"

	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

type usageReader struct {
	io.ReadCloser
	writer *io.PipeWriter
	done   chan struct{}
}

// CaptureAccountUsage 逐事件记录用量，原始响应字节保持不变。
func CaptureAccountUsage(body io.ReadCloser, email string) io.ReadCloser {
	if email == "" {
		return body
	}
	reader, writer := io.Pipe()
	stream := &usageReader{ReadCloser: body, writer: writer, done: make(chan struct{})}
	go func() {
		defer close(stream.done)
		defer reader.Close()
		decoder := ssestream.NewDecoder(&http.Response{Body: reader})
		for decoder.Next() {
			var event struct {
				Type         string                   `json:"type"`
				MessageLimit *repository.AccountUsage `json:"message_limit"`
			}
			if err := json.Unmarshal(decoder.Event().Data, &event); err != nil {
				if event.Type == "message_limit" || decoder.Event().Type == "message_limit" {
					slog.Warn("[账号用量] 解析失败", "email", email, "response", string(decoder.Event().Data), "err", err)
				}
				continue
			}
			if event.Type != "message_limit" || event.MessageLimit == nil || len(event.MessageLimit.Windows) == 0 {
				continue
			}
			if err := repository.UpdateAccountUsage(email, event.MessageLimit, time.Now().UTC()); err != nil {
				slog.Warn("[账号用量] 保存失败", "email", email, "usage", event.MessageLimit, "err", err)
			}
		}
		if err := decoder.Err(); err != nil {
			slog.Warn("[账号用量] 读取失败", "email", email, "err", err)
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
