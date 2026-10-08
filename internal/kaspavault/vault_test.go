package kaspavault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"zkas-node-manager/internal/wallet"
)

func TestSeparateVaultAndBackup(t *testing.T) {
	root := t.TempDir()
	password := "public testing password"
	z := &wallet.Vault{Wallets: []wallet.Wallet{{Name: "original ZKas wallet"}}}
	if e := wallet.Save(root, password, z); e != nil {
		t.Fatal(e)
	}
	original, _ := os.ReadFile(wallet.Path(root))
	v := &Vault{Wallets: []Wallet{{ID: "easy-test", Name: "Test", Password: "internal-random-secret", Passphrase: "recovery-passphrase", Backup: []byte("encrypted sdk wallet")}}}
	if e := Save(root, password, v); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(Path(root))
	if strings.Contains(string(b), "internal-random-secret") || strings.Contains(string(b), "recovery-passphrase") {
		t.Fatal("plaintext secret")
	}
	restored, e := Load(root, password)
	if e != nil || restored.Find("easy-test").Password != v.Wallets[0].Password {
		t.Fatal(e)
	}
	if _, e = Load(root, "wrong password"); e == nil {
		t.Fatal("accepted wrong password")
	}
	backup := filepath.Join(t.TempDir(), "backup.json")
	os.WriteFile(backup, b, 0600)
	if r, e := LoadFile(backup, password); e != nil || string(r.Wallets[0].Backup) != "encrypted sdk wallet" {
		t.Fatal("backup lost", e)
	}
	// Distinct authenticated context prevents treating a ZKas vault as Kaspa.
	os.WriteFile(backup, original, 0600)
	if _, e = LoadFile(backup, password); e == nil {
		t.Fatal("accepted ZKas vault")
	}
	after, _ := os.ReadFile(wallet.Path(root))
	if string(original) != string(after) {
		t.Fatal("modified ZKas vault")
	}
	b[len(b)/2] ^= 1
	os.WriteFile(Path(root), b, 0600)
	if _, e = Load(root, password); e == nil {
		t.Fatal("accepted tampering")
	}
}
