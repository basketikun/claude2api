package adapter

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"claude2api/internal/service"

	"github.com/gin-gonic/gin"
)

// responsesRequest 是 Responses API 请求。
type responsesRequest struct {
	Model        string          `json:"model"`
	Instructions string          `json:"instructions"`
	Input        json.RawMessage `json:"input"`
	Stream       bool            `json:"stream"`
	Tools        json.RawMessage `json:"tools"`
	ToolChoice   json.RawMessage `json:"tool_choice"`
	MaxTokens    int             `json:"max_output_tokens"`
	Temperature  *float64        `json:"temperature"`
	TopP         *float64        `json:"top_p"`
}

func OpenAIResponses(c *gin.Context) {
	var req responsesRequest
	raw, err := bindRequest(c, &req)
	if err != nil {
		apiError(c, http.StatusBadRequest, "无效请求体: "+err.Error())
		return
	}
	msgs, images := normalizeResponseInput(req.Input)
	if req.Instructions != "" {
		msgs = append([]Message{{Role: "system", Content: req.Instructions}}, msgs...)
	}
	if len(msgs) == 0 {
		apiError(c, http.StatusBadRequest, "input 不能为空")
		return
	}
	model := modelOrDefault(req.Model)

	tools := parseTools(req.Tools)
	prompt, err := buildPrompt(msgs, images, tools, true, req.ToolChoice)
	if err != nil {
		apiError(c, http.StatusBadRequest, "图片处理失败: "+err.Error())
		return
	}
	prompt.RawRequest = raw
	prompt.MaxTokens, prompt.Temperature, prompt.TopP = req.MaxTokens, req.Temperature, req.TopP
	if len(tools) > 0 {
		if req.Stream {
			responsesToolStream(c, model, prompt)
		} else {
			responsesToolNonStream(c, model, prompt)
		}
		return
	}

	if req.Stream {
		responsesStream(c, model, prompt)
	} else {
		responsesNonStream(c, model, prompt)
	}
}

func normalizeResponseInput(raw json.RawMessage) ([]Message, []string) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return []Message{{Role: "user", Content: text}}, nil
	}
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil, nil
	}
	var msgs []Message
	var images []string
	for _, item := range items {
		var itemType string
		_ = json.Unmarshal(item["type"], &itemType)

		switch itemType {
		case "function_call":
			// assistant 发起的工具调用历史。
			var name, callID, arguments string
			_ = json.Unmarshal(item["name"], &name)
			_ = json.Unmarshal(item["call_id"], &callID)
			_ = json.Unmarshal(item["arguments"], &arguments)
			msgs = append(msgs, Message{
				Role:      "assistant",
				ToolCalls: []ToolCall{{ID: callID, Name: name, Arguments: arguments}},
			})
			continue
		case "function_call_output":
			var callID, output string
			_ = json.Unmarshal(item["call_id"], &callID)
			_ = json.Unmarshal(item["output"], &output)
			msgs = append(msgs, Message{Role: "tool", Content: output, ToolCallID: callID})
			continue
		}

		var role string
		_ = json.Unmarshal(item["role"], &role)
		if role == "" {
			role = "user"
		}
		content, imgs := flattenResponseContent(item["content"])
		images = append(images, imgs...)
		if content != "" || len(imgs) > 0 {
			msgs = append(msgs, Message{Role: role, Content: content})
		}
	}
	return msgs, images
}

