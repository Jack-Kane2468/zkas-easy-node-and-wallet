package telegrambot

import (
	"errors"
	"fmt"
	"math"
	"time"
	"zkas-node-manager/internal/wallet"
)

type ChainLimit struct{ Transaction, Daily string }
type SpendingLimits struct {
	Enabled     bool
	ZKas, Kaspa ChainLimit
}
type SpendEntry struct {
	Chain  string
	At     time.Time
	Amount uint64
}
type SpendLedger struct{ Entries []SpendEntry }

func (l SpendingLimits) For(chain string) (ChainLimit, error) {
	switch chain {
	case "zkas":
		return l.ZKas, nil
	case "kaspa":
		return l.Kaspa, nil
	}
	return ChainLimit{}, errors.New("Unknown payment chain")
}
func limitValue(s string) (uint64, error) {
	if s == "" {
		return 0, nil
	}
	n, e := wallet.ParseAmount(s)
	if e != nil || n == 0 {
		return 0, errors.New("Enter a positive limit with at most 8 decimals, or leave blank for no limit")
	}
	return n, nil
}
func (l SpendingLimits) Validate() error {
	if !l.Enabled {
		return nil
	}
	any := false
	for _, c := range []ChainLimit{l.ZKas, l.Kaspa} {
		for _, s := range []string{c.Transaction, c.Daily} {
			n, e := limitValue(s)
			if e != nil {
				return e
			}
			any = any || n > 0
		}
	}
	if !any {
		return errors.New("Enter at least one spending limit, or turn limits off")
	}
	return nil
}

// Reserve conservatively counts confirmed attempts, including ambiguous failures.
// Callers must save the returned ledger durably before signing/submitting.
func (l SpendLedger) Reserve(limits SpendingLimits, chain string, amount, fee uint64, now time.Time) (SpendLedger, error) {
	if !limits.Enabled {
		return l, nil
	}
	if err := limits.Validate(); err != nil {
		return l, err
	}
	if amount > math.MaxUint64-fee {
		return l, errors.New("Payment total overflow")
	}
	total := amount + fee
	c, e := limits.For(chain)
	if e != nil {
		return l, e
	}
	per, e := limitValue(c.Transaction)
	if e != nil {
		return l, e
	}
	daily, e := limitValue(c.Daily)
	if e != nil {
		return l, e
	}
	if per > 0 && total > per {
		return l, fmt.Errorf("Payment plus fee exceeds the %s per-payment limit", chain)
	}
	next := SpendLedger{}
	var used uint64
	for _, v := range l.Entries {
		if v.At.After(now.Add(-24 * time.Hour)) {
			next.Entries = append(next.Entries, v)
			if v.Chain == chain {
				if used > math.MaxUint64-v.Amount {
					return l, errors.New("Spending ledger total overflow")
				}
				used += v.Amount
			}
		}
	}
	if daily > 0 && (used > daily || total > daily-used) {
		return l, fmt.Errorf("Payment plus fee exceeds the remaining %s rolling 24-hour allowance", chain)
	}
	if len(next.Entries) >= 10000 {
		return l, errors.New("Spending ledger is full; wait for earlier reservations to expire")
	}
	next.Entries = append(next.Entries, SpendEntry{Chain: chain, At: now, Amount: total})
	return next, nil
}
