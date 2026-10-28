package telegrambot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMiningGraceUnknownRecoveryAndIntentionalStop(t *testing.T) {
	o := AlertOptions{MiningDown: true, MinWorkers: 2, MinHashrate: 10, MiningDelayMinutes: 3}
	now := time.Now()
	var tr MiningTracker
	if a := tr.Observe(MiningSample{}, o, now); len(a) != 0 {
		t.Fatal("stopped before mining ever observed", a)
	}
	s := MiningSample{Running: true, Available: true, Workers: 2, Hashrate: 11}
	tr.Observe(s, o, now)
	s.Workers = 0
	if len(tr.Observe(s, o, now)) != 0 {
		t.Fatal("no grace")
	}
	if len(tr.Observe(s, o, now.Add(3*time.Minute))) != 1 {
		t.Fatal("missing worker alarm")
	}
	if len(tr.Observe(s, o, now.Add(4*time.Minute))) != 0 {
		t.Fatal("repeated alarm")
	}
	s.Workers = 2
	s.Hashrate = 11
	if a := tr.Observe(s, o, now.Add(5*time.Minute)); len(a) != 1 || !strings.Contains(a[0], "recovered") {
		t.Fatal(a)
	}
	s.Available = false
	tr.Observe(s, o, now)
	a := tr.Observe(s, o, now.Add(3*time.Minute))
	if len(a) != 1 || !strings.Contains(a[0], "unknown") {
		t.Fatal(a)
	}
	s = MiningSample{ExpectedStop: true}
	if len(tr.Observe(s, o, now.Add(4*time.Minute))) != 0 {
		t.Fatal("intentional stop alarm")
	}
}
func TestMiningLowHashAndCrash(t *testing.T) {
	o := AlertOptions{MiningDown: true, MinWorkers: 1, MinHashrate: 10, MiningDelayMinutes: 1}
	now := time.Now()
	var tr MiningTracker
	s := MiningSample{Running: true, Available: true, Workers: 1, Hashrate: 1}
	tr.Observe(s, o, now)
	if a := tr.Observe(s, o, now.Add(time.Minute)); len(a) != 1 || !strings.Contains(a[0], "hashrate") {
		t.Fatal(a)
	}
	s.Running = false
	tr.Observe(s, o, now)
	if a := tr.Observe(s, o, now.Add(time.Minute)); len(a) != 1 || !strings.Contains(a[0], "stopped") {
		t.Fatal(a)
	}
}
func TestReceiveBaselineDuplicatesRestartAndHistory(t *testing.T) {
	now := time.Now()
	var s MonitorState
	old := ReceiptEvent{ID: "old", TimestampMS: now.Add(-time.Hour).UnixMilli()}
	if len(s.NewReceipts("wallet", []ReceiptEvent{old}, now)) != 0 {
		t.Fatal("history flood")
	}
	fresh := ReceiptEvent{ID: "new", TimestampMS: now.Add(time.Second).UnixMilli()}
	a := s.NewReceipts("wallet", []ReceiptEvent{old, fresh, fresh}, now.Add(2*time.Second))
	if len(a) != 1 || a[0].ID != "new" {
		t.Fatal(a)
	}
	b, _ := json.Marshal(s)
	var reopened MonitorState
	json.Unmarshal(b, &reopened)
	if len(reopened.NewReceipts("wallet", []ReceiptEvent{fresh}, now.Add(3*time.Second))) != 0 {
		t.Fatal("restart duplicate")
	}
	if len(reopened.NewReceipts("other-wallet", []ReceiptEvent{fresh}, now.Add(3*time.Second))) != 0 {
		t.Fatal("other wallet not baselined")
	}
	missing := ReceiptEvent{ID: "no-date"}
	if len(reopened.NewReceipts("wallet", []ReceiptEvent{missing}, now)) != 0 {
		t.Fatal("unknown timestamp accepted")
	}
}
func TestSummaryLocalScheduleAndCoverage(t *testing.T) {
	loc := time.FixedZone("local", -5*3600)
	now := time.Date(2026, 10, 27, 8, 59, 0, 0, loc)
	s := MonitorState{}
	if s.SummaryDue(now, "09:00") {
		t.Fatal("early")
	}
	s.Summary.Observe(now, 20*time.Second, MiningSample{Running: true, Available: true, Workers: 2, Hashrate: 10}, true, false)
	s.Summary.Observe(now.Add(time.Hour), time.Hour, MiningSample{}, false, false)
	if s.Summary.Observed != 20 {
		t.Fatal("offline gap counted")
	}
	if !s.SummaryDue(now.Add(time.Minute), "09:00") {
		t.Fatal("local schedule missing")
	}
	s.LastSummaryDay = now.Format("2006-01-02")
	if s.SummaryDue(now.Add(2*time.Minute), "09:00") {
		t.Fatal("duplicate same day")
	}
	if !s.SummaryDue(now.Add(24*time.Hour+time.Minute), "09:00") {
		t.Fatal("next day")
	}
	text := s.Summary.Text(now, false, false)
	if !strings.Contains(text, "10.000") || strings.Contains(text, "incoming") {
		t.Fatal(text)
	}
	var late MonitorState
	late.Summary.Observed = 20
	if late.SummaryDue(now.Add(time.Hour), "09:00") {
		t.Fatal("initial late summary")
	}
}
