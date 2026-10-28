package telegrambot

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsSettingsProtectedAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	c := Config{Token: "12345:private-test-token-0123456789", OwnerOnly: true, OwnerID: 42, Permissions: Permissions{Status: true}, Watches: []WatchWallet{{ID: "watch", Name: "test", FVK: "private-viewing-key", Token: "local-wallet-token"}}}
	if err := Save(root, c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "telegram-bot", "settings.bin"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{c.Token, c.Watches[0].FVK, c.Watches[0].Token} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("plaintext secret on disk")
		}
	}
	got, err := Load(root)
	if err != nil || got.Token != c.Token || got.OwnerID != 42 || len(got.Watches) != 1 || got.Watches[0].FVK != c.Watches[0].FVK {
		t.Fatal("round trip failed", err)
	}
	c.OwnerID = 99
	if err = Save(root, c); err != nil {
		t.Fatal("replace failed", err)
	}
	got, err = Load(root)
	if err != nil || got.OwnerID != 99 {
		t.Fatal("replace did not persist", err)
	}
	if err = os.WriteFile(filepath.Join(root, "telegram-bot", "settings.bin"), []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(root); err == nil {
		t.Fatal("corrupt config accepted")
	}
}
