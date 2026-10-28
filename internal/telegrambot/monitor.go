package telegrambot

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"zkas-node-manager/internal/wallet"
)

func (o AlertOptions) Validate() error {
	if o.MiningDown && (o.MinWorkers < 1 || o.MinWorkers > 100000 || o.MiningDelayMinutes < 1 || o.MiningDelayMinutes > 60 || o.MinHashrate < 0 || math.IsNaN(o.MinHashrate) || math.IsInf(o.MinHashrate, 0)) {
		return errors.New("Mining alerts need at least 1 expected worker, a 1–60 minute delay, and a nonnegative GH/s threshold")
	}
	if o.Daily {
		if _, e := time.Parse("15:04", o.SummaryTime); e != nil {
			return errors.New("Daily summary time must be HH:MM in 24-hour local computer time")
		}
	}
	return nil
}

type MiningSample struct {
	Running, Available, ExpectedStop bool
	Workers                          int
	Hashrate                         float64
}
type MiningTracker struct {
	armed           bool
	reason          string
	since           time.Time
	warned, alerted bool
}

func (t *MiningTracker) Observe(s MiningSample, o AlertOptions, now time.Time) []string {
	if !o.MiningDown {
		*t = MiningTracker{}
		return nil
	}
	if !s.Running && s.ExpectedStop {
		*t = MiningTracker{}
		return nil
	}
	if s.Running {
		t.armed = true
	}
	if !t.armed {
		return nil
	}
	reason := ""
	switch {
	case !s.Running:
		reason = "Mining bridge stopped unexpectedly."
	case !s.Available:
		reason = "Mining statistics API is unavailable; worker activity and hashrate are unknown."
	case s.Workers < o.MinWorkers:
		reason = fmt.Sprintf("Recently active workers below the configured minimum of %d.", o.MinWorkers)
	case o.MinHashrate > 0 && s.Hashrate < o.MinHashrate:
		reason = fmt.Sprintf("Estimated hashrate below the configured minimum of %.3f GH/s.", o.MinHashrate)
	}
	if reason == "" {
		had := t.alerted
		t.reason = ""
		t.warned = false
		t.alerted = false
		t.since = time.Time{}
		if had {
			return []string{fmt.Sprintf("Mining recovered: %d recently active workers, %.3f GH/s estimated.", s.Workers, s.Hashrate)}
		}
		return nil
	}
	if reason != t.reason {
		t.reason = reason
		t.since = now
		t.warned = false
	}
	delay := o.MiningDelayMinutes
	if delay < 1 {
		delay = 3
	}
	if !t.warned && now.Sub(t.since) >= time.Duration(delay)*time.Minute {
		t.warned = true
		t.alerted = true
		return []string{fmt.Sprintf("Mining alert (persisted %d minutes): %s", delay, reason)}
	}
	return nil
}

type ReceiptEvent struct {
	ID, Chain, Wallet, Amount, Address, Memo, Status string
	TimestampMS                                      int64
}
type ReceiveCursor struct {
	StartMS int64
	Seen    map[string]int64
	Touched time.Time
}
type MonitorState struct {
	Receives       map[string]ReceiveCursor
	Summary        SummaryStats
	LastSummaryDay string
}
type SummaryStats struct {
	ZKasStatus, KaspaStatus                                        string
	Started                                                        time.Time
	Observed, MiningObserved, MiningUp, HashSeconds, WorkerSeconds float64
	ZKasUp, KaspaUp                                                float64
	ZKasBlocks, KaspaBlocks, ZKasReceives, KaspaReceives           uint64
}

