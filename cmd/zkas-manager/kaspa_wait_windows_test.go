package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"zkas-node-manager/internal/kaspavault"
)

func TestKaspaQueueWaitHonorsDeadlineWithoutStoppingOwner(t *testing.T) {
	p := &kaspaWalletProcess{}
	p.mu.Lock()
	defer p.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := p.callContext(ctx, map[string]any{"op": "status"}, nil)
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("unbounded wait: %v", err)
	}
	if p.stopped.Load() {
		t.Fatal("queued reader must not stop another request")
	}
}
func TestKaspaAvailabilityMatchesLocalFiles(t *testing.T) {
	m := &manager{}
	m.root = t.TempDir()
	m.kwVault = &kaspavault.Vault{Wallets: []kaspavault.Wallet{{ID: "present"}, {ID: "missing"}}}
	os.MkdirAll(filepath.Join(m.root, "kaspa-wallets"), 0700)
	os.WriteFile(filepath.Join(m.root, "kaspa-wallets", "present.wallet"), []byte("test"), 0600)
	if !m.kwWalletAvailable("present") || m.kwWalletAvailable("missing") || m.kwWalletAvailable("../present") {
		t.Fatal("availability mismatch")
	}
}
