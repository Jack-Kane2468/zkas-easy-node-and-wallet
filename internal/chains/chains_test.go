package chains

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMiningIsolation(t *testing.T) {
	c := Config{Mode: "merged", KaspaAddress: "kaspa:" + strings.Repeat("q", 61), ZKasAddress: "zkas:" + strings.Repeat("p", 79)}
	b, e := c.BridgeConfig(16810)
	if e != nil {
		t.Fatal(e)
	}
	var v map[string]any
	json.Unmarshal(b, &v)
	if v["kaspad_address"] != "127.0.0.1:16810" || v["merged_kaspa_address"] != "127.0.0.1:16110" || v["merged_kaspa_pay_address"] != c.KaspaAddress {
		t.Fatal(v)
	}
	if strings.Contains(string(b), "0.0.0.0") {
		t.Fatal("unexpected exposure")
	}
	c.Mode = "kaspa"
	c.ZKasAddress = ""
	c.LAN = true
	b, e = c.BridgeConfig(16810)
	if e != nil || strings.Contains(string(b), "merged_kaspa") || !strings.Contains(string(b), "0.0.0.0:5555") {
		t.Fatal(string(b), e)
	}
	c.KaspaAddress = "kaspa:bad\nattack: value"
	if _, e = c.BridgeConfig(16810); e == nil {
		t.Fatal("accepted malformed input")
	}
	c.Mode = "wrong"
	if c.ValidateMining() == nil {
		t.Fatal("unknown mode")
	}
}
func TestExtractRejectsUnexpectedArchive(t *testing.T) {
	for _, names := range [][]string{{"../kaspad.exe"}, {"kaspad.exe", "kaspad.exe"}, {"other.exe"}} {
		dir := t.TempDir()
		p := filepath.Join(dir, "x.zip")
		f, _ := os.Create(p)
		z := zip.NewWriter(f)
		for _, n := range names {
			w, _ := z.Create(n)
			w.Write([]byte("MZtest"))
		}
		z.Close()
		f.Close()
		if _, e := Extract(p, filepath.Join(dir, "out"), "v1", []string{"kaspad.exe"}); e == nil {
			t.Fatal(names)
		}
	}
}
func TestConfigAndDataIsolation(t *testing.T) {
	root := t.TempDir()
	c, e := Read(root)
	if e != nil || c.Mode != "merged" {
		t.Fatal(c, e)
	}
	c.KaspaAddress = "kaspa:test"
	if e = Save(root, c); e != nil {
		t.Fatal(e)
	}
	d, e := Read(root)
	if e != nil || d.KaspaAddress != c.KaspaAddress {
		t.Fatal(d, e)
	}
	if DataDir(root) == filepath.Join(root, "data") {
		t.Fatal("shared database")
	}
	args := strings.Join(KaspaArgs(root), " ")
	for _, x := range []string{"--utxoindex", "--disable-upnp", "127.0.0.1:16110", "127.0.0.1:18110", "--ram-scale=0.3"} {
		if !strings.Contains(args, x) {
			t.Fatal(args)
		}
	}
}
func TestReleaseArchives(t *testing.T) {
	for _, x := range []struct {
		env, version string
		names        []string
	}{{"KASPA_RELEASE_ZIP", KaspaVersion, []string{"kaspad.exe", "stratum-bridge.exe", "kaspa-wallet.exe"}}, {"MERGED_RELEASE_ZIP", DualVersion, []string{"stratum-bridge.exe"}}} {
		p := os.Getenv(x.env)
		if p == "" {
			continue
		}
		got, e := Extract(p, t.TempDir(), x.version, x.names)
		if e != nil {
			t.Fatal(e)
		}
		for _, b := range got {
			if e = b.Check(); e != nil {
				t.Fatal(e)
			}
		}
	}
}

func TestZKasOnlyRequiresNoKaspaAddressOrParent(t *testing.T) {
	c := Config{Mode: "zkas", ZKasAddress: "zkas:" + strings.Repeat("p", 79)}
	b, e := c.BridgeConfig(16810)
	if e != nil {
		t.Fatal(e)
	}
	var v map[string]any
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	if v["kaspad_address"] != "127.0.0.1:16810" {
		t.Fatal(v)
	}
	if strings.Contains(string(b), "merged_kaspa") || strings.Contains(string(b), "16110") {
		t.Fatal("unexpected Kaspa dependency", string(b))
	}
	c.ZKasAddress = ""
	if c.ValidateMining() == nil {
		t.Fatal("missing ZKas address accepted")
	}
}

func TestWalletOnlyExtractionPreservesRunningNode(t *testing.T) {
	p := os.Getenv("KASPA_RELEASE_ZIP")
	if p == "" {
		t.Skip("set release archive")
	}
	dir := t.TempDir()
	nodePath := filepath.Join(dir, "kaspad.exe")
	if e := os.WriteFile(nodePath, []byte("running-node-placeholder"), 0600); e != nil {
		t.Fatal(e)
	}
	out, e := Extract(p, dir, KaspaVersion, []string{"kaspa-wallet.exe"})
	if e != nil {
		t.Fatal(e)
	}
	if e = out["kaspa-wallet.exe"].Check(); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(nodePath)
	if e != nil || string(b) != "running-node-placeholder" {
		t.Fatal("node file changed")
	}
}

func TestMemorySettingsPreserveExistingInstallation(t *testing.T) {
	root := t.TempDir()
	old := `{"mode":"kaspa","kaspa":{"path":"existing.exe","sha256":"saved","version":"v2.1.0"},"kaspaAddress":"saved-address"}`
	if e := os.WriteFile(filepath.Join(root, "chains.json"), []byte(old), 0600); e != nil {
		t.Fatal(e)
	}
	c, e := Read(root)
	if e != nil || c.RAMScale != 0.3 {
		t.Fatal(c, e)
	}
	c.RAMScale = 0.6
	if e = Save(root, c); e != nil {
		t.Fatal(e)
	}
	got, e := Read(root)
	if e != nil || got.Kaspa.Path != "existing.exe" || got.KaspaAddress != "saved-address" {
		t.Fatal(got, e)
	}
	args := strings.Join(KaspaArgs(root), " ")
	if !strings.Contains(args, "--ram-scale=0.6") || !strings.Contains(args, "--utxoindex") {
		t.Fatal(args)
	}
}
