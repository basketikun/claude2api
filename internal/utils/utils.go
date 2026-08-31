package utils

// Truncate 截断到 n 字节。
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
