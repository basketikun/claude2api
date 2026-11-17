package service

import (
	"fmt"
	"net/http"
	"sync"

	"claude2api/internal/config"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type ProxyRoundTripper struct {
	hc    tlsclient.HttpClient
	mu    sync.Mutex
	proxy string
}

func NewProxyRoundTripper() *ProxyRoundTripper {
	hc, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(),
		tlsclient.WithClientProfile(profiles.Chrome_146),
		// 长超时避免镜像 SSE 被代理 CONNECT 截断。
		tlsclient.WithTimeoutSeconds(3600),
		tlsclient.WithNotFollowRedirects())
	if err != nil {
		return &ProxyRoundTripper{proxy: config.Get().Proxy}
	}
	hc.SetCookieJar(nil)
	p := &ProxyRoundTripper{hc: hc, proxy: config.Get().Proxy}
	if p.proxy != "" {
		if err := hc.SetProxy(p.proxy); err != nil {
			p.hc = nil
		}
	}
	return p
}

func (p *ProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if p.hc == nil {
		return nil, fmt.Errorf("上游代理未正确初始化")
	}
	p.mu.Lock()
	if proxy := config.Get().Proxy; proxy != p.proxy {
		if err := p.hc.SetProxy(proxy); err != nil {
			p.mu.Unlock()
			return nil, fmt.Errorf("更新上游代理失败: %w", err)
		}
		p.proxy = proxy
	}
	p.mu.Unlock()
	fr, _ := fhttp.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), req.Body)
	for k, values := range req.Header {
		for _, value := range values {
			fr.Header.Add(k, value)
		}
	}
	fr.Host, fr.ContentLength = req.Host, req.ContentLength
	resp, err := p.hc.Do(fr)
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	for k, values := range resp.Header {
		for _, value := range values {
			h.Add(k, value)
		}
	}
	return &http.Response{Status: resp.Status, StatusCode: resp.StatusCode, Proto: resp.Proto,
		ProtoMajor: resp.ProtoMajor, ProtoMinor: resp.ProtoMinor, Header: h, Body: resp.Body,
		ContentLength: resp.ContentLength, Request: req}, nil
}
