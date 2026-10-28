package telegrambot

import (
	"strings"
	"testing"
	"time"
)

func TestStopsStallsAndRecovery(t *testing.T) {
	o := AlertOptions{Stops: true, Sync: true, StallMinutes: 1}
	now := time.Now()
	var tr NodeTracker
	s := NodeSample{Running: true, RPC: true, Synced: true, ProgressOK: true, Progress: "1"}
	if len(tr.Observe("ZKas", s, now, o)) != 0 {
		t.Fatal("initial alert")
	}
	if a := tr.Observe("ZKas", s, now.Add(time.Minute), o); len(a) != 1 || !strings.Contains(a[0], "no observed") {
		t.Fatal(a)
	}
	if len(tr.Observe("ZKas", s, now.Add(2*time.Minute), o)) != 0 {
		t.Fatal("duplicate stall")
	}
	s.Progress = "2"
	if len(tr.Observe("ZKas", s, now.Add(3*time.Minute), o)) != 1 {
		t.Fatal("missing recovery")
	}
	s.Running = false
	s.Reason = "exit status 3"
	if a := tr.Observe("ZKas", s, now, o); len(a) != 1 || !strings.Contains(a[0], "exit status 3") {
		t.Fatal(a)
	}
	if len(tr.Observe("ZKas", s, now, o)) != 0 {
		t.Fatal("duplicate stop")
	}
	s.Running = true
	tr.Observe("ZKas", s, now, o)
	s.Running = false
	s.ExpectedStop = true
	if len(tr.Observe("ZKas", s, now, o)) != 0 {
		t.Fatal("intentional stop alert")
	}
}
func TestRPCFailureAndLostSync(t *testing.T) {
	var tr NodeTracker
	o := AlertOptions{Sync: true}
	now := time.Now()
	s := NodeSample{Running: true, RPC: true, Synced: true}
	tr.Observe("Kaspa", s, now, o)
	s.Synced = false
	if len(tr.Observe("Kaspa", s, now, o)) != 1 {
		t.Fatal("lost sync")
	}
	s.RPC = false
	if len(tr.Observe("Kaspa", s, now, o)) != 0 {
		t.Fatal("immediate RPC alarm")
	}
	if len(tr.Observe("Kaspa", s, now.Add(41*time.Second), o)) != 1 {
		t.Fatal("missing RPC alarm")
	}
	s.RPC = true
	if len(tr.Observe("Kaspa", s, now.Add(42*time.Second), o)) != 1 {
		t.Fatal("missing RPC recovery")
	}
}
func TestBlockBaselinesAndCounters(t *testing.T) {
	var tr BlockTracker
	kas := uint64(3)
	if len(tr.Observe("pid1", "merged", 10, &kas, 100)) != 0 {
		t.Fatal("historical replay")
	}
	kas = 4
	if len(tr.Observe("pid1", "merged", 11, &kas, 110)) != 2 {
		t.Fatal("missing merged blocks")
	}
	if len(tr.Observe("pid1", "merged", 11, &kas, 120)) != 0 {
		t.Fatal("duplicate")
	}
	if len(tr.Observe("pid2", "merged", 11, &kas, 1)) != 0 {
		t.Fatal("restart replay")
	}
	if len(tr.Observe("pid2", "merged", 0, &kas, 2)) != 0 {
		t.Fatal("reset replay")
	}
}
