package service

import (
	"io"
	"log/slog"
	"sync"
	"time"

	"claude2api/internal/repository"

	fhttp "github.com/bogdanfinn/fhttp"
)

// keepAliveInterval stays well under typical NAT/proxy idle cutoffs so each
// account's upstream connection survives between round-robin turns.
const keepAliveInterval = 45 * time.Second

// Ping sends a cheap authenticated request on the existing connection.
func (claudeAI *ClaudeAI) Ping() error {
	req, err := claudeAI.request(fhttp.MethodGet, claudeAIBaseURL+"/api/account", nil)
	if err != nil {
		return err
	}
	resp, err := claudeAI.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}

// StartConnectionKeeper pre-warms and then periodically pings every usable
// account so user requests reuse a live connection instead of paying a fresh
// (and frequently stalled) residential-proxy handshake.
func StartConnectionKeeper() {
	go func() {
		for {
			keepAllWarm()
			time.Sleep(keepAliveInterval)
		}
	}()
}

func keepAllWarm() {
	accounts := repository.LoadAccounts()
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i := range accounts {
		acct := accounts[i]
		if !AccountUsable(&acct) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			lease := clientFor(&acct, AccountProxy(&acct))
			// Never block or delay a live user request; skip busy accounts.
			if !lease.TryLock() {
				return
			}
			defer lease.Unlock()
			client := lease.ClaudeAI
			if !lease.ready {
				if err := client.WarmUp(); err != nil {
					slog.Debug("[保活] 预热失败", "email", acct.Email, "err", err)
					return
				}
				lease.ready = true
				return
			}
			if err := client.Ping(); err != nil {
				slog.Debug("[保活] 心跳失败", "email", acct.Email, "err", err)
			}
		}()
	}
	wg.Wait()
}
