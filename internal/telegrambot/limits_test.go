package telegrambot

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestLimitsIncludeFeesAndShareAcrossWallets(t *testing.T) {
	now := time.Now()
	l := SpendingLimits{Enabled: true, ZKas: ChainLimit{Transaction: "2", Daily: "3"}, Kaspa: ChainLimit{Transaction: "4", Daily: "5"}}
	v := SpendLedger{}
	if _, e := v.Reserve(l, "zkas", 200000000, 1, now); e == nil {
		t.Fatal("fee bypass")
	}
	var e error
	v, e = v.Reserve(l, "zkas", 150000000, 10000000, now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = v.Reserve(l, "zkas", 150000000, 0, now); e == nil {
		t.Fatal("daily aggregate bypass")
	}
	if _, e = v.Reserve(l, "kaspa", 300000000, 0, now); e != nil {
		t.Fatal("chain budgets not independent", e)
	}
	// Simulate restart: JSON round-trip must retain reservations.
	b, _ := json.Marshal(v)
	var reloaded SpendLedger
	if e = json.Unmarshal(b, &reloaded); e != nil {
		t.Fatal(e)
	}
	if _, e = reloaded.Reserve(l, "zkas", 150000000, 0, now); e == nil {
		t.Fatal("restart reset limit")
	}
	if _, e = reloaded.Reserve(l, "zkas", 150000000, 0, now.Add(24*time.Hour)); e != nil {
		t.Fatal("rolling expiry failed", e)
	}
	if _, e = reloaded.Reserve(l, "zkas", 150000000, 0, now.Add(-time.Hour)); e == nil {
		t.Fatal("clock rollback discarded reservation")
	}
}
func TestLimitValidationAndOverflow(t *testing.T) {
	for _, s := range []string{"-1", "0", "oops", "1.000000001"} {
		if e := (SpendingLimits{Enabled: true, ZKas: ChainLimit{Daily: s}}).Validate(); e == nil {
			t.Fatal(s)
		}
	}
	if e := (SpendingLimits{Enabled: true}).Validate(); e == nil {
		t.Fatal("empty enabled limits")
	}
	if e := (SpendingLimits{}).Validate(); e != nil {
		t.Fatal(e)
	}
	l := SpendingLimits{Enabled: true, ZKas: ChainLimit{Daily: "1"}}
	if _, e := (SpendLedger{}).Reserve(l, "zkas", math.MaxUint64, 1, time.Now()); e == nil {
		t.Fatal("overflow")
	}
	if _, e := (SpendLedger{}).Reserve(l, "other", 1, 0, time.Now()); e == nil {
		t.Fatal("unknown chain")
	}
	if _, e := (SpendLedger{}).Reserve(l, "kaspa", 200000000, 0, time.Now()); e != nil {
		t.Fatal("blank chain should be uncapped", e)
	}
}
