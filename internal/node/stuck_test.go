package node

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func addStuckFixture(t *testing.T, root, token, cursor string) {
	t.Helper()
	os.MkdirAll(filepath.Join(root, "wallets"), 0700)
	os.WriteFile(filepath.Join(root, "wallets", token+".json"), []byte(`{"seed_hex":"","network":"mainnet"}`), 0600)
	b := make([]byte, 77)
	copy(b, "FCWS")
	b[4] = 4
	g, _ := hex.DecodeString(MainnetGenesis)
	copy(b[5:37], g)
	c, _ := hex.DecodeString(cursor)
	copy(b[37:69], c)
	os.WriteFile(filepath.Join(root, "wallets", token+".scan"), b, 0600)
}
func TestLogWalletMatchingRejectsAmbiguousAndStale(t *testing.T) {
	root := t.TempDir()
	cursor := strings.Repeat("1", 64)
	addStuckFixture(t, root, "bot", cursor)
	if token, e := UniqueCheckpointOwner(root, cursor); e != nil || token != "bot" {
		t.Fatal(token, e)
	}
	addStuckFixture(t, root, "another", cursor)
	if _, e := UniqueCheckpointOwner(root, cursor); e == nil {
		t.Fatal("ambiguous checkpoint selected a wallet")
	}
	os.Remove(filepath.Join(root, "wallets", "bot.scan"))
	os.Remove(filepath.Join(root, "wallets", "another.scan"))
	if _, e := UniqueCheckpointOwner(root, cursor); e == nil {
		t.Fatal("stale checkpoint accepted")
	}
}
func TestRecentLoopsRequireDistinctRecentFailures(t *testing.T) {
	now := time.Date(2026, 10, 17, 21, 0, 0, 0, time.UTC)
	c := strings.Repeat("a", 64)
	line := func(tm time.Time) string {
		return tm.Format("2006-01-02 15:04:05.000-07:00") + " [WARN ] cannot verify checkpoint cursor against selected chain (" + c + " lies in peer-supplied history; this node serves no frontier there (scan from genesis)); keeping checkpoint and retrying\n"
	}
	if len(RecentLoopCursors(line(now), now)) != 0 {
		t.Fatal("single failure classified as loop")
	}
	if len(RecentLoopCursors(line(now)+line(now), now)) != 0 {
		t.Fatal("duplicate event classified as loop")
	}
	if !RecentLoopCursors(line(now)+line(now.Add(-time.Minute)), now)[c] {
		t.Fatal("recent loop missed")
	}
	if len(RecentLoopCursors(line(now.Add(-time.Hour))+line(now.Add(-2*time.Hour)), now)) != 0 {
		t.Fatal("old log classified as current loop")
	}
}
