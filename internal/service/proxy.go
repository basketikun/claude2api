package service

import (
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
	hc, _ := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(),
		tlsclient.WithClientProfile(profiles.Chrome_146),
		// 长超时避免镜像 SSE 被代理 CONNECT 截断。
		tlsclient.WithTimeoutSeconds(3600),
		tlsclient.WithInsecureSkipVerify(),
		tlsclient.WithNotFollowRedirects())
	hc.SetCookieJar(nil)
	p := &ProxyRoundTripper{hc: hc, proxy: config.Get().Proxy}
	_ = hc.SetProxy(p.proxy)
	return p
}

func (p *ProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	p.mu.Lock()
	if proxy := config.Get().Proxy; proxy != p.proxy {
		_ = p.hc.SetProxy(proxy)
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
