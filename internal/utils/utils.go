package utils

// EstimateTokens 按 UTF-8 字节数 / 4 粗估可见文本 Token，不是模型分词结果。
func EstimateTokens(size int) int {
	return size / 4
}

// Truncate 截断到 n 字节。
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