func flattenResponseContent(raw json.RawMessage) (string, []string) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	var parts []map[string]any
	if json.Unmarshal(raw, &parts) != nil {
		return "", nil
	}
	var texts []string
	var images []string
	for _, part := range parts {
		switch part["type"] {
		case "input_text", "output_text", "text":
			if v, ok := part["text"].(string); ok {
				texts = append(texts, v)
			}
		case "input_image", "image_url":
			var source string
			switch image := part["image_url"].(type) {
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
	return strings.Join(texts, "\n\n"), images
}

func responseObject(id, model, status, text string, created int64) gin.H {
	return gin.H{
		"id": id, "object": "response", "created_at": created, "status": status,
		"model": model, "error": nil, "incomplete_details": nil,
		"output": []gin.H{{
			"id": "msg_" + shortID(), "type": "message", "status": status,
			"role": "assistant", "content": []gin.H{{"type": "output_text", "text": text, "annotations": []any{}}},
		}},
		"output_text":         text,
		"parallel_tool_calls": true,
		"tool_choice":         "auto", "tools": []any{},
		"text": gin.H{"format": gin.H{"type": "text"}},
	}
}

func responsesNonStream(c *gin.Context, model string, prompt service.Prompt) {
	text, ok := runNonStream(c, "responses", model, prompt)
	if !ok {
		return
	}
	text = stripThinkingTags(text)
	out := responseObject("resp_"+shortID(), model, "completed", text, time.Now().Unix())
	out["usage"] = gin.H{"input_tokens": tokenCount(prompt.Text), "output_tokens": tokenCount(text), "total_tokens": tokenCount(prompt.Text) + tokenCount(text)}
	c.JSON(http.StatusOK, out)
}

type responseStream struct {
	*sseWriter
	seq int
}

func newResponseStream(c *gin.Context) *responseStream {
	return &responseStream{sseWriter: newSSE(c)}
}

func (s *responseStream) emit(event string, payload gin.H) {
	payload["type"] = event
	payload["sequence_number"] = s.seq
	s.seq++
	s.event(event, payload)
}

func responsesStream(c *gin.Context, model string, prompt service.Prompt) {
	stream := newResponseStream(c)
	id := "resp_" + shortID()
	created := time.Now().Unix()
	base := responseObject(id, model, "in_progress", "", created)
	base["output"] = []any{}
	stream.emit("response.created", gin.H{"response": base})
	stream.emit("response.in_progress", gin.H{"response": base})
	item := gin.H{"id": "msg_" + shortID(), "type": "message", "status": "in_progress", "role": "assistant", "content": []any{}}
	stream.emit("response.output_item.added", gin.H{"output_index": 0, "item": item})
	stream.emit("response.content_part.added", gin.H{"item_id": item["id"], "output_index": 0, "content_index": 0, "part": gin.H{"type": "output_text", "text": "", "annotations": []any{}}})

	text, err := runAndCollect("responses", model, true, prompt, func(text string) {
		text = stripThinkingTags(text)
		stream.emit("response.output_text.delta", gin.H{"item_id": item["id"], "output_index": 0, "content_index": 0, "delta": text})
	})
	text = stripThinkingTags(text)
	if err != nil {
		failed := responseObject(id, model, "failed", text, created)
		failed["error"] = gin.H{"message": err.Error(), "type": "api_error"}
		stream.emit("response.failed", gin.H{"response": failed})
		return
	}
	stream.emit("response.output_text.done", gin.H{"item_id": item["id"], "output_index": 0, "content_index": 0, "text": text})
	part := gin.H{"type": "output_text", "text": text, "annotations": []any{}}
	stream.emit("response.content_part.done", gin.H{"item_id": item["id"], "output_index": 0, "content_index": 0, "part": part})
	doneItem := gin.H{"id": item["id"], "type": "message", "status": "completed", "role": "assistant", "content": []gin.H{part}}
	stream.emit("response.output_item.done", gin.H{"output_index": 0, "item": doneItem})
	completed := responseObject(id, model, "completed", text, created)
	completed["output"] = []gin.H{doneItem}
	completed["usage"] = gin.H{"input_tokens": tokenCount(prompt.Text), "output_tokens": tokenCount(text), "total_tokens": tokenCount(prompt.Text) + tokenCount(text)}
	stream.emit("response.completed", gin.H{"response": completed})
}

// stripThinkingTags removes internal pseudo-protocol thinking blocks before
// text is exposed through the OpenAI Responses API.
func stripThinkingTags(text string) string {
	for {
		start := strings.Index(text, "<think>")
		if start < 0 {
			return strings.ReplaceAll(text, "</think>", "")
		}
		end := strings.Index(text[start+len("<think>"):], "</think>")
		if end < 0 {
			return text[:start]
		}
		end += start + len("<think>") + len("</think>")
		text = text[:start] + text[end:]
	}
}

// functionCallItem 构造 Responses API 的 function_call 输出项。
func functionCallItem(callID, name, arguments string) gin.H {
	return gin.H{
		"id":        "fc_" + shortID(),
		"type":      "function_call",
		"status":    "completed",
		"call_id":   callID,
		"name":      name,
		"arguments": arguments,
	}
}

// responsesToolNonStream 带 tools 的非流式：输出 function_call 或文本。
func responsesToolNonStream(c *gin.Context, model string, prompt service.Prompt) {
	raw, ok := runNonStream(c, "responses", model, prompt)
	if !ok {
		return
	}

	// 容错解析：上游不遵守标签协议时降级为纯文本，不报错。
	parsed := ParseTaggedOutputTolerant(raw)

	id := "resp_" + shortID()
	created := time.Now().Unix()
	if parsed.IsToolCall() {
		output := make([]gin.H, 0, len(parsed.ToolCalls))
		for _, tc := range parsed.ToolCalls {
			output = append(output, functionCallItem(newToolCallID(), tc.Name, argsJSON(tc.Arguments)))
		}
		out := gin.H{
			"id": id, "object": "response", "created_at": created, "status": "completed",
			"model": model, "error": nil, "incomplete_details": nil,
			"output": output, "output_text": "",
			"parallel_tool_calls": true, "tool_choice": "auto", "tools": []any{},
			"text":  gin.H{"format": gin.H{"type": "text"}},
			"usage": gin.H{"input_tokens": tokenCount(prompt.Text), "output_tokens": tokenCount(raw), "total_tokens": tokenCount(prompt.Text) + tokenCount(raw)},
		}
		c.JSON(http.StatusOK, out)
		return
	}

	text := formatOpenAITaggedAnswer(parsed)
	out := responseObject(id, model, "completed", text, created)
	out["usage"] = gin.H{"input_tokens": tokenCount(prompt.Text), "output_tokens": tokenCount(text), "total_tokens": tokenCount(prompt.Text) + tokenCount(text)}
	c.JSON(http.StatusOK, out)
}

// responsesToolStream 带 tools 的流式：把标签事件转成 Responses 事件。
func responsesToolStream(c *gin.Context, model string, prompt service.Prompt) {
	stream := newResponseStream(c)
	id := "resp_" + shortID()
	created := time.Now().Unix()

	base := responseObject(id, model, "in_progress", "", created)
	base["output"] = []any{}
	stream.emit("response.created", gin.H{"response": base})
	stream.emit("response.in_progress", gin.H{"response": base})

	outputIndex := 0
	var output []gin.H
	// 文本块（thinking/final_answer）用 message item 承载，仅在需要时开启。
	textItemID := "msg_" + shortID()
	textOpen := false
	openTextItem := func() {
		if textOpen {
			return
		}
		textOpen = true
		item := gin.H{"id": textItemID, "type": "message", "status": "in_progress", "role": "assistant",
			"content": []any{}}
		stream.emit("response.output_item.added", gin.H{"output_index": outputIndex, "item": item})
		stream.emit("response.content_part.added", gin.H{"item_id": textItemID, "output_index": outputIndex, "content_index": 0,
			"part": gin.H{"type": "output_text", "text": "", "annotations": []any{}}})
	}
	var textBuf strings.Builder
	closeTextItem := func() {
		if !textOpen {
			return
		}
		stream.emit("response.output_text.done", gin.H{"item_id": textItemID, "output_index": outputIndex, "content_index": 0, "text": textBuf.String()})
		part := gin.H{"type": "output_text", "text": textBuf.String(), "annotations": []any{}}
		stream.emit("response.content_part.done", gin.H{"item_id": textItemID, "output_index": outputIndex, "content_index": 0, "part": part})
		item := gin.H{"id": textItemID, "type": "message", "status": "completed", "role": "assistant",
			"content": []gin.H{part}}
		stream.emit("response.output_item.done", gin.H{"output_index": outputIndex, "item": item})
		output = append(output, item)
		outputIndex++
		textOpen = false
	}

	emitText := func(s string) {
		openTextItem()
		textBuf.WriteString(s)
		stream.emit("response.output_text.delta", gin.H{"item_id": textItemID, "output_index": outputIndex, "content_index": 0, "delta": s})
	}

	rawText, err := runTaggedStream("responses", model, prompt, func(ev TaggedStreamEvent) {
		switch ev.Type {
		case EventBlockStart:
			// Internal thinking is not exposed as Responses output_text.
		case EventBlockDelta:
			if ev.BlockType != BlockThinking && ev.Text != "" {
				emitText(ev.Text)
			}
		case EventBlockEnd:
			// Internal thinking is discarded.
		case EventToolCall:
			closeTextItem()
			callID := newToolCallID()
			args := argsJSON(ev.Arguments)
			fcItemID := "fc_" + shortID()
			item := gin.H{"id": fcItemID, "type": "function_call", "status": "in_progress",
				"call_id": callID, "name": ev.Name, "arguments": ""}
			stream.emit("response.output_item.added", gin.H{"output_index": outputIndex, "item": item})
			stream.emit("response.function_call_arguments.delta", gin.H{"item_id": fcItemID, "output_index": outputIndex, "delta": args})
			stream.emit("response.function_call_arguments.done", gin.H{"item_id": fcItemID, "output_index": outputIndex, "arguments": args})
			doneItem := gin.H{"id": fcItemID, "type": "function_call", "status": "completed",
				"call_id": callID, "name": ev.Name, "arguments": args}
			stream.emit("response.output_item.done", gin.H{"output_index": outputIndex, "item": doneItem})
			output = append(output, doneItem)
			outputIndex++
		}
	})
	closeTextItem()
	if err != nil {
		failed := responseObject(id, model, "failed", rawText, created)
		failed["error"] = gin.H{"message": err.Error(), "type": "api_error"}
		stream.emit("response.failed", gin.H{"response": failed})
		return
	}
	completed := responseObject(id, model, "completed", "", created)
	completed["output"] = output
	completed["output_text"] = textBuf.String()
	completed["usage"] = gin.H{"input_tokens": tokenCount(prompt.Text), "output_tokens": tokenCount(rawText), "total_tokens": tokenCount(prompt.Text) + tokenCount(rawText)}
	stream.emit("response.completed", gin.H{"response": completed})
}
