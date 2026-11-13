package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"claude2api/internal/config"
	"claude2api/internal/middleware"
)

// TelegramInfo 供前端渲染登录按钮（未配置时 enabled=false，页面不显示）。
func TelegramInfo(c *gin.Context) {
	cfg := config.Get()
	c.JSON(http.StatusOK, gin.H{
		"enabled":  strings.TrimSpace(cfg.TelegramBotToken) != "" && strings.TrimSpace(cfg.TelegramBotName) != "",
		"bot_name": strings.TrimSpace(cfg.TelegramBotName),
	})
}

// TelegramLogin 处理 Telegram Login Widget 的回调。
//
// 校验规则（官方文档）：
//   - 用 SHA256(bot_token) 作为 key 对去掉 hash 的字段做 HMAC-SHA256，与 hash 比对
//   - auth_date 不能过旧（默认 24h）
//   - 若配置了 telegram_allowed_ids，只允许名单内的 Telegram user id
func TelegramLogin(c *gin.Context) {
	cfg := config.Get()
	token := strings.TrimSpace(cfg.TelegramBotToken)
	if token == "" {
		c.String(http.StatusServiceUnavailable, "Telegram 登录未启用")
		return
	}
	// Widget 可能用 GET(query) 也可能用 POST(form)，两种都收。
	values := url.Values{}
	for k, v := range c.Request.URL.Query() {
		values[k] = v
	}
	if c.Request.Method == http.MethodPost {
		_ = c.Request.ParseForm()
		for k, v := range c.Request.PostForm {
			values[k] = v
		}
	}
	hash := values.Get("hash")
	if hash == "" {
		c.String(http.StatusBadRequest, "缺少 hash")
		return
	}
	userID, err := strconv.ParseInt(values.Get("id"), 10, 64)
	if err != nil || userID <= 0 {
		c.String(http.StatusBadRequest, "缺少 Telegram user id")
		return
	}
	authDate, _ := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	maxAge := cfg.TelegramAuthMaxAge
	if maxAge <= 0 {
		maxAge = 86400
	}
	if authDate <= 0 || time.Since(time.Unix(authDate, 0)) > time.Duration(maxAge)*time.Second {
		c.String(http.StatusUnauthorized, "登录已过期，请重试")
		return
	}
	if !telegramHashValid(values, token, hash) {
		c.String(http.StatusUnauthorized, "Telegram 校验失败")
		return
	}
	if !telegramAllowed(userID, cfg.TelegramAllowedIDs) {
		c.String(http.StatusForbidden, fmt.Sprintf("Telegram 账号 %d 不在白名单内", userID))
		return
	}

	middleware.SetAuthCookie(c, cfg.AdminPassword)
	slog.Info("[登录] Telegram 登录成功", "telegram_id", userID, "username", values.Get("username"))
	c.Redirect(http.StatusFound, "/")
}

func telegramHashValid(values url.Values, token, hash string) bool {
	pairs := make([]string, 0, len(values))
	for k, v := range values {
		if k == "hash" || len(v) == 0 {
			continue
		}
		pairs = append(pairs, k+"="+v[0])
	}
	sort.Strings(pairs)
	secret := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(strings.Join(pairs, "\n")))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(hash)))
}

func telegramAllowed(id int64, list string) bool {
	list = strings.TrimSpace(list)
	if list == "" {
		return false // 没配白名单一律拒绝，避免 bot token 泄露即后台失守
	}
	for _, part := range strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, err := strconv.ParseInt(part, 10, 64); err == nil && n == id {
			return true
		}
		if part == "*" {
			return true
		}
	}
	return false
}
