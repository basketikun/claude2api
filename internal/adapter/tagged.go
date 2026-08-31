package adapter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 标签工具协议（tagged tool protocol）：claude.ai 网页端只能输出纯文本，
// 因此让模型只输出一套强约束标签 DSL，服务端再解析为协议无关的中间语义，
// 最后由各协议 handler 输出成 OpenAI / Anthropic 的 tool_calls / tool_use。
//
// 模型输出契约：
//
//	<think>...</think>
//	<tool_calls>[{"name":"Read","arguments":{"path":"..."}}]</tool_calls>
//
// 或：
//
//	<think>...</think>
//	<final_answer>给用户的最终回复</final_answer>

const taggedToolPromptParallel = `Write the AI assistant's next response using only the following XML-like tags:

- <think>...</think>
- <tool_calls>[{"name":"ToolName","arguments":{...}}]</tool_calls>
- <final_answer>...</final_answer>

Rules:
- You may output one or more <think> blocks.
- You must then output exactly one terminal block: either <tool_calls> or <final_answer>.
- Do not output any text outside these tags.
- In <tool_calls>, the content must be a valid JSON array. Each item must be an object with keys "name" and "arguments".
- If you need only one tool, still use <tool_calls> with an array of length 1.
- In string values inside <tool_calls>, you must escape quotes, backslashes, and newlines exactly as JSON requires.
- After </tool_calls> or </final_answer>, stop immediately.
- Never generate Observation, tool results, or a second terminal block in the same response.
- Never output <observation>; the system will provide tool results in the next turn.
`

const taggedToolPromptSingle = `Write the AI assistant's next response using only the following XML-like tags:

- <think>...</think>
- <tool_call>{"name":"ToolName","arguments":{...}}</tool_call>
- <final_answer>...</final_answer>

Rules:
- You may output one or more <think> blocks.
- You must then output exactly one terminal block: either <tool_call> or <final_answer>.
- Do not output any text outside these tags.
- In <tool_call>, the content must be a valid JSON object with keys "name" and "arguments".
- In string values inside <tool_call>, you must escape quotes, backslashes, and newlines exactly as JSON requires.
- After </tool_call> or </final_answer>, stop immediately.
- Never generate Observation, tool results, or a second terminal block in the same response.
- Never output <observation>; the system will provide tool results in the next turn.
`

// TaggedToolCall 是解析出的单次工具调用。
type TaggedToolCall struct {
	Name      string
	Arguments map[string]any
}

// TaggedOutput 是模型一轮响应的解析结果。
type TaggedOutput struct {
	Thinking    string
	ToolCalls   []TaggedToolCall
	FinalAnswer string
	HasFinal    bool
}

// IsToolCall 表示本轮是工具调用。
func (o TaggedOutput) IsToolCall() bool { return len(o.ToolCalls) > 0 }

// IsFinalAnswer 表示本轮是最终回答。
func (o TaggedOutput) IsFinalAnswer() bool { return o.HasFinal }

// FormatToolsForPrompt 把 OpenAI / Anthropic 风格的 tools 转成可读文本列表。
func FormatToolsForPrompt(tools []map[string]any) string {
	if len(tools) == 0 {
		return ""
	}
	lines := make([]string, 0, len(tools))
	for _, tool := range tools {
		fn := tool
		if t, _ := tool["type"].(string); t == "function" {
			if f, ok := tool["function"].(map[string]any); ok {
				fn = f
			}
		}
		name, _ := fn["name"].(string)
		if name == "" {
			continue
		}
		description, _ := fn["description"].(string)
		if description == "" {
			if s, ok := fn["summary"].(string); ok {
				description = s
			}
		}
		params := mapField(fn, "parameters")
		if params == nil {
			params = mapField(fn, "input_schema")
		}
		var argsDesc string
		if params != nil {
			props := mapField(params, "properties")
			required := map[string]bool{}
			if reqList, ok := params["required"].([]any); ok {
				for _, r := range reqList {
					if s, ok := r.(string); ok {
						required[s] = true
					}
				}
			}
			if props != nil {
				argParts := make([]string, 0, len(props))
				for key, v := range props {
					vm, ok := v.(map[string]any)
					if !ok {
						continue
					}
					typ, _ := vm["type"].(string)
					if typ == "" {
						typ = "any"
					}
					part := fmt.Sprintf("%s: %s", key, typ)
					if required[key] {
						part += " (required)"
					}
					argParts = append(argParts, part)
				}
				argsDesc = strings.Join(argParts, ", ")
			}
		}
		suffix := ""
		if len(description) > 200 {
			description = description[:200]
			suffix = "..."
		}
		lines = append(lines, fmt.Sprintf("- %s(%s): %s%s", name, argsDesc, description, suffix))
	}
	return strings.Join(lines, "\n")
}

