package wallet

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestVaultEncryptionAuthenticationAndIsolation(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "config.json"), []byte("existing config"), 0600)
	password := "test password for tests only"
	v := &Vault{Wallets: []Wallet{{Name: "Test", Secret: "fictional secret", Token: "fictional token", NextIndex: 5}}}
	if e := Save(root, password, v); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(Path(root))
	if strings.Contains(string(b), "fictional") {
		t.Fatal("plaintext secret stored")
	}
	got, e := Load(root, password)
	if e != nil || got.Wallets[0].Secret != v.Wallets[0].Secret || got.Wallets[0].NextIndex != 5 {
		t.Fatal(got, e)
	}
	if _, e = Load(root, "wrong password"); e == nil {
		t.Fatal("wrong password accepted")
	}
	var env Envelope
	json.Unmarshal(b, &env)
	if env.Data[0] == 'a' {
		env.Data = "b" + env.Data[1:]
	} else {
		env.Data = "a" + env.Data[1:]
	}
	tampered, _ := json.Marshal(env)
	os.WriteFile(Path(root), tampered, 0600)
	if _, e = Load(root, password); e == nil {
		t.Fatal("tampered vault accepted")
	}
	c, _ := os.ReadFile(filepath.Join(root, "config.json"))
	if string(c) != "existing config" {
		t.Fatal("node config changed")
	}
}
func TestExactAmounts(t *testing.T) {
	for s, want := range map[string]uint64{"1": 100000000, "0.00000001": 1, "1.12345678": 112345678, "184467440737.09551615": ^uint64(0)} {
		n, e := ParseAmount(s)
		if e != nil || n != want {
			t.Fatal(s, n, e)
		}
	}
	for _, s := range []string{"0", "-1", "1e8", "NaN", "1.123456789", "184467440737.09551616", "1,5"} {
		if _, e := ParseAmount(s); e == nil {
			t.Fatal("accepted", s)
		}
	}
}
func TestOVKMatchesOfficialOrchardVectors(t *testing.T) {
	b, e := os.ReadFile("testdata/orchard-key-vectors.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []map[string]string
	json.Unmarshal(b, &cases)
	if len(cases) < 10 {
		t.Fatal("missing vectors")
	}
	for _, v := range cases {
		got, e := OVK(v["ak"] + v["nk"] + v["rivk"])
		if e != nil || got != v["ovk"] {
			t.Fatal("OVK vector mismatch", e)
		}
	}
}
func TestPreparedPaymentRejectsChanges(t *testing.T) {
	p := Prepared{Session: "session", Bundle: "bundle", Disclosure: json.RawMessage(`[]`), SpendAuth: json.RawMessage(`[]`), Amount: 100, Fee: 2}
	if e := p.Validate(100, 3); e != nil {
		t.Fatal(e)
	}
	p.Amount = 99
	if p.Validate(100, 3) == nil {
		t.Fatal("changed amount")
	}
	p.Amount = 100
	p.Remaining = 1
	if p.Validate(100, 3) == nil {
		t.Fatal("partial payment")
	}
	p.Remaining = 0
	p.Fee = 4
	if p.Validate(100, 3) == nil {
		t.Fatal("excess fee")
	}
	p.Fee = 2
	p.Bundle = ""
	if p.Validate(100, 3) == nil {
		t.Fatal("blind signing")
	}
}
func TestWalletRequestsContainViewingKeyNotSeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Wallet-Token") != "token" {
			t.Error("lost wallet token")
		}
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		if _, ok := b["seed"]; ok {
			t.Error("seed sent")
		}
		if _, ok := b["secret"]; ok {
			t.Error("secret sent")
		}
		if b["fvk_hex"] != "viewing-key" {
			t.Error("missing viewing key")
		}
		if r.URL.Path == "/api/wallet/prepare" && b["amount_sompi"] != "9007199254740993" {
			t.Error("amount lost precision")
		}
		w.Write([]byte(`{"session":"x","bundle_hex":"x","disclosure":[],"spend_auth":[],"amount_sompi":9007199254740993,"fee_sompi":1}`))
	}))
	defer server.Close()
	_, p, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	port, _ := strconv.Atoi(p)
	c := Client{Port: port, Token: "token"}
	if e := c.Watch(context.Background(), "viewing-key"); e != nil {
		t.Fatal(e)
	}
	got, e := c.Prepare(context.Background(), "viewing-key", "zkas:test", 9007199254740993)
	if e != nil || got.Amount != 9007199254740993 {
		t.Fatal(got, e)
	}
}
