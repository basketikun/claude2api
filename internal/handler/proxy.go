package handler

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"claude2api/internal/config"
	"claude2api/internal/middleware"
	"claude2api/internal/repository"
	"claude2api/internal/service"
	"claude2api/internal/utils"

	"github.com/gin-gonic/gin"
)

const (
	upstreamHost = "claude.ai"
	assetsHost   = "assets-proxy.anthropic.com"
	ucHost       = "www.claudeusercontent.com"
	authPrefix   = "/__auth/"
	authAcctPath = "__acct/"
)

// sessionLimitCapturingReader wraps an io.ReadCloser to capture session limits from SSE streams
type sessionLimitCapturingReader struct {
	io.Reader
	accountEmail string
	buffer       []byte
}

func (r *sessionLimitCapturingReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if n > 0 && r.accountEmail != "" {
		// Append to buffer for parsing
		r.buffer = append(r.buffer, p[:n]...)
		
		// Try to parse for message_limit events
		scanner := bufio.NewScanner(bytes.NewReader(r.buffer))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				go r.parseSessionLimit(line[6:])
			}
		}
		// Keep unprocessed data in buffer
		if len(r.buffer) > 100000 { // Reset buffer if too large
			r.buffer = r.buffer[len(r.buffer)-10000:]
		}
	}
	return n, err
}

func (r *sessionLimitCapturingReader) Close() error {
	if closer, ok := r.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (r *sessionLimitCapturingReader) parseSessionLimit(data string) {
	var ev struct {
		Type         string                    `json:"type"`
		MessageLimit repository.SessionLimits `json:"message_limit"`
	}
	
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		return
	}
	
	if ev.Type == "message_limit" {
		slog.Info("[Proxy] Captured message_limit event from web chat", "email", r.accountEmail)
		if updated := repository.UpdateAccountSessionLimits(r.accountEmail, &ev.MessageLimit); updated {
			slog.Info("[Proxy] Session limits updated successfully from web chat", "email", r.accountEmail)
		} else {
			slog.Warn("[Proxy] Failed to update session limits from web chat", "email", r.accountEmail)
		}
	}
}

// 转发到上游的请求头白名单。
var forwardReqHeaders = map[string]bool{
	"accept": true, "accept-language": true, "content-type": true, "user-agent": true,
	"anthropic-client-platform": true, "anthropic-client-version": true,
	"anthropic-client-sha": true, "anthropic-anonymous-id": true, "anthropic-device-id": true,
	"x-requested-with": true, "sec-fetch-dest": true, "sec-fetch-mode": true, "sec-fetch-site": true,
}

// 上游响应头黑名单。
var stripRespHeaders = map[string]bool{
	"content-encoding": true, "content-length": true, "transfer-encoding": true,
	"connection": true, "content-security-policy": true,
	"content-security-policy-report-only": true, "x-frame-options": true,
	"strict-transport-security": true, "report-to": true, "nel": true, "alt-svc": true,
}

// 需要改写 body 的内容类型。
var rewriteCT = []string{"text/html", "application/javascript", "text/javascript",
	"application/json", "text/css"}

// proxyCtx 供 Director 和 ModifyResponse 共享。
type proxyCtx struct {
	record         *repository.Account
	email          string
	browserCookies string
	onMain         bool
	mainOrig       string
	ucOrig         string
}

type ctxKey struct{}

var newProxy = buildReverseProxy()

// buildReverseProxy 构造共享反代。
func buildReverseProxy() *httputil.ReverseProxy {
	rp := &httputil.ReverseProxy{
		Transport:      service.NewProxyRoundTripper(),
		FlushInterval:  -1,
		Director:       proxyDirector,
		ModifyResponse: proxyModifyResponse,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, context.Canceled) || errors.Is(err, http.ErrAbortHandler) {
				return
			}
			slog.Error("[反代] 上游请求失败", "method", r.Method, "url", r.URL.String(), "err", err)
			http.Error(w, fmt.Sprintf("upstream error: %s", utils.Truncate(err.Error(), 100)), http.StatusBadGateway)
		},
	}
	return rp
}

