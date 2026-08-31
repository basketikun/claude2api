package adapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"claude2api/internal/service"
)

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

func buildToolPrompt(messages []Message, tools []map[string]any, parallel bool) service.Prompt {
	parts := []string{FormatTaggedPrompt(tools, parallel)}
	for _, message := range messages {
		switch message.Role {
		case "system":
			if message.Content != "" {
				parts = append(parts, "System："+message.Content)
			}
		case "user":
			if message.Content != "" {
				parts = append(parts, "**User**: "+message.Content)
			}
		case "assistant":
			parts = appendAssistant(parts, message, parallel)
		case "tool":
			result, _ := json.Marshal(message.Content)
			parts = append(parts, fmt.Sprintf("Verbatim JSON-string result from tool call_id=%s:\n<tool_result_json>%s</tool_result_json>", message.ToolCallID, result))
		}
	}
	return service.Prompt{Text: strings.Join(parts, "\n\n")}
}

func toolChoiceInstruction(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var choice any
	if json.Unmarshal(raw, &choice) != nil {
		return ""
	}
	if value, ok := choice.(string); ok {
		switch value {
		case "none":
			return "\n\nTool choice: do not call tools; return <final_answer>."
		case "required", "any":
			return "\n\nTool choice: you must call at least one tool."
		}
	}
	if value, ok := choice.(map[string]any); ok {
		switch value["type"] {
		case "none":
			return "\n\nTool choice: do not call tools; return <final_answer>."
		case "required", "any":
			return "\n\nTool choice: you must call at least one tool."
		}
		name, _ := value["name"].(string)
		if fn, ok := value["function"].(map[string]any); ok {
			name, _ = fn["name"].(string)
		}
		if name != "" {
			return "\n\nTool choice: you must call " + name + "."
		}
	}
	return ""
}

func appendAssistant(parts []string, message Message, parallel bool) []string {
	if len(message.ToolCalls) == 0 {
		if message.Content != "" {
			return append(parts, "**Assistant**:\n\n"+message.Content)
		}
		return parts
	}
	if message.Content != "" {
		parts = append(parts, "**Assistant**:\n\n"+message.Content)
	}
	ids := make([]string, 0, len(message.ToolCalls))
	payloads := make([]map[string]any, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		if call.ID != "" {
			ids = append(ids, call.ID)
		}
		payloads = append(payloads, map[string]any{"name": call.Name, "arguments": decodeArgs(call.Arguments)})
	}
	label := "**Assistant**:"
	if len(ids) == 1 {
		label = fmt.Sprintf("**Assistant(Call ID: %s)**:", ids[0])
	} else if len(ids) > 1 {
		label = "**Assistant(Call IDs: " + strings.Join(ids, ", ") + ")**:"
	}
	payload, tag := mustJSON(payloads), "tool_calls"
	if len(payloads) == 1 && !parallel {
		payload, tag = mustJSON(payloads[0]), "tool_call"
	}
	return append(parts, label+"\n\n<"+tag+">"+string(payload)+"</"+tag+">")
}

func decodeArgs(arguments string) any {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return map[string]any{}
	}
	var value any
	if json.Unmarshal([]byte(arguments), &value) != nil {
		return map[string]any{"raw": arguments}
	}
	return value
}

func mustJSON(value any) []byte { data, _ := json.Marshal(value); return data }

func formatOpenAITaggedAnswer(parsed TaggedOutput) string {
	parts := []string{}
	if parsed.Thinking != "" {
		parts = append(parts, "<think>"+parsed.Thinking+"</think>")
	}
	if parsed.FinalAnswer != "" {
		parts = append(parts, parsed.FinalAnswer)
	}
	return strings.Join(parts, "\n\n")
}

// parseTools 把 json 原文解析为 []map[string]any。
func parseTools(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var tools []map[string]any
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	return tools
}

// allowParallel 从 parallel_tool_calls 判断是否允许并行工具调用，默认 true。
func allowParallel(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return true
	}
	return b
}

func parseStops(raw json.RawMessage) []string {
	var stops []string
	if json.Unmarshal(raw, &stops) == nil {
		return stops
	}
	var stop string
	_ = json.Unmarshal(raw, &stop)
	if stop != "" {
		return []string{stop}
	}
	return nil
}

// newToolCallID 生成 OpenAI 风格工具调用 id。
func newToolCallID() string {
	return "call_" + shortID()
}

// newAnthropicToolID 生成 Anthropic 风格工具调用 id。
func newAnthropicToolID() string {
	return "toolu_" + shortID()
}

// argsJSON 把工具参数序列化为紧凑 JSON 字符串。
func argsJSON(args map[string]any) string {
	if args == nil {
		return "{}"
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// runTaggedStream 消费 Runner 的文本回调，逐分片喂给标签流式解析器，
// 把中间事件交给 onEvent 处理。返回 completion 结果与错误。
func runTaggedStream(endpoint, model string, prompt service.Prompt, onEvent func(TaggedStreamEvent)) (string, error) {
	parser := NewTaggedStreamParser()
	start := time.Now()
	firstTokenMs := int64(0)
	firstSeen := false
	var output strings.Builder
	res, err := runner.Complete(model, prompt, func(t string) {
		if t != "" && !firstSeen {
			firstSeen = true
			firstTokenMs = time.Since(start).Milliseconds()
		}
		output.WriteString(t)
		events, perr := parser.Feed(t)
		if perr != nil {
			return
		}
		for _, ev := range events {
			onEvent(ev)
		}
	})
	raw := output.String()
	if err == nil {
		var events []TaggedStreamEvent
		events, err = parser.Finish()
		for _, ev := range events {
			onEvent(ev)
		}
	}
	logCompletion(endpoint, model, true, prompt, raw, res, err, start, firstTokenMs)
	return raw, err
}