// Baseline once and require a timestamp after it; never replay pre-existing history.
func (s *MonitorState) NewReceipts(key string, events []ReceiptEvent, now time.Time) []ReceiptEvent {
	if s.Receives == nil {
		s.Receives = map[string]ReceiveCursor{}
	}
	c, ok := s.Receives[key]
	if !ok {
		if len(s.Receives) >= 256 {
			oldKey := ""
			old := now
			for k, v := range s.Receives {
				if oldKey == "" || v.Touched.Before(old) {
					oldKey = k
					old = v.Touched
				}
			}
			delete(s.Receives, oldKey)
		}
		c = ReceiveCursor{StartMS: now.UnixMilli(), Seen: map[string]int64{}}
	}
	if c.Seen == nil {
		c.Seen = map[string]int64{}
	}
	cutoff := now.Add(-7 * 24 * time.Hour).UnixMilli()
	for id, ms := range c.Seen {
		if ms < cutoff {
			delete(c.Seen, id)
		}
	}
	var out []ReceiptEvent
	for _, e := range events {
		if e.ID == "" || e.TimestampMS <= 0 {
			continue
		}
		_, seen := c.Seen[e.ID]
		if ok && !seen && e.TimestampMS >= c.StartMS && e.TimestampMS >= cutoff && e.TimestampMS <= now.Add(5*time.Minute).UnixMilli() {
			out = append(out, e)
		}
		c.Seen[e.ID] = e.TimestampMS
	}
	if len(c.Seen) > 2048 { // Keep the most recent IDs; timestamp cutoff still suppresses old records.
		floor := int64(0)
		for len(c.Seen) > 2048 {
			var oldest string
			floor = math.MaxInt64
			for id, ms := range c.Seen {
				if ms < floor {
					floor = ms
					oldest = id
				}
			}
			delete(c.Seen, oldest)
		}
		if floor >= c.StartMS {
			c.StartMS = floor + 1
		}
	}
	c.Touched = now
	s.Receives[key] = c
	return out
}
func (e ReceiptEvent) Text() string {
	out := fmt.Sprintf("Received %s %s\nWallet: %s\nStatus: %s\nTX: %s", e.Amount, strings.ToUpper(e.Chain), e.Wallet, e.Status, e.ID)
	if e.Address != "" {
		out += "\nAddress: " + e.Address
	}
	if e.Memo != "" {
		out += "\nMemo: " + e.Memo
	}
	return out
}
func (s *SummaryStats) Observe(now time.Time, elapsed time.Duration, mining MiningSample, zkas, kaspa bool) {
	if s.Started.IsZero() {
		s.Started = now
	}
	seconds := elapsed.Seconds()
	if seconds <= 0 || seconds > 90 {
		return
	}
	s.Observed += seconds
	if zkas {
		s.ZKasUp += seconds
	}
	if kaspa {
		s.KaspaUp += seconds
	}
	if mining.Running && mining.Available {
		s.MiningObserved += seconds
		if mining.Workers > 0 {
			s.MiningUp += seconds
		}
		s.HashSeconds += mining.Hashrate * seconds
		s.WorkerSeconds += float64(mining.Workers) * seconds
	}
}
func (s SummaryStats) Text(now time.Time, includeZKas, includeKaspa bool) string {
	pct := func(n, d float64) float64 {
		if d == 0 {
			return 0
		}
		return n / d * 100
	}
	mining := "Mining: no usable statistics samples."
	if s.MiningObserved > 0 {
		mining = fmt.Sprintf("Mining observed: %s\nAverage estimated hashrate: %.3f GH/s\nAverage recently active workers: %.1f\nWorker activity in usable samples: %.1f%%", (time.Duration(s.MiningObserved) * time.Second).Round(time.Minute), s.HashSeconds/s.MiningObserved, s.WorkerSeconds/s.MiningObserved, pct(s.MiningUp, s.MiningObserved))
	}
	out := fmt.Sprintf("Daily manager summary\n%s to %s\nObserved coverage: %s (gaps excluded)\nZKas running: %.1f%% | Kaspa running: %.1f%% of observed time\n%s\nNew bridge-reported blocks observed: ZKas %d | Kaspa %d\nBlock reports are not payout confirmations.", s.Started.Local().Format("Jan 2 15:04 MST"), now.Local().Format("Jan 2 15:04 MST"), (time.Duration(s.Observed) * time.Second).Round(time.Minute), pct(s.ZKasUp, s.Observed), pct(s.KaspaUp, s.Observed), mining, s.ZKasBlocks, s.KaspaBlocks)
	out += "\nCurrent ZKas: " + s.ZKasStatus + " | Kaspa: " + s.KaspaStatus
	if includeZKas {
		out += fmt.Sprintf("\nNew incoming ZKas transactions observed: %d", s.ZKasReceives)
	}
	if includeKaspa {
		out += fmt.Sprintf("\nNew incoming Kaspa transactions observed: %d", s.KaspaReceives)
	}
	return out
}
func (s *MonitorState) SummaryDue(now time.Time, at string) bool {
	day := now.Format("2006-01-02")
	clock, e := time.Parse("15:04", at)
	if e != nil {
		return false
	}
	due := time.Date(now.Year(), now.Month(), now.Day(), clock.Hour(), clock.Minute(), 0, 0, now.Location())
	if s.LastSummaryDay == "" {
		s.LastSummaryDay = day
		if now.Before(due) {
			s.LastSummaryDay = now.AddDate(0, 0, -1).Format("2006-01-02")
		}
		return false
	}
	return s.LastSummaryDay < day && !now.Before(due) && s.Summary.Observed > 0
}

// Normalize backend timestamps without inventing a confirmation count.
func TimestampMS(n uint64) int64 {
	if n > math.MaxInt64 {
		return 0
	}
	if n < 1_000_000_000_000 {
		return int64(n) * 1000
	}
	return int64(n)
}
func ReceiveAmount(s string) string {
	n, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		return "unknown amount"
	}
	return wallet.FormatAmount(n)
}

// Bound notification traffic during high-volume mining or wallet catch-up.
// Every new receipt is counted; at most three transaction details per wallet
// are included in one notification, with history as the complete record.
func ReceiptMessages(events []ReceiptEvent) []string {
	groups := map[string][]ReceiptEvent{}
	order := []string{}
	for _, e := range events {
		key := e.Chain + ":" + e.Wallet
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], e)
	}
	trim := func(s string, n int) string {
		r := []rune(s)
		if len(r) > n {
			return string(r[:n]) + "…"
		}
		return s
	}
	out := []string{}
	for _, key := range order {
		rows := groups[key]
		text := fmt.Sprintf("%d new incoming transaction(s) — %s\nWallet: %s", len(rows), strings.ToUpper(rows[0].Chain), trim(rows[0].Wallet, 100))
		for i, e := range rows {
			if i == 3 {
				break
			}
			text += fmt.Sprintf("\n\nReceived %s %s\nTX: %s\n%s", e.Amount, strings.ToUpper(e.Chain), trim(e.ID, 100), trim(e.Status, 140))
			if e.Address != "" {
				text += "\nAddress: " + trim(e.Address, 200)
			}
			if e.Memo != "" {
				text += "\nMemo: " + trim(e.Memo, 300)
			}
		}
		if len(rows) > 3 {
			text += fmt.Sprintf("\n\n%d more received transactions. Open wallet history for the complete list.", len(rows)-3)
		}
		out = append(out, text)
	}
	return out
}
