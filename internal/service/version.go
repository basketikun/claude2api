package service

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"claude2api/internal/utils"

	"golang.org/x/mod/semver"
)

var versionClient = &http.Client{Timeout: 5 * time.Second}
var versionCache struct {
	sync.Mutex
	checkedAt time.Time
	latest    string
	err       error
}

// LatestVersion 缓存 GitHub 主分支的版本，失败时保留真实错误，不判断为最新。
func LatestVersion() (string, error) {
	versionCache.Lock()
	defer versionCache.Unlock()
	if time.Since(versionCache.checkedAt) < time.Hour {
		return versionCache.latest, versionCache.err
	}
	versionCache.latest, versionCache.err = fetchLatestVersion()
	versionCache.checkedAt = time.Now()
	if versionCache.err != nil {
		slog.Warn("[版本检查] 读取 GitHub VERSION 失败", "err", utils.RedactMedia(versionCache.err.Error()))
	}
	return versionCache.latest, versionCache.err
}

func fetchLatestVersion() (string, error) {
	response, err := versionClient.Get("https://raw.githubusercontent.com/basketikun/claude2api/main/VERSION")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub VERSION HTTP %d: %s", response.StatusCode, body)
	}
	var latest string
	if _, err := fmt.Sscan(string(body), &latest); err != nil {
		return "", fmt.Errorf("GitHub VERSION 解析失败: %w，response=%s", err, body)
	}
	if !semver.IsValid(latest) {
		return "", fmt.Errorf("GitHub VERSION 不是语义化版本: %s", body)
	}
	return latest, nil
}
