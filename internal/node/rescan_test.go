package node

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRescanIncludesDaemonWalletsAndPreservesFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "wallets")
	os.Mkdir(dir, 0700)
	original := `{"version":1,"network":"mainnet","seed_hex":"","fvk_hex":"test"}`
	for _, name := range []string{"bot-token", "manager-token"} {
		os.WriteFile(filepath.Join(dir, name+".json"), []byte(original), 0600)
		os.WriteFile(filepath.Join(dir, name+".scan"), []byte("checkpoint"), 0600)
	}
	tokens, err := RegisteredWalletTokens(root)
	if err != nil || len(tokens) != 2 {
		t.Fatalf("registrations: %d %v", len(tokens), err)
	}
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Path != "/api/wallet/rescan" || string(body) != `{"force":false,"birthday":0}` {
			t.Error("incorrect rescan request")
		}
		seen = append(seen, r.Header.Get("X-Wallet-Token"))
		io.WriteString(w, `{"rescanning":true}`)
	}))
	defer server.Close()
	n, err := rescanWallets(context.Background(), server.Client(), server.URL, tokens, nil)
	if err != nil || n != 2 || !reflect.DeepEqual(tokens, seen) {
		t.Fatalf("rescan: %d %v", n, err)
	}
	for _, token := range tokens {
		data, _ := os.ReadFile(filepath.Join(dir, token+".json"))
		scan, _ := os.ReadFile(filepath.Join(dir, token+".scan"))
		if string(data) != original || string(scan) != "checkpoint" {
			t.Fatal("manager altered wallet files")
		}
	}
}

func TestRescanStopsAtRefusalAndCountsPartialSuccess(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(409)
			io.WriteString(w, `{"error":"secret-token"}`)
			return
		}
		io.WriteString(w, `{"rescanning":true}`)
	}))
	defer server.Close()
	n, err := rescanWallets(context.Background(), server.Client(), server.URL, []string{"first", "second", "third"}, nil)
	if n != 1 || calls != 2 || err == nil {
		t.Fatalf("partial result: %d %d %v", n, calls, err)
	}
}

func TestInvalidRegistrationStopsBeforeRequests(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "wallets"), 0700)
	os.WriteFile(filepath.Join(root, "wallets", "broken.json"), []byte("{"), 0600)
	if _, err := RegisteredWalletTokens(root); err == nil {
		t.Fatal("invalid registration accepted")
	}
}