// mapField 取一个可能是 map 的字段。
func mapField(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

// FormatTaggedPrompt 构造标签协议的系统前缀。
func FormatTaggedPrompt(tools []map[string]any, allowParallel bool) string {
	toolsText := FormatToolsForPrompt(tools)
	base := taggedToolPromptParallel
	if !allowParallel {
		base = taggedToolPromptSingle
	}
	if toolsText != "" {
		return base + "\n\n---\n\n## Available tools\n\n" + toolsText + "\n"
	}
	return base
}

// parseToolCallItem 校验并转换单个工具调用。
func parseToolCallItem(payload any) (TaggedToolCall, error) {
	obj, ok := payload.(map[string]any)
	if !ok {
		return TaggedToolCall{}, fmt.Errorf("tool call payload must be an object")
	}
	name, _ := obj["name"].(string)
	if strings.TrimSpace(name) == "" {
		return TaggedToolCall{}, fmt.Errorf("tool_call.name must be a non-empty string")
	}
	args, ok := obj["arguments"].(map[string]any)
	if !ok {
		return TaggedToolCall{}, fmt.Errorf("tool_call.arguments must be an object")
	}
	return TaggedToolCall{Name: strings.TrimSpace(name), Arguments: args}, nil
}

// parseToolCallBlock 解析单对象形式 <tool_call>{...}</tool_call>。
func parseToolCallBlock(rawJSON string) ([]TaggedToolCall, error) {
	var payload any
	if err := json.Unmarshal([]byte(rawJSON), &payload); err != nil {
		return nil, fmt.Errorf("invalid tool_call json: %w", err)
	}
	tc, err := parseToolCallItem(payload)
	if err != nil {
		return nil, err
	}
	return []TaggedToolCall{tc}, nil
}

// parseToolCallsBlock 解析数组形式 <tool_calls>[...]</tool_calls>。
func parseToolCallsBlock(rawJSON string) ([]TaggedToolCall, error) {
	var payload any
	if err := json.Unmarshal([]byte(rawJSON), &payload); err != nil {
		return nil, fmt.Errorf("invalid tool_calls json: %w", err)
	}
	arr, ok := payload.([]any)
	if !ok {
		return nil, fmt.Errorf("tool_calls payload must be an array")
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("tool_calls payload must not be empty")
	}
	out := make([]TaggedToolCall, 0, len(arr))
	for _, item := range arr {
		tc, err := parseToolCallItem(item)
		if err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, nil
}

// ParseTaggedOutput 严格解析上游模型输出的标签协议文本（非流式）。
func ParseTaggedOutput(text string) (TaggedOutput, error) {
	content := strings.TrimSpace(text)
	if content == "" {
		return TaggedOutput{}, fmt.Errorf("empty tagged output")
	}
	n := len(content)

	skipWS := func(pos int) int {
		for pos < n && isSpace(content[pos]) {
			pos++
		}
		return pos
	}
	readBlock := func(pos int, tag string) (string, int, error) {
		open := "<" + tag + ">"
		closeTag := "</" + tag + ">"
		if !strings.HasPrefix(content[pos:], open) {
			return "", pos, fmt.Errorf("expected %s", open)
		}
		start := pos + len(open)
		end := strings.Index(content[start:], closeTag)
		if end < 0 {
			return "", pos, fmt.Errorf("missing %s", closeTag)
		}
		return content[start : start+end], start + end + len(closeTag), nil
	}

	pos := skipWS(0)
	var thinkingBlocks []string
	var toolCalls []TaggedToolCall
	var finalAnswer string
	hasFinal := false

	for pos < n {
		switch {
		case strings.HasPrefix(content[pos:], "<think>"):
			raw, next, err := readBlock(pos, "think")
			if err != nil {
				return TaggedOutput{}, err
			}
			if t := strings.TrimSpace(raw); t != "" {
				thinkingBlocks = append(thinkingBlocks, t)
			}
			pos = skipWS(next)
		case strings.HasPrefix(content[pos:], "<tool_calls>"):
			raw, _, err := readBlock(pos, "tool_calls")
			if err != nil {
				return TaggedOutput{}, err
			}
			toolCalls, err = parseToolCallsBlock(strings.TrimSpace(raw))
			if err != nil {
				return TaggedOutput{}, err
			}
			pos = n
		case strings.HasPrefix(content[pos:], "<tool_call>"):
			raw, _, err := readBlock(pos, "tool_call")
			if err != nil {
				return TaggedOutput{}, err
			}
			toolCalls, err = parseToolCallBlock(strings.TrimSpace(raw))
			if err != nil {
				return TaggedOutput{}, err
			}
			pos = n
		case strings.HasPrefix(content[pos:], "<final_answer>"):
			raw, _, err := readBlock(pos, "final_answer")
			if err != nil {
				return TaggedOutput{}, err
			}
			finalAnswer = strings.TrimSpace(raw)
			hasFinal = true
			pos = n
		case isSpace(content[pos]):
			pos++
		default:
			return TaggedOutput{}, fmt.Errorf("text outside tags is not allowed")
		}
	}

	if len(toolCalls) == 0 && !hasFinal {
		return TaggedOutput{}, fmt.Errorf("expected <tool_calls>, <tool_call>, or <final_answer>")
	}
	return TaggedOutput{
		Thinking:    strings.Join(thinkingBlocks, "\n\n"),
		ToolCalls:   toolCalls,
		FinalAnswer: finalAnswer,
		HasFinal:    hasFinal,
	}, nil
}

// ParseTaggedOutputTolerant 是 ParseTaggedOutput 的容错版本：当上游模型不遵守
// 标签协议（纯文本、缺失闭合标签、标签外夹带文字等）导致严格解析失败时，
// 直接把整段原始输出降级成一次普通的最终回答，而不是报错中断请求。
func ParseTaggedOutputTolerant(text string) TaggedOutput {
	if parsed, err := ParseTaggedOutput(text); err == nil {
		return parsed
	}
	return TaggedOutput{
		FinalAnswer: strings.TrimSpace(text),
		HasFinal:    true,
	}
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

type TaggedBlockType string

const (
	BlockThinking TaggedBlockType = "thinking"
	BlockText     TaggedBlockType = "text"
)

type TaggedEventType string

const (
	EventMessageStart TaggedEventType = "message_start"
	EventBlockStart   TaggedEventType = "block_start"
	EventBlockDelta   TaggedEventType = "block_delta"
	EventBlockEnd     TaggedEventType = "block_end"
	EventToolCall     TaggedEventType = "tool_call"
)

type TaggedStreamEvent struct {
	Type      TaggedEventType
	BlockType TaggedBlockType
	Text      string
	Name      string
	Arguments map[string]any
}

var taggedKnownTags = []string{
	"<think>", "</think>",
	"<tool_call>", "</tool_call>",
	"<tool_calls>", "</tool_calls>",
	"<final_answer>", "</final_answer>",
}

func isKnownTag(s string) bool {
	for _, tag := range taggedKnownTags {
		if s == tag {
			return true
		}
	}
	return false
}

func isTagPrefix(s string) bool {
	for _, tag := range taggedKnownTags {
		if strings.HasPrefix(tag, s) {
			return true
		}
	}
	return false
}

type TaggedStreamParser struct {
	messageStarted bool
	terminalClosed bool
	pendingTag     bool
	tagBuf         strings.Builder
	textMode       TaggedBlockType
	openBlock      TaggedBlockType
	textBuf        strings.Builder
	inFinal        bool
	inToolJSON     bool
	toolTag        string
	toolBuf        strings.Builder
}

func NewTaggedStreamParser() *TaggedStreamParser {
	return &TaggedStreamParser{textMode: BlockText}
}

func (p *TaggedStreamParser) Feed(chunk string) ([]TaggedStreamEvent, error) {
	var events []TaggedStreamEvent
	for _, char := range chunk {
		p.onChar(&events, char)
	}
	p.flushText(&events)
	return events, nil
}

func (p *TaggedStreamParser) Finish() ([]TaggedStreamEvent, error) {
	var events []TaggedStreamEvent
	if !p.terminalClosed {
		if p.pendingTag {
			p.emitRaw(&events, p.tagBuf.String())
			p.pendingTag = false
			p.tagBuf.Reset()
		}
		if p.inToolJSON {
			p.appendText(&events, "<"+p.toolTag+">"+p.toolBuf.String())
			p.inToolJSON = false
		}
	}
	p.flushText(&events)
	p.closeBlock(&events)
	p.ensureStarted(&events)
	return events, nil
}

func (p *TaggedStreamParser) onChar(events *[]TaggedStreamEvent, char rune) {
	if p.terminalClosed {
		return
	}
	if p.pendingTag {
		p.tagBuf.WriteRune(char)
		value := p.tagBuf.String()
		if isKnownTag(value) {
			p.pendingTag = false
			p.tagBuf.Reset()
			p.handleTag(events, value)
			return
		}
		if isTagPrefix(value) {
			return
		}
		if char == '<' {
			p.emitRaw(events, value[:len(value)-1])
			p.tagBuf.Reset()
			p.tagBuf.WriteByte('<')
			return
		}
		p.emitRaw(events, value)
		p.pendingTag = false
		p.tagBuf.Reset()
		return
	}
	if char == '<' {
		p.pendingTag = true
		p.tagBuf.Reset()
		p.tagBuf.WriteByte('<')
		return
	}
	p.emitRaw(events, string(char))
}

func (p *TaggedStreamParser) emitRaw(events *[]TaggedStreamEvent, text string) {
	if text == "" {
		return
	}
	if p.inToolJSON {
		p.toolBuf.WriteString(text)
		return
	}
	p.appendText(events, text)
}

func (p *TaggedStreamParser) handleTag(events *[]TaggedStreamEvent, tag string) {
	if p.inFinal && tag != "</final_answer>" {
		p.appendText(events, tag)
		return
	}
	if p.inToolJSON && tag != "</"+p.toolTag+">" {
		p.toolBuf.WriteString(tag)
		return
	}
	if p.textMode == BlockThinking && tag != "</think>" {
		p.appendText(events, tag)
		return
	}
	switch tag {
	case "<think>":
		p.closeBlock(events)
		p.textMode = BlockThinking
	case "</think>":
		p.closeBlock(events)
		p.textMode = BlockText
	case "<final_answer>":
		p.closeBlock(events)
		p.textMode = BlockText
		p.inFinal = true
	case "</final_answer>":
		if p.inFinal {
			p.closeBlock(events)
			p.inFinal = false
			p.terminalClosed = true
		} else {
			p.appendText(events, tag)
		}
	case "<tool_call>", "<tool_calls>":
		p.closeBlock(events)
		p.inToolJSON = true
		p.toolTag = strings.Trim(tag, "<>")
		p.toolBuf.Reset()
	case "</tool_call>":
		if p.inToolJSON {
			p.finishToolJSON(events, true)
		} else {
			p.appendText(events, tag)
		}
	case "</tool_calls>":
		if p.inToolJSON {
			p.finishToolJSON(events, false)
		} else {
			p.appendText(events, tag)
		}
	}
}

func (p *TaggedStreamParser) finishToolJSON(events *[]TaggedStreamEvent, single bool) {
	raw := strings.TrimSpace(p.toolBuf.String())
	openTag := "<" + p.toolTag + ">"
	closeTag := "</" + p.toolTag + ">"
	p.inToolJSON = false
	p.toolBuf.Reset()
	var calls []TaggedToolCall
	var err error
	if single {
		calls, err = parseToolCallBlock(raw)
	} else {
		calls, err = parseToolCallsBlock(raw)
	}
	if err != nil {
		p.appendText(events, openTag+raw+closeTag)
		return
	}
	p.ensureStarted(events)
	p.terminalClosed = true
	for _, call := range calls {
		*events = append(*events, TaggedStreamEvent{Type: EventToolCall, Name: call.Name, Arguments: call.Arguments})
	}
}

func (p *TaggedStreamParser) ensureStarted(events *[]TaggedStreamEvent) {
	if p.messageStarted {
		return
	}
	p.messageStarted = true
	*events = append(*events, TaggedStreamEvent{Type: EventMessageStart})
}

func (p *TaggedStreamParser) appendText(events *[]TaggedStreamEvent, text string) {
	p.ensureStarted(events)
	if p.openBlock != p.textMode {
		p.flushText(events)
		p.closeBlock(events)
		*events = append(*events, TaggedStreamEvent{Type: EventBlockStart, BlockType: p.textMode})
		p.openBlock = p.textMode
	}
	p.textBuf.WriteString(text)
}

func (p *TaggedStreamParser) flushText(events *[]TaggedStreamEvent) {
	if p.textBuf.Len() == 0 || p.openBlock == "" {
		return
	}
	*events = append(*events, TaggedStreamEvent{Type: EventBlockDelta, BlockType: p.openBlock, Text: p.textBuf.String()})
	p.textBuf.Reset()
}

func (p *TaggedStreamParser) closeBlock(events *[]TaggedStreamEvent) {
	if p.openBlock == "" {
		return
	}
	p.flushText(events)
	*events = append(*events, TaggedStreamEvent{Type: EventBlockEnd, BlockType: p.openBlock})
	p.openBlock = ""
}
