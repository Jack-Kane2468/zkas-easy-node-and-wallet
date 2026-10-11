package node

import (
	"strings"
	"testing"
)

func TestMergedDashboard(t *testing.T) {
	// Fields match solo-dual-mode v1.0.11 bridge/src/prom.rs StatsResponse.
	s, e := DecodeMiningStats(strings.NewReader(`{"activeWorkers":1,"totalBlocks":2,"totalKasBlocks":3,"totalShares":45,"bridgeUptime":60,"blocks":[],"kasBlocks":[],"workers":[{"worker":"rig-1","hashrate":125.5,"shares":40,"stale":2,"invalid":1,"blocks":2,"status":"online"}]}`))
	if e != nil {
		t.Fatal(e)
	}
	text := s.Dashboard("merged")
	for _, want := range []string{"ZKas: 2", "Kaspa: 3", "125.500 GH/s", "Accepted shares (bridge): 45", "rig-1", "stale: 2", "invalid: 1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	s.TotalKasBlocks = nil
	if !strings.Contains(s.Dashboard("merged"), "Kaspa: Not reported") {
		t.Fatal("missing Kaspa counter became zero")
	}
	if strings.Contains(s.Dashboard("kaspa"), "ZKas") || !strings.Contains(s.Dashboard("kaspa"), "Kaspa blocks reported: 2") {
		t.Fatal("wrong single-chain label")
	}
}
func TestDashboardBoundsAndUnknown(t *testing.T) {
	s := MiningStats{Workers: make([]MiningWorker, 110)}
	if got := strings.Count(s.Dashboard("zkas"), "Unnamed worker"); got != 100 {
		t.Fatalf("rendered %d workers", got)
	}
	for _, raw := range []string{`null`, `{"activeWorkers":null,"totalBlocks":0,"totalShares":0,"workers":[],"blocks":[]}`, `{"activeWorkers":0,"totalBlocks":0,"totalShares":0,"workers":null,"blocks":[]}`} {
		if _, e := DecodeMiningStats(strings.NewReader(raw)); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
