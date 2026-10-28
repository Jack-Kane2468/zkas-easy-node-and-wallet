package telegrambot

import (
	"fmt"
	"time"
)

type AlertOptions struct {
	MiningDown, Daily, ReceiveZKas, ReceiveKaspa bool
	MinWorkers, MiningDelayMinutes               int
	MinHashrate                                  float64
	SummaryTime                                  string
	Blocks, Stops, Sync                          bool
	StallMinutes                                 int
}
type NodeSample struct {
	Running, Synced, RPC, ProgressOK, ExpectedStop bool
	Progress, Reason                               string
}
type NodeTracker struct {
	seen, running, synced, stalled, rpcWarn bool
	progress                                string
	changed, failed                         time.Time
}

func (t *NodeTracker) Observe(name string, s NodeSample, now time.Time, o AlertOptions) []string {
	var out []string
	if !s.Running {
		if t.seen && t.running && !s.ExpectedStop && o.Stops {
			reason := s.Reason
			if reason == "" {
				reason = "No reason was recorded; inspect the node log."
			}
			out = append(out, name+" stopped unexpectedly. "+reason)
		}
		*t = NodeTracker{seen: true}
		return out
	}
	fresh := !t.seen || !t.running
	if fresh {
		t.changed = now
		t.progress = s.Progress
		t.synced = s.Synced
	}
	t.seen = true
	t.running = true
	if !s.RPC {
		if t.failed.IsZero() {
			t.failed = now
		}
		if !t.rpcWarn && now.Sub(t.failed) >= 40*time.Second && o.Sync {
			out = append(out, name+" is running, but its RPC has been unavailable for at least 40 seconds. Sync status cannot be checked.")
			t.rpcWarn = true
		}
		return out
	}
	if t.rpcWarn && o.Sync {
		out = append(out, name+" RPC is responding again.")
	}
	t.failed = time.Time{}
	t.rpcWarn = false
	if !fresh && t.synced && !s.Synced && o.Sync {
		out = append(out, name+" no longer reports synced; it may be catching up or disconnected from peers.")
	}
	t.synced = s.Synced
	if s.ProgressOK {
		if s.Progress != t.progress {
			if t.stalled && o.Sync {
				out = append(out, name+" is making chain progress again.")
			}
			t.progress = s.Progress
			t.changed = now
			t.stalled = false
		}
		minutes := o.StallMinutes
		if minutes < 1 {
			minutes = 5
		}
		if !t.stalled && now.Sub(t.changed) >= time.Duration(minutes)*time.Minute && o.Sync {
			out = append(out, fmt.Sprintf("%s has made no observed block/header/DAA progress for %d minutes. It may be stalled, isolated or on an idle chain; inspect its peers and log.", name, minutes))
			t.stalled = true
		}
	} else {
		t.changed = now
	} // Do not infer a stall when progress cannot be read.
	return out
}

type BlockTracker struct {
	DeltaMain, DeltaKas uint64
	session             string
	main, kas, uptime   uint64
	kasKnown            bool
}

func (t *BlockTracker) Observe(session, mode string, main uint64, kas *uint64, uptime uint64) []string {
	reset := session == "" || t.session != session || main < t.main || uptime < t.uptime || (kas != nil && t.kasKnown && *kas < t.kas)
	var out []string
	t.DeltaMain = 0
	t.DeltaKas = 0
	if !reset {
		chain := "ZKas"
		if mode == "kaspa" {
			chain = "Kaspa"
		}
		if main > t.main {
			t.DeltaMain = main - t.main
			out = append(out, fmt.Sprintf("Mining bridge reports %d new %s block(s). This is a bridge report, not confirmation of a payout.", main-t.main, chain))
		}
		if mode == "merged" && kas != nil && t.kasKnown && *kas > t.kas {
			t.DeltaKas = *kas - t.kas
			out = append(out, fmt.Sprintf("Mining bridge reports %d new Kaspa block(s). This is a bridge report, not confirmation of a payout.", *kas-t.kas))
		}
	}
	t.session = session
	t.main = main
	t.uptime = uptime
	t.kasKnown = kas != nil
	if kas != nil {
		t.kas = *kas
	}
	return out
}