// proxyDirector 改写上游请求。
func proxyDirector(req *http.Request) {
	pc, _ := req.Context().Value(ctxKey{}).(*proxyCtx)
	if pc == nil {
		return
	}
	path := req.URL.RequestURI()

	orig := req.Header.Clone()

	var targetHost, scheme string
	switch {
	case !pc.onMain:
		targetHost, scheme = ucHost, "https"
	case strings.HasPrefix(path, "/claude-ai/") || strings.HasPrefix(path, "/api/assets/"):
		targetHost, scheme = assetsHost, "https"
	default:
		targetHost, scheme = upstreamHost, "https"
	}

	req.URL.Scheme = scheme
	req.URL.Host = targetHost
	req.Host = targetHost

	req.Header = http.Header{}
	for k, v := range service.BuildHeaders(pc.email) {
		req.Header.Set(k, v)
	}
	for k, vs := range orig {
		if forwardReqHeaders[strings.ToLower(k)] {
			for _, v := range vs {
				req.Header.Set(k, v)
			}
		}
	}
	originURL := scheme + "://" + targetHost
	req.Header.Set("origin", originURL)
	req.Header.Set("referer", originURL+"/")
	req.Header.Set("cookie", mergeProxyCookies(pc.browserCookies, cookieHeader(pc.record)))
}

// maxRewriteBody 是 body 改写上限。
const maxRewriteBody = 10 << 20

type readCloserPair struct {
	io.Reader
	io.Closer
}

// proxyModifyResponse 改写上游响应。
func proxyModifyResponse(resp *http.Response) error {
	pc, _ := resp.Request.Context().Value(ctxKey{}).(*proxyCtx)
	if pc == nil {
		return nil
	}
	ct := resp.Header.Get("content-type")
	isStream := strings.Contains(ct, "text/event-stream")

	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		keys = append(keys, k)
	}
	for _, k := range keys {
		lk := strings.ToLower(k)
		if stripRespHeaders[lk] {
			resp.Header.Del(k)
			continue
		}
		if lk == "location" {
			v := resp.Header.Get(k)
			// /login 重定向短窗口内二次确认后才标失效。
			if pc.onMain && isLoginRedirect(v) {
				if confirmLoginExpired(pc.email) {
					repository.UpdateAccount(pc.email, func(a *repository.Account) { a.Status = "expired" })
					slog.Warn("[号池] 账号会话失效（二次确认），已踢回选择页", "email", pc.email)
				} else {
					slog.Info("[号池] 账号疑似会话失效（首次 /login 重定向），暂不标记", "email", pc.email)
				}
				resp.Header.Set(k, "/")
				resp.Header.Add("Set-Cookie", "pool_acct=; Path=/; Max-Age=0")
				continue
			}
			v = strings.ReplaceAll(v, "https://"+ucHost+"?", pc.ucOrig+"/?")
			v = strings.ReplaceAll(v, "https://"+ucHost, pc.ucOrig)
			v = strings.ReplaceAll(v, "https://"+upstreamHost, pc.mainOrig)
			resp.Header.Set(k, v)
		} else if lk == "set-cookie" {
			vals := resp.Header.Values(k)
			resp.Header.Del(k)
			for _, v := range vals {
				v = domainStripRe.ReplaceAllString(v, "")
				v = strings.ReplaceAll(v, "; Secure", "")
				v = strings.ReplaceAll(v, "; secure", "")
				resp.Header.Add(k, v)
			}
		}
	}

	if isStream {
		resp.Header.Set("Cache-Control", "no-cache")
		// Wrap the response body to capture session limits from event stream
		resp.Body = &sessionLimitCapturingReader{
			Reader:       resp.Body,
			accountEmail: pc.email,
		}
		return nil
	}

	needRewrite := false
	for _, t := range rewriteCT {
		if strings.Contains(ct, t) {
			needRewrite = true
			break
		}
	}
	injectBar := pc.onMain && strings.Contains(ct, "text/html")
	if !needRewrite && !injectBar {
		return nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRewriteBody+1))
	if err != nil {
		resp.Body.Close()
		return err
	}
	if len(data) > maxRewriteBody {
		// 超限时拼回已读内容并透传。
		slog.Info("[反代] 响应过大，跳过域名改写直接透传", "limit_mb", maxRewriteBody>>20, "content_type", ct)
		resp.Body = readCloserPair{io.MultiReader(bytes.NewReader(data), resp.Body), resp.Body}
		return nil
	}
	resp.Body.Close()
	if needRewrite {
		data = rewriteBody(data, pc.mainOrig, pc.ucOrig)
	}
	if injectBar {
		data = injectPoolBar(data, pc.email)
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))
	resp.ContentLength = int64(len(data))
	return nil
}

