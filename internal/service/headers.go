package service

import "github.com/google/uuid"

const (
	claudeAccept         = "*/*"
	claudeAcceptLanguage = "en-US,en;q=0.9"
	claudeClientPlatform = "web_claude_ai"
	claudeClientVersion  = "1.0.0"
	claudeClientSHA      = "882d9a7d43eced6a100e636e1dfdebc55764bd78"

	fallbackAnonID   = "claudeai.v1.551770c2-6f7e-499b-a039-9cbb0998b0a9"
	fallbackDeviceID = "8370843e-1ef9-4a50-badf-7894e7957a29"
)

var (
	anonNS   = uuid.MustParse("6f4a1c2e-1b3d-4e5f-8a90-0c1d2e3f4a5b")
	deviceNS = uuid.MustParse("9d8c7b6a-5e4f-4321-9a8b-7c6d5e4f3a2b")
)

// BuildHeaders 返回 claude.ai 请求头。
func BuildHeaders(seed string) map[string]string {
	anonID := fallbackAnonID
	deviceID := fallbackDeviceID
	if seed != "" {
		anonID = "claudeai.v1." + uuid.NewSHA1(anonNS, []byte(seed)).String()
		deviceID = uuid.NewSHA1(deviceNS, []byte(seed)).String()
	}
	return map[string]string{
		"accept":                    claudeAccept,
		"content-type":              "application/json",
		"origin":                    claudeAIBaseURL,
		"referer":                   claudeAIBaseURL + "/",
		"user-agent":                claudeAIUserAgent,
		"accept-language":           claudeAcceptLanguage,
		"anthropic-client-platform": claudeClientPlatform,
		"anthropic-client-version":  claudeClientVersion,
		"anthropic-client-sha":      claudeClientSHA,
		"anthropic-anonymous-id":    anonID,
		"anthropic-device-id":       deviceID,
	}
}
