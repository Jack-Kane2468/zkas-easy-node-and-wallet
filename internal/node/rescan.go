package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RegisteredWalletTokens includes daemon registrations, independently of the manager vault.
// Tokens are credentials and must not be displayed or logged.
func RegisteredWalletTokens(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "wallets"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Cannot read the wallet registrations")
	}
	var tokens []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "wallets", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("Cannot read a wallet registration; no rescans requested")
		}
		var meta struct {
			Seed    *string `json:"seed_hex"`
			Network string  `json:"network"`
		}
		if json.Unmarshal(data, &meta) != nil || meta.Seed == nil || meta.Network == "" {
			return nil, fmt.Errorf("An invalid wallet registration was found; no rescans requested")
		}
		token := strings.TrimSuffix(entry.Name(), ".json")
		if token == "" || strings.ContainsAny(token, "\r\n") {
			return nil, fmt.Errorf("An invalid wallet registration was found; no rescans requested")
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

// RescanRegisteredWallets restarts scans through the official API, retaining its
// genesis-availability guard. It never edits wallet files or bypasses that guard.
func RescanRegisteredWallets(ctx context.Context, port int, tokens []string, progress func(int, int)) (int, error) {
	client := &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	return rescanWallets(ctx, client, fmt.Sprintf("http://127.0.0.1:%d", port), tokens, progress)
}

func rescanWallets(ctx context.Context, client *http.Client, base string, tokens []string, progress func(int, int)) (int, error) {
	for i, token := range tokens {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/wallet/rescan", strings.NewReader(`{"force":false,"birthday":0}`))
		if err != nil {
			return i, fmt.Errorf("Could not prepare the rescan request")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Wallet-Token", token)
		resp, err := client.Do(req)
		if err != nil {
			return i, fmt.Errorf("Wallet backend did not confirm the request. Check that it is running; the last request may have been accepted")
		}
		var reply struct {
			Rescanning bool `json:"rescanning"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&reply)
		resp.Body.Close()
		if resp.StatusCode == http.StatusConflict {
			return i, fmt.Errorf("The backend refused the rescan because this node cannot currently serve history from genesis. No history check was bypassed")
		}
		if resp.StatusCode != http.StatusOK || decodeErr != nil || !reply.Rescanning {
			return i, fmt.Errorf("The backend did not confirm rescan (HTTP %d). Check the wallet backend log", resp.StatusCode)
		}
		if progress != nil {
			progress(i+1, len(tokens))
		}
	}
	return len(tokens), nil
}