var domainStripRe = regexp.MustCompile(`;\s*[Dd]omain=[^;]+`)

var (
	loginSuspectLock sync.Mutex
	loginSuspect     = map[string]time.Time{}
)

const loginSuspectWindow = 10 * time.Minute

type artifactTicket struct {
	credential string
	email      string
	expires    time.Time
}

var artifactTickets = struct {
	sync.Mutex
	items map[string]artifactTicket
}{items: map[string]artifactTicket{}}

func issueArtifactTicket(credential, email string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	now := time.Now()
	token := base64.RawURLEncoding.EncodeToString(b)
	artifactTickets.Lock()
	for key, ticket := range artifactTickets.items {
		if now.After(ticket.expires) {
			delete(artifactTickets.items, key)
		}
	}
	artifactTickets.items[token] = artifactTicket{credential, email, now.Add(10 * time.Minute)}
	artifactTickets.Unlock()
	return token, nil
}

// confirmLoginExpired 二次确认 /login 重定向。
func confirmLoginExpired(email string) bool {
	now := time.Now()
	loginSuspectLock.Lock()
	defer loginSuspectLock.Unlock()
	if t, ok := loginSuspect[email]; ok && now.Sub(t) <= loginSuspectWindow {
		delete(loginSuspect, email)
		return true
	}
	loginSuspect[email] = now
	return false
}

// isLoginRedirect 只识别 /login。
func isLoginRedirect(loc string) bool {
	if loc == "" {
		return false
	}
	low := strings.ToLower(loc)
	return strings.HasPrefix(low, "/login") ||
		strings.Contains(low, "claude.ai/login")
}

// ServeMainProxy 处理主站反代。
func ServeMainProxy(c *gin.Context) {
	ServeReverseProxy(c.Writer, c.Request, true)
}

// ServeUCProxy 处理 artifact 反代。
func ServeUCProxy(c *gin.Context) {
	ServeReverseProxy(c.Writer, c.Request, false)
}

