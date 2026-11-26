package node

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWalletOptionsLegacyAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	c := Defaults(root)
	before := strings.Join(c.WalletArgs(root), " ")
	if strings.Contains(before, "--compact-nullifiers") || strings.Contains(before, "--shared-leaf-store") {
		t.Fatal("legacy behavior changed")
	}
	c.WalletDaemon = WalletOptions{CompactNullifiers: true, SharedLeafStore: true, HardMaxResident: 8, ResidentBudgetMB: 512, SyncWallets: 2, LoadWallets: 1, ProofThreads: 2, ConcurrentProves: 1, IdleEvict: 600, CheckpointInterval: 120, CatchupCheckpointInterval: 300, PrefetchDepth: 2}
	if e := Save(root, c); e != nil {
		t.Fatal(e)
	}
	got, e := ReadConfig(root)
	if e != nil {
		t.Fatal(e)
	}
	if got.WalletDaemon != c.WalletDaemon {
		t.Fatal("options lost across restart")
	}
	args := strings.Join(got.WalletArgs(root), " ")
	for _, arg := range c.WalletDaemon.Args() {
		if !strings.Contains(args, arg) {
			t.Fatal("missing", arg)
		}
	}
	if !strings.Contains(args, "--no-custodial") || !strings.Contains(args, "--listen=127.0.0.1:") {
		t.Fatal("existing security defaults changed")
	}
	var legacy Config
	if e = json.Unmarshal([]byte(`{"residentWallets":5}`), &legacy); e != nil {
		t.Fatal(e)
	}
	if len(legacy.WalletDaemon.Args()) != 0 {
		t.Fatal("old config opts in")
	}
}
func TestWalletOptionsCompatibilityAndBounds(t *testing.T) {
	o := WalletOptions{CompactNullifiers: true, SharedLeafStore: true, ProofThreads: 2}
	if e := o.CheckHelp("--compact-nullifiers --shared-leaf-store --proof-threads <N>"); e != nil {
		t.Fatal(e)
	}
	if o.CheckHelp("--compact-nullifiers --shared-leaf-store-extra --proof-threads") == nil {
		t.Fatal("accepted unsupported flag")
	}
	for _, o := range []WalletOptions{{ProofThreads: -1}, {SyncWallets: 1025}, {CheckpointInterval: 86401}, {PrefetchDepth: 65}} {
		if o.Validate() == nil {
			t.Fatal("accepted invalid option", o)
		}
	}
}
func TestWalletStorageReportDoesNotModifyFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "wallets")
	os.Mkdir(dir, 0700)
	for _, name := range []string{"x.scan", "x.scan.bak", "x.scan.bak.100", "x.scan.stale-1", "x.scan.divergent-2", "__nullifiers__.nfs", "x.json"} {
		if e := os.WriteFile(filepath.Join(dir, name), []byte("keep"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	report, e := WalletStorageReport(root)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(report, "x.scan.bak.100") || !strings.Contains(report, "Files: 7") {
		t.Fatal(report)
	}
	for _, name := range []string{"x.scan", "__nullifiers__.nfs", "x.json", "x.scan.tmp"} {
		if isRetiredCheckpoint(name) {
			t.Fatal("classified active file as retired", name)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 7 {
		t.Fatal("files changed")
	}
	for _, entry := range entries {
		b, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
		if string(b) != "keep" {
			t.Fatal("content changed")
		}
	}
}
