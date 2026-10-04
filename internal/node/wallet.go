package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// This checks daemon availability only, never a wallet balance or spend readiness.
func WalletHealth(ctx context.Context, port int) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/health", port), nil)
	if e != nil {
		return e
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	resp, e := client.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("wallet health returned %s", resp.Status)
	}
	var result struct {
		OK      bool   `json:"ok"`
		Service string `json:"service"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&result); e != nil {
		return e
	}
	if !result.OK || result.Service != "zkas-walletd" {
		return fmt.Errorf("port %d did not identify as zkas-walletd", port)
	}
	return nil
}