// ServeReverseProxy 是反代入口。
func ServeReverseProxy(w http.ResponseWriter, r *http.Request, onMain bool) {
	credential := middleware.RequestCredential(r)
	if onMain {
		if !middleware.PoolCredentialValid(credential) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}
	artifactEmail := ""
	if !onMain {
		if pathCredential, pathAccount, path, ok := artifactAuth(r.URL.Path); ok {
			if !middleware.PoolCredentialValid(pathCredential) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			credential = pathCredential
			artifactEmail = pathAccount
			middleware.SetAuthResponseCookie(w, r, credential)
			r.URL.Path = path
			r.URL.RawPath = ""
		} else if !middleware.PoolCredentialValid(credential) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	browserCookies := parseCookieHeader(r.Header.Get("Cookie"))
	selected := browserCookies["pool_acct"]
	if artifactEmail != "" {
		selected = artifactEmail
		http.SetCookie(w, &http.Cookie{Name: "pool_acct", Value: artifactEmail, Path: "/", MaxAge: 31536000})
	}
	record := service.AccountByEmail(selected)

	if onMain {
		if record == nil {
			if selected != "" {
				http.SetCookie(w, &http.Cookie{Name: "pool_acct", Value: "", Path: "/", MaxAge: -1})
			}
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}
	if record == nil {
		http.Error(w, "no selected account", http.StatusUnauthorized)
		return
	}
	email := record.Email
	mainOrig := requestOrigin(r)
	ucOrig := artifactOrigin(r)
	if onMain {
		ticket, err := issueArtifactTicket(credential, email)
		if err != nil {
			http.Error(w, "artifact authorization unavailable", http.StatusInternalServerError)
			return
		}
		ucOrig += authPrefix + ticket
	}
	if !onMain {
		ucOrig = requestOrigin(r)
		if parentOrigin := strings.TrimSpace(r.URL.Query().Get("parentOrigin")); parentOrigin != "" {
			mainOrig = parentOrigin
		}
	}
	pc := &proxyCtx{
		record:         record,
		email:          email,
		browserCookies: r.Header.Get("Cookie"),
		onMain:         onMain,
		mainOrig:       mainOrig,
		ucOrig:         ucOrig,
	}
	ctx := context.WithValue(r.Context(), ctxKey{}, pc)
	defer func() {
		if v := recover(); v != nil {
			if v == http.ErrAbortHandler {
				return
			}
			panic(v)
		}
	}()
	newProxy.ServeHTTP(w, r.WithContext(ctx))
}

func artifactAuth(path string) (string, string, string, bool) {
	if !strings.HasPrefix(path, authPrefix) {
		return "", "", "", false
	}
	rest := strings.TrimPrefix(path, authPrefix)
	token, path, _ := strings.Cut(rest, "/")
	if token == "" {
		return "", "", "", false
	}
	artifactTickets.Lock()
	ticket, found := artifactTickets.items[token]
	if found && time.Now().After(ticket.expires) {
		delete(artifactTickets.items, token)
		found = false
	}
	artifactTickets.Unlock()
	if !found {
		return "", "", "", false
	}
	return ticket.credential, ticket.email, "/" + path, true
}

// IsUCHost 判断 artifact Host。
func IsUCHost(hostport string) bool {
	host := hostname(hostport)
	if configured := hostname(config.Get().WebUCHost); configured != "" {
		return strings.EqualFold(host, configured)
	}
	return strings.EqualFold(host, "localhost") || strings.HasPrefix(strings.ToLower(host), "uc.")
}

func artifactOrigin(r *http.Request) string {
	scheme := requestScheme(r)
	mainHost := hostname(r.Host)
	ucHostName := hostname(config.Get().WebUCHost)
	if ucHostName == "" {
		if mainHost == "127.0.0.1" || mainHost == "::1" {
			ucHostName = "localhost"
		} else {
			ucHostName = "uc." + mainHost
		}
	}
	if port := requestPort(r.Host); port != "" {
		ucHostName = net.JoinHostPort(ucHostName, port)
	}
	return scheme + "://" + ucHostName
}

func requestOrigin(r *http.Request) string {
	return requestScheme(r) + "://" + r.Host
}

func requestScheme(r *http.Request) string {
	scheme := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	return scheme
}

func hostname(hostport string) string {
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	return strings.Trim(hostport, "[]")
}

func requestPort(hostport string) string {
	_, port, _ := net.SplitHostPort(hostport)
	return port
}

var skipAccountCookies = map[string]bool{
	"cf_clearance": true,
	"__cf_bm":      true,
}

func cookieHeader(record *repository.Account) string {
	ck := record.Cookies
	parts := make([]string, 0, len(ck))
	for k, v := range ck {
		if skipAccountCookies[k] {
			continue
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

func mergeProxyCookies(browser, account string) string {
	merged := map[string]string{}
	for _, header := range []string{browser, account} {
		for _, part := range strings.Split(header, ";") {
			name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
			if ok && name != middleware.AuthCookieName && name != "pool_acct" && name != "pool_cid" && name != "mirror" {
				merged[name] = value
			}
		}
	}
	parts := make([]string, 0, len(merged))
	for name, value := range merged {
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

// parseCookieHeader 解析 Cookie，并还原 URL 编码值。
func parseCookieHeader(header string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(header, ";") {
		if i := strings.IndexByte(part, '='); i >= 0 {
			key := strings.TrimSpace(part[:i])
			val := strings.TrimSpace(part[i+1:])
			if dec, err := url.QueryUnescape(val); err == nil {
				val = dec
			}
			out[key] = val
		}
	}
	return out
}

// rewriteBody 改写官网域名。
func rewriteBody(data []byte, mainOrigin, ucOrigin string) []byte {
	text := string(data)
	ucNetloc := strings.SplitN(ucOrigin, "//", 2)
	ucNet := ucOrigin
	if len(ucNetloc) == 2 {
		ucNet = ucNetloc[1]
	}
	text = strings.ReplaceAll(text, "https://"+ucHost, ucOrigin)
	text = strings.ReplaceAll(text, "https://claudeusercontent.com", ucOrigin)
	text = strings.ReplaceAll(text, ucHost, ucNet)
	text = strings.ReplaceAll(text, "claudeusercontent.com", ucNet)
	mainNet := mainOrigin
	if p := strings.SplitN(mainOrigin, "//", 2); len(p) == 2 {
		mainNet = p[1]
	}
	text = strings.ReplaceAll(text, "https://"+assetsHost, mainOrigin)
	text = strings.ReplaceAll(text, "https://"+upstreamHost, mainOrigin)
	text = strings.ReplaceAll(text, "//"+upstreamHost, "//"+mainNet)
	return []byte(text)
}

// injectPoolBar 注入账号浮条。
func injectPoolBar(data []byte, email string) []byte {
	text := string(data)
	if !strings.Contains(text, "</body>") {
		return data
	}
	safeEmail := html.EscapeString(email)
	if safeEmail == "" {
		safeEmail = "—"
	}

	// Get account info for initial render
	account := service.AccountByEmail(email)
	sessionUsage := "—"
	resetTime := "—"
	statusText := "—"
	
	if account != nil && account.SessionLimits != nil && account.SessionLimits.Resolved != nil && account.SessionLimits.Resolved.Limit != nil {
		limit := account.SessionLimits.Resolved.Limit
		sessionUsage = fmt.Sprintf("%d%%", limit.Percent)
		if limit.ResetsAt != "" {
			resetTime = limit.ResetsAt
		}
		statusText = html.EscapeString(string(account.Status))
	}

	bar := `<div id="__pool_bar__" class="pb-card">` +
		`<div class="pb-header">` +
		`<span class="pb-acct" title="` + safeEmail + `">` +
		`<span class="pb-label">Account</span>` +
		`<b class="pb-email">` + safeEmail + `</b></span>` +
		`<a class="pb-switch" href="/">Switch</a>` +
		`</div>` +
		`<div class="pb-stats">` +
		`<div class="pb-stat"><span class="pb-stat-label">Usage</span><span class="pb-stat-value" id="pb-usage">` + sessionUsage + `</span></div>` +
		`<div class="pb-stat"><span class="pb-stat-label">Resets</span><span class="pb-stat-value" id="pb-reset">` + resetTime + `</span></div>` +
		`<div class="pb-stat"><span class="pb-stat-label">Status</span><span class="pb-stat-value" id="pb-status">` + statusText + `</span></div>` +
		`</div>` +
		`</div>`

	css := `<style>` +
		`#__pool_bar__.pb-card{position:fixed;right:14px;top:14px;z-index:2147483647;` +
		`display:flex;flex-direction:column;gap:10px;max-width:420px;min-width:320px;box-sizing:border-box;` +
		`padding:12px 14px;` +
		`font:12px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI","Microsoft YaHei",sans-serif;` +
		`color:#3d3a34;background:rgba(252,251,248,.95);` +
		`-webkit-backdrop-filter:saturate(1.4) blur(12px);backdrop-filter:saturate(1.4) blur(12px);` +
		`border:1px solid #e7e3da;border-radius:12px;` +
		`box-shadow:0 8px 32px -8px rgba(60,50,30,.2),0 2px 6px rgba(60,50,30,.1);` +
		`animation:pbin .3s cubic-bezier(.2,.8,.3,1)}` +
		`@keyframes pbin{from{opacity:0;transform:translateY(-10px)}to{opacity:1;transform:none}}` +
		`#__pool_bar__ .pb-header{display:flex;align-items:center;justify-content:space-between;gap:10px}` +
		`#__pool_bar__ .pb-acct{display:flex;align-items:baseline;gap:5px;min-width:0;flex:1}` +
		`#__pool_bar__ .pb-label{color:#9a968c;flex:none;font-size:10px;font-weight:700;text-transform:uppercase;letter-spacing:0.05em}` +
		`#__pool_bar__ .pb-email{font-weight:600;color:#1f1e1c;white-space:nowrap;overflow:hidden;` +
		`text-overflow:ellipsis;min-width:0;font-size:13px}` +
		`#__pool_bar__ .pb-switch{flex:none;color:#c96442;font-weight:600;text-decoration:none;` +
		`padding:4px 10px;border-radius:6px;transition:all .12s;font-size:11px;border:1px solid transparent}` +
		`#__pool_bar__ .pb-switch:hover{background:rgba(201,100,66,.12);border-color:rgba(201,100,66,.3)}` +
		`#__pool_bar__ .pb-stats{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;` +
		`padding-top:10px;border-top:1px solid #eae6dd}` +
		`#__pool_bar__ .pb-stat{display:flex;flex-direction:column;gap:3px}` +
		`#__pool_bar__ .pb-stat-label{font-size:9px;font-weight:700;text-transform:uppercase;` +
		`letter-spacing:0.06em;color:#9a968c}` +
		`#__pool_bar__ .pb-stat-value{font-size:14px;font-weight:650;color:#1f1e1c}` +
		`#__pool_bar__ .pb-stat-value.usage-high{color:#b4402f}` +
		`#__pool_bar__ .pb-stat-value.usage-medium{color:#b7791f}` +
		`#__pool_bar__ .pb-stat-value.usage-low{color:#3c7d47}` +
		`@media (max-width:640px){#__pool_bar__.pb-card{right:10px;top:10px;max-width:calc(100vw - 20px);min-width:280px}}` +
		`</style>`

	script := `<script>` +
		`(function(){` +
		`console.log('[PoolBar] Real-time session monitor initialized');` +
		`var currentEmail='` + safeEmail + `';` +
		`var refreshing=false;` +
		`var updateSessionInfo=function(account){` +
		`if(!account||!account.session_limits)return;` +
		`var limit=account.session_limits.resolved?.limit;` +
		`if(!limit)return;` +
		`console.log('[PoolBar] Updating to:',limit.percent+'%');` +
		`var usageEl=document.getElementById('pb-usage');` +
		`var resetEl=document.getElementById('pb-reset');` +
		`var statusEl=document.getElementById('pb-status');` +
		`if(usageEl){` +
		`var percent=limit.percent||0;` +
		`usageEl.textContent=percent+'%';` +
		`usageEl.className='pb-stat-value';` +
		`if(percent>=90)usageEl.classList.add('usage-high');` +
		`else if(percent>=70)usageEl.classList.add('usage-medium');` +
		`else usageEl.classList.add('usage-low');}` +
		`if(resetEl&&limit.resets_at){` +
		`resetEl.dataset.resetTime=limit.resets_at;` +
		`updateResetTime();}` +
		`if(statusEl&&account.status){` +
		`statusEl.textContent=account.status;}};` +
		`var updateResetTime=function(){` +
		`var resetEl=document.getElementById('pb-reset');` +
		`if(!resetEl||!resetEl.dataset.resetTime)return;` +
		`var resetDate=new Date(resetEl.dataset.resetTime);` +
		`var now=new Date();` +
		`var diff=resetDate-now;` +
		`if(diff<=0){resetEl.textContent='Reset';}` +
		`else{` +
		`var minutes=Math.floor(diff/60000);` +
		`var hours=Math.floor(minutes/60);` +
		`var days=Math.floor(hours/24);` +
		`if(days>0)resetEl.textContent=days+'d '+(hours%24)+'h';` +
		`else if(hours>0)resetEl.textContent=hours+'h '+(minutes%60)+'m';` +
		`else if(minutes>0)resetEl.textContent=minutes+'m';` +
		`else resetEl.textContent='<1m';}};` +
		`var refreshSessionData=function(){` +
		`if(refreshing)return;` +
		`refreshing=true;` +
		`console.log('[PoolBar] Refreshing session data...');` +
		`fetch('/pool/api/refresh',{` +
		`method:'POST',` +
		`credentials:'include'` +
		`}).then(function(r){` +
		`if(!r.ok){console.error('[PoolBar] Refresh failed:',r.status);return;}` +
		`return r.json();` +
		`}).then(function(data){` +
		`if(!data||!data.account)return;` +
		`console.log('[PoolBar] Refresh complete, updating banner');` +
		`updateSessionInfo(data.account);` +
		`}).catch(function(e){` +
		`console.error('[PoolBar] Refresh error:',e);` +
		`}).finally(function(){` +
		`setTimeout(function(){refreshing=false;},2000);` +
		`});};` +
		`var originalFetch=window.fetch;` +
		`var messageCount=0;` +
		`window.fetch=function(){` +
		`var args=arguments;` +
		`var url=typeof args[0]==='string'?args[0]:(args[0]&&args[0].url)||'';` +
		`var method=(args[1]&&args[1].method)||'GET';` +
		`var promise=originalFetch.apply(this,args);` +
		`if(method==='POST'&&(url.includes('append_message')||url.includes('chat_conversations'))){` +
		`messageCount++;` +
		`console.log('[PoolBar] Message #'+messageCount+' detected');` +
		`promise.then(function(response){` +
		`if(response.ok){` +
		`console.log('[PoolBar] Message completed, refreshing in 3s...');` +
		`setTimeout(refreshSessionData,3000);}` +
		`return response;` +
		`});}` +
		`return promise;};` +
		`var resetEl=document.getElementById('pb-reset');` +
		`if(resetEl&&resetEl.textContent&&resetEl.textContent.includes('202')){` +
		`resetEl.dataset.resetTime=resetEl.textContent;` +
		`updateResetTime();}` +
		`setInterval(updateResetTime,1000);` +
		`})();` +
		`</script>`

	text = strings.Replace(text, "</body>", css+bar+script+"</body>", 1)
	return []byte(text)
}
