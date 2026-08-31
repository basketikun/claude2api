package adapter

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"
)

func pretty(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

const (
	sdkTestBaseURL = "http://127.0.0.1:8787"
	sdkTestAPIKey  = "claude2api"
	sdkTestModel   = "claude-sonnet-5"
)

func sdkTestConfig(t *testing.T) (context.Context, string, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx, sdkTestBaseURL, sdkTestAPIKey, sdkTestModel
}

func requireToolCalls(t *testing.T, got []string, raw string) {
	t.Helper()
	want := map[string]bool{"get_weather": true, "get_time": true, "calculate": true}
	for _, name := range got {
		delete(want, name)
	}
	if len(want) > 0 {
		t.Fatalf("缺少工具调用 %v：%s", want, raw)
	}
}

func TestOpenAISDKToolCall(t *testing.T) {
	ctx, base, key, model := sdkTestConfig(t)
	client := openai.NewClient(openaioption.WithBaseURL(base+"/v1"), openaioption.WithAPIKey(key))
	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("请同时调用工具查询上海天气、上海当前时间，并计算 12*8；三个工具都必须调用，不要直接回答。")},
		Tools: []openai.ChatCompletionToolUnionParam{openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:       "get_weather",
			Parameters: openai.FunctionParameters{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}, "required": []string{"city"}},
		}), openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:       "get_time",
			Parameters: openai.FunctionParameters{"type": "object", "properties": map[string]any{"timezone": map[string]any{"type": "string"}}, "required": []string{"timezone"}},
		}), openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:       "calculate",
			Parameters: openai.FunctionParameters{"type": "object", "properties": map[string]any{"expression": map[string]any{"type": "string"}}, "required": []string{"expression"}},
		})},
	}
	t.Logf("OpenAI 输入：\n%s", pretty(params))
	res, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("OpenAI 输出：\n%s", res.RawJSON())
	if len(res.Choices) == 0 {
		t.Fatalf("未返回结果：%s", res.RawJSON())
	}
	names := make([]string, 0, len(res.Choices[0].Message.ToolCalls))
	for _, call := range res.Choices[0].Message.ToolCalls {
		names = append(names, call.Function.Name)
	}
	requireToolCalls(t, names, res.RawJSON())
}

func TestAnthropicSDKToolCall(t *testing.T) {
	ctx, base, key, model := sdkTestConfig(t)
	client := anthropic.NewClient(anthropicoption.WithBaseURL(base), anthropicoption.WithAPIKey(key))
	params := anthropic.MessageNewParams{
		Model: anthropic.Model(model), MaxTokens: 1024,
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("请同时调用工具查询上海天气、上海当前时间，并计算 12*8；三个工具都必须调用，不要直接回答。"))},
		Tools: []anthropic.ToolUnionParam{anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{
			Properties: map[string]any{"city": map[string]any{"type": "string"}}, Required: []string{"city"},
		}, "get_weather"), anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{
			Properties: map[string]any{"timezone": map[string]any{"type": "string"}}, Required: []string{"timezone"},
		}, "get_time"), anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{
			Properties: map[string]any{"expression": map[string]any{"type": "string"}}, Required: []string{"expression"},
		}, "calculate")},
	}
	t.Logf("Anthropic 输入：\n%s", pretty(params))
	res, err := client.Messages.New(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Anthropic 输出：\n%s", res.RawJSON())
	var names []string
	for _, block := range res.Content {
		if block.Type == "tool_use" {
			names = append(names, block.Name)
		}
	}
	requireToolCalls(t, names, res.RawJSON())
}
