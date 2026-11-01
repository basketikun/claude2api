package utils

import "github.com/tiktoken-go/tokenizer/codec"

var textTokenizer = codec.NewCl100kBase()

// EstimateTokens 使用 cl100k_base 参考分词估算文本，不代表 Claude 分词或计费用量。
func EstimateTokens(text string) int {
	tokens, err := textTokenizer.Count(text)
	if err != nil {
		panic(err)
	}
	return tokens
}

// Truncate 截断到 n 字节。
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
