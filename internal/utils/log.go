package utils

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var mediaDataURI = regexp.MustCompile(`data:[^;,\s]+;base64,[A-Za-z0-9+/=_-]+`)

func RequestLog(raw json.RawMessage) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(RedactMedia(value))
}

func RedactMedia(value any) any {
	switch item := value.(type) {
	case map[string]any:
		if item["type"] == "base64" {
			if data, ok := item["data"].(string); ok {
				item["data"] = fmt.Sprintf("[媒体内容未保存，Base64 字符数 %d]", len(data))
			}
		}
		for key, child := range item {
			if key == "image_url" {
				if image, ok := child.(map[string]any); ok {
					if data, ok := image["url"].(string); ok && !strings.HasPrefix(data, "data:") && !strings.HasPrefix(data, "https://") && !strings.HasPrefix(data, "http://") {
						image["url"] = fmt.Sprintf("[图片内容未保存，字符数 %d]", len(data))
					}
				}
				if data, ok := child.(string); ok && !strings.HasPrefix(data, "data:") && !strings.HasPrefix(data, "https://") && !strings.HasPrefix(data, "http://") {
					item[key] = fmt.Sprintf("[图片内容未保存，字符数 %d]", len(data))
					continue
				}
			}
			item[key] = RedactMedia(child)
		}
	case []any:
		for i, child := range item {
			item[i] = RedactMedia(child)
		}
	case string:
		return mediaDataURI.ReplaceAllStringFunc(item, func(media string) string {
			metadata, payload, _ := strings.Cut(media, ";base64,")
			return fmt.Sprintf("[媒体内容未保存，类型 %s，Base64 字符数 %d]", strings.TrimPrefix(metadata, "data:"), len(payload))
		})
	}
	return value
}
