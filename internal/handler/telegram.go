package handler

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"claude2api/internal/config"
	"claude2api/internal/middleware"
)

const (
	tgOIDCIssuer   = "https://oauth.telegram.org"
	tgOIDCAuthURL  = "https://oauth.telegram.org/auth"
	tgOIDCTokenURL = "https://oauth.telegram.org/token"
	tgOIDCJWKSURL  = "https://oauth.telegram.org/.well-known/jwks.json"
)

type tgOIDCState struct {
	Verifier    string
	Nonce       string
	RedirectURI string
	ExpiresAt   time.Time
}

var (
	tgStateMu sync.Mutex
	tgStates  = map[string]tgOIDCState{}

	tgJWKSMu      sync.Mutex
	tgJWKSKeys    map[string]any
	tgJWKSExpires time.Time
)

func oidcEnabled(cfg config.Settings) bool {
	return strings.TrimSpace(cfg.TelegramClientID) != "" && strings.TrimSpace(cfg.TelegramClientSecret) != ""
}

func widgetEnabled(cfg config.Settings) bool {
	return strings.TrimSpace(cfg.TelegramBotToken) != "" && strings.TrimSpace(cfg.TelegramBotName) != ""
}

// TelegramInfo 供前端渲染登录按钮（优先新版 OIDC，其次旧版 Login Widget）。
func TelegramInfo(c *gin.Context) {
	cfg := config.Get()
	if oidcEnabled(cfg) {
		c.JSON(http.StatusOK, gin.H{
			"enabled":   true,
			"mode":      "oidc",
			"client_id": strings.TrimSpace(cfg.TelegramClientID),
			"login_url": "/api/auth/telegram/login",
		})
		return
	}
	if widgetEnabled(cfg) {
		c.JSON(http.StatusOK, gin.H{
			"enabled":  true,
			"mode":     "widget",
			"bot_name": strings.TrimSpace(cfg.TelegramBotName),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": false, "mode": "none"})
}

func randomB64URL(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func buildCallbackURL(c *gin.Context) string {
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if idx := strings.Index(scheme, ","); idx >= 0 {
		scheme = strings.TrimSpace(scheme[:idx])
	}
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if idx := strings.Index(host, ","); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	if host == "" {
		host = c.Request.Host
	}
	return fmt.Sprintf("%s://%s/api/auth/telegram/callback", scheme, host)
}

// TelegramOIDCStart 发起 Telegram OIDC (PKCE S256) 登录重定向。
func TelegramOIDCStart(c *gin.Context) {
	cfg := config.Get()
	if !oidcEnabled(cfg) {
		c.String(http.StatusServiceUnavailable, "Telegram OIDC 登录未启用")
		return
	}

	state, err := randomB64URL(24)
	if err != nil {
		c.String(http.StatusInternalServerError, "生成 state 失败")
		return
	}
	nonce, err := randomB64URL(24)
	if err != nil {
		c.String(http.StatusInternalServerError, "生成 nonce 失败")
		return
	}
	verifier, err := randomB64URL(32)
	if err != nil {
		c.String(http.StatusInternalServerError, "生成 PKCE 失败")
		return
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	redirectURI := buildCallbackURL(c)

	now := time.Now()
	tgStateMu.Lock()
	for k, v := range tgStates {
		if now.After(v.ExpiresAt) {
			delete(tgStates, k)
		}
	}
	tgStates[state] = tgOIDCState{
		Verifier:    verifier,
		Nonce:       nonce,
		RedirectURI: redirectURI,
		ExpiresAt:   now.Add(10 * time.Minute),
	}
	tgStateMu.Unlock()

	q := url.Values{}
	q.Set("client_id", strings.TrimSpace(cfg.TelegramClientID))
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	c.Redirect(http.StatusFound, tgOIDCAuthURL+"?"+q.Encode())
}

// TelegramOIDCCallback 处理 oauth.telegram.org 的 authorization_code 回调。
func TelegramOIDCCallback(c *gin.Context) {
	cfg := config.Get()
	if !oidcEnabled(cfg) {
		c.String(http.StatusServiceUnavailable, "Telegram OIDC 登录未启用")
		return
	}
	if errCode := strings.TrimSpace(c.Query("error")); errCode != "" {
		desc := strings.TrimSpace(c.Query("error_description"))
		c.String(http.StatusUnauthorized, fmt.Sprintf("Telegram 授权失败: %s %s", errCode, desc))
		return
	}

	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		c.String(http.StatusBadRequest, "缺少 code 或 state")
		return
	}

	tgStateMu.Lock()
	st, ok := tgStates[state]
	if ok {
		delete(tgStates, state)
	}
	tgStateMu.Unlock()

	if !ok || time.Now().After(st.ExpiresAt) {
		c.String(http.StatusUnauthorized, "登录状态已过期或无效，请返回重试")
		return
	}

	clientID := strings.TrimSpace(cfg.TelegramClientID)
	clientSecret := strings.TrimSpace(cfg.TelegramClientSecret)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", st.RedirectURI)
	form.Set("client_id", clientID)
	form.Set("code_verifier", st.Verifier)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, tgOIDCTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		c.String(http.StatusInternalServerError, "构造 token 请求失败")
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	basic := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+basic)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Warn("[登录] Telegram OIDC /token 请求失败", "err", err)
		c.String(http.StatusBadGateway, "连接 Telegram OAuth 服务器失败")
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var tokResp struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tokResp); err != nil || resp.StatusCode != http.StatusOK {
		slog.Warn("[登录] Telegram OIDC /token 返回错误", "status", resp.StatusCode, "body", string(body))
		c.String(http.StatusUnauthorized, fmt.Sprintf("换取 Telegram Token 失败 (%d): %s", resp.StatusCode, tokResp.ErrorDesc))
		return
	}
	if tokResp.IDToken == "" {
		c.String(http.StatusUnauthorized, "Telegram 未返回 id_token")
		return
	}

	claims, err := verifyTelegramIDToken(tokResp.IDToken, clientID, st.Nonce)
	if err != nil {
		slog.Warn("[登录] Telegram OIDC id_token 校验失败", "err", err)
		c.String(http.StatusUnauthorized, "Telegram 身份令牌校验失败: "+err.Error())
		return
	}

	userID := claims.UserID()
	if userID <= 0 {
		c.String(http.StatusUnauthorized, "未能从 id_token 解析 Telegram 用户 ID")
		return
	}
	if !telegramAllowed(userID, cfg.TelegramAllowedIDs) {
		c.String(http.StatusForbidden, fmt.Sprintf("Telegram 账号 %d 不在白名单内", userID))
		return
	}

	middleware.SetAuthCookie(c, cfg.AdminPassword)
	slog.Info("[登录] Telegram OIDC 登录成功", "telegram_id", userID, "username", claims.PreferredUsername, "name", claims.Name)
	c.Redirect(http.StatusFound, "/")
}

type tgIDClaims struct {
	Iss               string          `json:"iss"`
	Aud               json.RawMessage `json:"aud"`
	Sub               string          `json:"sub"`
	Nonce             string          `json:"nonce"`
	Exp               int64           `json:"exp"`
	Iat               int64           `json:"iat"`
	ID                json.Number     `json:"id"`
	Name              string          `json:"name"`
	PreferredUsername string          `json:"preferred_username"`
}

func (cl *tgIDClaims) UserID() int64 {
	if s := strings.TrimSpace(cl.ID.String()); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	if n, err := strconv.ParseInt(strings.TrimSpace(cl.Sub), 10, 64); err == nil && n > 0 {
		return n
	}
	return 0
}

func (cl *tgIDClaims) matchAudience(clientID string) bool {
	var single string
	if json.Unmarshal(cl.Aud, &single) == nil {
		return single == clientID
	}
	var multi []string
	if json.Unmarshal(cl.Aud, &multi) == nil {
		for _, a := range multi {
			if a == clientID {
				return true
			}
		}
	}
	return false
}

func getTelegramJWK(kid string, forceRefresh bool) (any, error) {
	tgJWKSMu.Lock()
	if !forceRefresh && tgJWKSKeys != nil && time.Now().Before(tgJWKSExpires) {
		k, ok := tgJWKSKeys[kid]
		tgJWKSMu.Unlock()
		if ok {
			return k, nil
		}
	} else {
		tgJWKSMu.Unlock()
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(tgOIDCJWKSURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS HTTP %d", resp.StatusCode)
	}
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&jwks); err != nil {
		return nil, err
	}

	parsed := make(map[string]any, len(jwks.Keys))
	for _, k := range jwks.Keys {
		switch k.Kty {
		case "RSA":
			nb, err1 := base64.RawURLEncoding.DecodeString(k.N)
			eb, err2 := base64.RawURLEncoding.DecodeString(k.E)
			if err1 != nil || err2 != nil {
				continue
			}
			eInt := new(big.Int).SetBytes(eb)
			parsed[k.Kid] = &rsa.PublicKey{
				N: new(big.Int).SetBytes(nb),
				E: int(eInt.Int64()),
			}
		case "EC":
			if k.Crv != "P-256" {
				continue
			}
			xb, err1 := base64.RawURLEncoding.DecodeString(k.X)
			yb, err2 := base64.RawURLEncoding.DecodeString(k.Y)
			if err1 != nil || err2 != nil {
				continue
			}
			parsed[k.Kid] = &ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     new(big.Int).SetBytes(xb),
				Y:     new(big.Int).SetBytes(yb),
			}
		}
	}

	tgJWKSMu.Lock()
	tgJWKSKeys = parsed
	tgJWKSExpires = time.Now().Add(1 * time.Hour)
	tgJWKSMu.Unlock()

	if pub, ok := parsed[kid]; ok {
		return pub, nil
	}
	return nil, fmt.Errorf("未找到 kid=%s 的公钥", kid)
}

func verifyTelegramIDToken(token, clientID, expectedNonce string) (*tgIDClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("JWT 格式非法")
	}
	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("JWT header 解码失败")
	}
	var hdr struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(hdrBytes, &hdr); err != nil {
		return nil, errors.New("JWT header 解析失败")
	}

	pub, err := getTelegramJWK(hdr.Kid, false)
	if err != nil {
		pub, err = getTelegramJWK(hdr.Kid, true)
		if err != nil {
			return nil, err
		}
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("JWT 签名解码失败")
	}
	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))

	switch hdr.Alg {
	case "RS256":
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("公钥类型不是 RSA")
		}
		if err := rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, digest[:], sig); err != nil {
			return nil, errors.New("RS256 验签失败")
		}
	case "ES256":
		ecPub, ok := pub.(*ecdsa.PublicKey)
		if !ok || len(sig) != 64 {
			return nil, errors.New("ES256 公钥或签名格式不符")
		}
		r := new(big.Int).SetBytes(sig[:32])
		s := new(big.Int).SetBytes(sig[32:])
		if !ecdsa.Verify(ecPub, digest[:], r, s) {
			return nil, errors.New("ES256 验签失败")
		}
	default:
		return nil, fmt.Errorf("不支持的签名算法 %s", hdr.Alg)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("JWT payload 解码失败")
	}
	var claims tgIDClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("JWT claims 解析失败")
	}

	if claims.Iss != tgOIDCIssuer {
		return nil, fmt.Errorf("iss 不匹配: %s", claims.Iss)
	}
	if !claims.matchAudience(clientID) {
		return nil, errors.New("aud 与 client_id 不匹配")
	}
	if claims.Exp <= time.Now().Add(-60*time.Second).Unix() {
		return nil, errors.New("id_token 已过期")
	}
	if expectedNonce != "" && claims.Nonce != expectedNonce {
		return nil, errors.New("nonce 不匹配")
	}
	return &claims, nil
}

// TelegramLogin 处理旧版 Telegram Login Widget 的回调（若同时配置了 bot_token）。
func TelegramLogin(c *gin.Context) {
	cfg := config.Get()
	token := strings.TrimSpace(cfg.TelegramBotToken)
	if token == "" {
		c.String(http.StatusServiceUnavailable, "Telegram Login Widget 未启用")
		return
	}
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
	slog.Info("[登录] Telegram Widget 登录成功", "telegram_id", userID, "username", values.Get("username"))
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
		return false
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
