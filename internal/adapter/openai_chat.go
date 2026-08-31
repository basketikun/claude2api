package adapter

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"claude2api/internal/service"

	"github.com/gin-gonic/gin"
)

func ListModels(c *gin.Context) {
	now := time.Now().Unix()
	data := make([]gin.H, 0)
	for _, id := range supportedModels {
		for _, model := range []string{id, id + "-thinking"} {
			data = append(data, gin.H{
				"id": model, "object": "model", "created": now, "owned_by": "anthropic",
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

type openAIChatRequest struct {
	Model             string          `json:"model"`
	Messages          []openAIMessage `json:"messages"`
	Stream            bool            `json:"stream"`
	Tools             json.RawMessage `json:"tools"`
	ParallelToolCalls json.RawMessage `json:"parallel_tool_calls"`
	ToolChoice        json.RawMessage `json:"tool_choice"`
	MaxTokens         int             `json:"max_tokens"`
	MaxCompletion     int             `json:"max_completion_tokens"`
	Temperature       *float64        `json:"temperature"`
	TopP              *float64        `json:"top_p"`
	Stop              json.RawMessage `json:"stop"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls"`
	ToolCallID string           `json:"tool_call_id"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// normalizeMessages 归一化消息并抽出图片。
func normalizeOpenAIChatMessages(msgs []openAIMessage) ([]Message, []string) {
	out := make([]Message, 0, len(msgs))
	var images []string
	for _, m := range msgs {
		text, imgs := flattenContent(m.Content)
		images = append(images, imgs...)
		am := Message{Role: m.Role, Content: text, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			am.ToolCalls = append(am.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
		out = append(out, am)
	}
	return out, images
}

// flattenContent 压平多模态 content。
func flattenContent(raw json.RawMessage) (string, []string) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return "", nil
	}
	var sb strings.Builder
	var images []string
	for _, item := range arr {
		switch item["type"] {
		case "text":
			if t, ok := item["text"].(string); ok {
				sb.WriteString(t)
				sb.WriteString("\n\n")
			}
		case "image_url":
			var source string
			switch image := item["image_url"].(type) {
			case string:
				source = image
			case map[string]any:
				source, _ = image["url"].(string)
			}
			if source = normalizeImageSource(source, ""); source != "" {
				images = append(images, source)
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n"), images
}

func OpenAIChat(c *gin.Context) {
	var req openAIChatRequest
	raw, err := bindRequest(c, &req)
	if err != nil {
		apiError(c, http.StatusBadRequest, "无效请求体: "+err.Error())
		return
	}
	if len(req.Messages) == 0 {
		apiError(c, http.StatusBadRequest, "messages 不能为空")
		return
	}
	model := modelOrDefault(req.Model)
	msgs, images := normalizeOpenAIChatMessages(req.Messages)

	tools := parseTools(req.Tools)
	prompt, err := buildPrompt(msgs, images, tools, allowParallel(req.ParallelToolCalls), req.ToolChoice)
	if err != nil {
		apiError(c, http.StatusBadRequest, "图片处理失败: "+err.Error())
		return
	}
	prompt.RawRequest = raw
	prompt.MaxTokens, prompt.Temperature, prompt.TopP, prompt.Stop = req.MaxTokens, req.Temperature, req.TopP, parseStops(req.Stop)
	if prompt.MaxTokens == 0 {
		prompt.MaxTokens = req.MaxCompletion
	}
	if len(tools) > 0 {
		if req.Stream {
			openAIToolStream(c, model, prompt)
		} else {
			openAIToolNonStream(c, model, prompt)
		}
		return
	}

	if req.Stream {
		openAIStream(c, model, prompt)
	} else {
		openAINonStream(c, model, prompt)
	}
}

func openAINonStream(c *gin.Context, model string, prompt service.Prompt) {
	content, ok := runNonStream(c, "chat/completions", model, prompt)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":      "chatcmpl-" + shortID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []gin.H{{
			"index":         0,
			"message":       gin.H{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
		"usage": gin.H{
			"prompt_tokens":     tokenCount(prompt.Text),
			"completion_tokens": tokenCount(content),
			"total_tokens":      tokenCount(prompt.Text) + tokenCount(content),
		},
	})
}

type chatStream struct {
	*sseWriter
	id      string
	model   string
	created int64
}

func newChatStream(c *gin.Context, model string) *chatStream {
	return &chatStream{newSSE(c), "chatcmpl-" + shortID(), model, time.Now().Unix()}
}

func (s *chatStream) send(delta gin.H, finish any) {
	s.data(gin.H{
		"id": s.id, "object": "chat.completion.chunk", "created": s.created, "model": s.model,
		"choices": []gin.H{{"index": 0, "delta": delta, "finish_reason": finish}},
	})
}

func openAIStream(c *gin.Context, model string, prompt service.Prompt) {
	stream := newChatStream(c, model)
	stream.send(gin.H{"role": "assistant"}, nil)
	_, err := runAndCollect("chat/completions", model, true, prompt, func(text string) {
		stream.send(gin.H{"content": text}, nil)
	})
	if err != nil {
		stream.send(gin.H{"content": "\n[错误] " + err.Error()}, nil)
	}
	stream.send(gin.H{}, "stop")
	stream.done()
}

// openAIToolNonStream 带 tools 的非流式：解析标签协议后输出 tool_calls 或最终答案。
func openAIToolNonStream(c *gin.Context, model string, prompt service.Prompt) {
	raw, ok := runNonStream(c, "chat/completions", model, prompt)
	if !ok {
		return
	}

	// 容错解析：上游不遵守标签协议时降级为纯文本，不报错。
	parsed := ParseTaggedOutputTolerant(raw)

	message := gin.H{"role": "assistant"}
	finish := "stop"
	if parsed.IsToolCall() {
		toolCalls := make([]gin.H, 0, len(parsed.ToolCalls))
		for _, tc := range parsed.ToolCalls {
			toolCalls = append(toolCalls, gin.H{
				"id":   newToolCallID(),
				"type": "function",
				"function": gin.H{
					"name":      tc.Name,
					"arguments": argsJSON(tc.Arguments),
				},
			})
		}
		if parsed.Thinking != "" {
			message["content"] = "<think>" + parsed.Thinking + "</think>"
		} else {
			message["content"] = nil
		}
		message["tool_calls"] = toolCalls
		finish = "tool_calls"
	} else {
		message["content"] = formatOpenAITaggedAnswer(parsed)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      "chatcmpl-" + shortID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []gin.H{{
			"index":         0,
			"message":       message,
			"finish_reason": finish,
		}},
		"usage": gin.H{
			"prompt_tokens":     tokenCount(prompt.Text),
			"completion_tokens": tokenCount(raw),
			"total_tokens":      tokenCount(prompt.Text) + tokenCount(raw),
		},
	})
}

// openAIToolStream 带 tools 的流式：把标签事件转成 OpenAI delta。
func openAIToolStream(c *gin.Context, model string, prompt service.Prompt) {
	stream := newChatStream(c, model)
	stopReason := "stop"
	toolIndex := 0

	_, err := runTaggedStream("chat/completions", model, prompt, func(ev TaggedStreamEvent) {
		switch ev.Type {
		case EventMessageStart:
			stream.send(gin.H{"role": "assistant", "content": ""}, nil)
		case EventBlockStart:
			if ev.BlockType == BlockThinking {
				stream.send(gin.H{"content": "<think>"}, nil)
			}
		case EventBlockDelta:
			if ev.Text != "" {
				stream.send(gin.H{"content": ev.Text}, nil)
			}
		case EventBlockEnd:
			if ev.BlockType == BlockThinking {
				stream.send(gin.H{"content": "</think>"}, nil)
			}
		case EventToolCall:
			stopReason = "tool_calls"
			// 先发出带 id/name 的分片，再发出 arguments 分片。
			stream.send(gin.H{"tool_calls": []gin.H{{
				"index":    toolIndex,
				"id":       newToolCallID(),
				"type":     "function",
				"function": gin.H{"name": ev.Name, "arguments": ""},
			}}}, nil)
			if args := argsJSON(ev.Arguments); args != "" {
				stream.send(gin.H{"tool_calls": []gin.H{{
					"index":    toolIndex,
					"function": gin.H{"arguments": args},
				}}}, nil)
			}
			toolIndex++
		}
	})
	if err != nil {
		stream.send(gin.H{"content": "\n[错误] " + err.Error()}, nil)
	}
	stream.send(gin.H{}, stopReason)
	stream.done()
}
