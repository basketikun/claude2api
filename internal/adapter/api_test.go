package adapter

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"claude2api/internal/repository"
	"claude2api/internal/service"

	"github.com/gin-gonic/gin"
)

type testRunner func(string, service.Prompt, func(string)) (service.CompletionResult, error)

func (f testRunner) Complete(model string, prompt service.Prompt, emit func(string)) (service.CompletionResult, error) {
	return f(model, prompt, emit)
}

func TestAPIContracts(t *testing.T) {
	old := runner
	oldFetch := fetchImage
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	fetchImage = func(string) ([]byte, error) { return png, nil }
	runner = testRunner(func(_ string, prompt service.Prompt, emit func(string)) (service.CompletionResult, error) {
		if len(prompt.RawRequest) == 0 {
			t.Error("应保留原始请求 JSON")
		}
		if strings.Contains(string(prompt.RawRequest), `"image`) && len(prompt.Images) == 0 {
			t.Error("应归一化图片输入")
		}
		text := "ok"
		if strings.Contains(prompt.Text, "tool_result_json") {
			text = "<final_answer>done</final_answer>"
		} else if strings.Contains(prompt.Text, "Available tools") {
			text = `<tool_calls>[{"name":"weather","arguments":{"city":"上海"}}]</tool_calls>`
		}
		emit(text)
		return service.CompletionResult{StatusCode: http.StatusOK}, nil
	})
	defer func() { runner, fetchImage = old, oldFetch }()

	tool := `"tools":[{"type":"function","function":{"name":"weather","parameters":{"type":"object"}}}]`
	cases := []struct {
		name, method, body, want string
		handler                  gin.HandlerFunc
	}{
		{"chat single", "POST", `{"messages":[{"role":"user","content":"hi"}]}`, `"chat.completion"`, OpenAIChat},
		{"chat multi image stream", "POST", `{"stream":true,"messages":[{"role":"user","content":"first"},{"role":"assistant","content":"answer"},{"role":"user","content":[{"type":"text","text":"image"},{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}}]}]}`, "data: [DONE]", OpenAIChat},
		{"chat image url", "POST", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.com/image.png"}}]}]}`, `"chat.completion"`, OpenAIChat},
		{"chat raw base64", "POST", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}]}]}`, `"chat.completion"`, OpenAIChat},
		{"responses", "POST", `{"input":"hi"}`, `"object":"response"`, OpenAIResponses},
		{"responses image stream", "POST", `{"stream":true,"input":[{"role":"user","content":[{"type":"input_text","text":"hi"},{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}]}]}`, "response.completed", OpenAIResponses},
		{"responses image url", "POST", `{"input":[{"role":"user","content":[{"type":"input_image","image_url":"https://example.com/image.png"}]}]}`, `"object":"response"`, OpenAIResponses},
		{"messages", "POST", `{"messages":[{"role":"user","content":"hi"}]}`, `"type":"message"`, AnthropicMessages},
		{"messages image stream", "POST", `{"stream":true,"messages":[{"role":"user","content":[{"type":"text","text":"hi"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}}]}]}`, "message_stop", AnthropicMessages},
		{"messages image url", "POST", `{"messages":[{"role":"user","content":[{"type":"image","source":{"type":"url","url":"https://example.com/image.png"}}]}]}`, `"type":"message"`, AnthropicMessages},
		{"chat tool", "POST", `{"messages":[{"role":"user","content":"weather"}],` + tool + `}`, "tool_calls", OpenAIChat},
		{"responses tool", "POST", `{"input":"weather",` + tool + `}`, "function_call", OpenAIResponses},
		{"messages tool", "POST", `{"messages":[{"role":"user","content":"weather"}],` + tool + `}`, "tool_use", AnthropicMessages},
		{"chat tool stream", "POST", `{"stream":true,"messages":[{"role":"user","content":"weather"}],` + tool + `}`, "tool_calls", OpenAIChat},
		{"responses tool stream", "POST", `{"stream":true,"input":"weather",` + tool + `}`, "response.function_call_arguments.done", OpenAIResponses},
		{"messages tool stream", "POST", `{"stream":true,"messages":[{"role":"user","content":"weather"}],` + tool + `}`, `"stop_reason":"tool_use"`, AnthropicMessages},
		{"chat tool result", "POST", `{"messages":[{"role":"user","content":"weather"},{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"weather","arguments":"{\"city\":\"上海\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"sunny"}],` + tool + `}`, "done", OpenAIChat},
		{"responses tool result", "POST", `{"input":[{"role":"user","content":"weather"},{"type":"function_call","call_id":"call_1","name":"weather","arguments":"{\"city\":\"上海\"}"},{"type":"function_call_output","call_id":"call_1","output":"sunny"}],` + tool + `}`, "done", OpenAIResponses},
		{"messages tool result", "POST", `{"messages":[{"role":"user","content":"weather"},{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"weather","input":{"city":"上海"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"sunny"}]}],` + tool + `}`, "done", AnthropicMessages},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tc.method, "/v1", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			tc.handler(c)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAPIUsageUsesCapturedCounts(t *testing.T) {
	old := runner
	runner = testRunner(func(_ string, _ service.Prompt, emit func(string)) (service.CompletionResult, error) {
		emit("<think>visible reasoning</think>answer")
		return service.CompletionResult{StatusCode: http.StatusOK, Usage: &repository.UsageRecord{Success: true, TextUsage: repository.TextUsage{InputTokens: 321, OutputTokens: 5, ThinkingTokens: 7}}}, nil
	})
	t.Cleanup(func() { runner = old })
	for _, tc := range []struct {
		name, body, inputKey, outputKey string
		handler                         gin.HandlerFunc
	}{
		{"chat", `{"messages":[{"role":"user","content":"hi"}]}`, "prompt_tokens", "completion_tokens", OpenAIChat},
		{"chat stream", `{"stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`, "prompt_tokens", "completion_tokens", OpenAIChat},
		{"responses", `{"input":"hi"}`, "input_tokens", "output_tokens", OpenAIResponses},
		{"responses stream", `{"stream":true,"input":"hi"}`, "input_tokens", "output_tokens", OpenAIResponses},
		{"messages", `{"messages":[{"role":"user","content":"hi"}]}`, "input_tokens", "output_tokens", AnthropicMessages},
		{"messages stream", `{"stream":true,"messages":[{"role":"user","content":"hi"}]}`, "input_tokens", "output_tokens", AnthropicMessages},
	} {
		for _, withTools := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/tools=%v", tc.name, withTools), func(t *testing.T) {
				body := tc.body
				if withTools {
					body = body[:len(body)-1] + `,"tools":[{"type":"function","function":{"name":"weather","parameters":{"type":"object"}}}]}`
				}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1", strings.NewReader(body))
				tc.handler(c)
				payloads := []string{w.Body.String()}
				if strings.Contains(body, `"stream":true`) {
					payloads = nil
					for _, line := range strings.Split(w.Body.String(), "\n") {
						if data, ok := strings.CutPrefix(line, "data: "); ok && data != "[DONE]" {
							payloads = append(payloads, data)
						}
					}
				}
				var usage map[string]int
				for _, payload := range payloads {
					var event struct {
						Usage    map[string]int `json:"usage"`
						Response struct {
							Usage map[string]int `json:"usage"`
						} `json:"response"`
					}
					if err := json.Unmarshal([]byte(payload), &event); err != nil {
						t.Fatal(err)
					}
					if event.Usage != nil {
						usage = event.Usage
					}
					if event.Response.Usage != nil {
						usage = event.Response.Usage
					}
				}
				if w.Code != http.StatusOK || usage[tc.inputKey] != 321 || usage[tc.outputKey] != 12 {
					t.Fatalf("未复用采集用量：%d %s", w.Code, w.Body.String())
				}
				if total, ok := usage["total_tokens"]; ok && total != 333 {
					t.Fatalf("思考计数重复或遗漏：%v", usage)
				}
			})
		}
	}
}

func TestUpstreamStatus(t *testing.T) {
	old := runner
	runner = testRunner(func(string, service.Prompt, func(string)) (service.CompletionResult, error) {
		err := errors.New("rate limited")
		return service.CompletionResult{StatusCode: http.StatusTooManyRequests}, &service.CompletionError{StatusCode: http.StatusTooManyRequests, Err: err}
	})
	defer func() { runner = old }()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	OpenAIChat(c)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
