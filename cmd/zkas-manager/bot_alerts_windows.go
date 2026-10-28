package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
	"zkas-node-manager/internal/telegrambot"
)

func stopIntent(root, kind string) string { return filepath.Join(root, kind+"-expected-stop.txt") }
func expectedStop(root, kind string) bool {
	st, err := os.Stat(stopIntent(root, kind))
	return err == nil && time.Since(st.ModTime()) < 5*time.Minute
}

var exitStatusPattern = regexp.MustCompile(`exit status [0-9]+`)

func stopReason(root, kind string, since time.Time) string {
	reason := ""
	p := serviceErrorFile(root, kind)
	if st, e := os.Stat(p); e == nil && !st.ModTime().Before(since) {
		if b, e := os.ReadFile(p); e == nil {
			reason = exitStatusPattern.FindString(string(b))
		}
	}
	name := "console.log"
	if kind != "node" {
		name = kind + "-console.log"
	}
	p = filepath.Join(root, "logs", name)
	if st, e := os.Stat(p); e == nil && !st.ModTime().Before(since) {
		if f, e := os.Open(p); e == nil {
			defer f.Close()
			if st.Size() > 8192 {
				f.Seek(-8192, io.SeekEnd)
			}
			b, _ := io.ReadAll(io.LimitReader(f, 8192))
			s := strings.ToLower(string(b))
			for _, x := range []struct{ match, label string }{{"memory allocation", "The recent log reports a memory allocation failure."}, {"out of memory", "The recent log reports an out-of-memory failure."}, {"no space left", "The recent log reports insufficient disk space."}, {"panicked at", "The recent log reports a process panic."}, {"address already in use", "The recent log reports a port conflict."}, {"access is denied", "The recent log reports access denied."}} {
				if strings.Contains(s, x.match) {
					return strings.TrimSpace(reason + " " + x.label)
				}
			}
		}
	}
	return reason
}
func (m *manager) botTouch(ctx context.Context, id int64) {
	m.botUIRun(ctx, func() {
		for _, v := range m.botConfig.Contacts {
			if v == id {
				return
			}
		}
		if len(m.botConfig.Contacts) >= 128 {
			return
		}
		m.botConfig.Contacts = append(m.botConfig.Contacts, id)
		if err := telegrambot.Save(m.root, m.botConfig); err != nil {
			m.botState.SetText("Could not save alert recipient; alerts may not resume after restart.")
		}
	})
}
func (m *manager) botMonitorStatus(ctx context.Context, s string) {
	m.botUIRun(ctx, func() { m.botMonitorState.SetText(s) })
}
func (m *manager) botAlerts(ctx context.Context, api *telegrambot.API, c telegrambot.Config) {
	m.botMonitorMu.Lock()
	defer m.botMonitorMu.Unlock()
	if ctx.Err() != nil {
		return
	}
	o := c.Alerts
	if !o.Blocks && !o.Stops && !o.Sync && !o.MiningDown && !o.Daily && !o.ReceiveZKas && !o.ReceiveKaspa {
		m.botMonitorStatus(ctx, "Optional notifications: off.")
		return
	}
	state, err := telegrambot.LoadMonitor(m.root)
	if err != nil {
		m.botMonitorStatus(ctx, "Notifications stopped: saved monitoring state could not be read. Bot commands remain available.")
		return
	}
	if !o.Daily {
		state.Summary = telegrambot.SummaryStats{}
		state.LastSummaryDay = ""
	}
	trackers := map[string]*telegrambot.NodeTracker{"node": {}, "kaspa": {}}
	monitors := map[string]*node.InfoMonitor{"node": {}, "kaspa": {}}
	defer monitors["node"].Close()
	defer monitors["kaspa"].Close()
	lastRunning := map[string]time.Time{}
	var blocks telegrambot.BlockTracker
	var miningTracker telegrambot.MiningTracker
	var lastTick, lastReceive, lastSave time.Time
	rotation := 0
	lastBridge := ""
	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()
	m.botMonitorStatus(ctx, "Notifications active. Receive history is baselined before new-payment alerts; daily summaries use this computer's local time.")
	for ctx.Err() == nil {
		now := time.Now()
		events := []string{}
		durable := []string{}
		cfg, ce := node.ReadConfig(m.root)
		kc, ke := chains.Read(m.root)
		running := map[string]bool{}
		for _, x := range []struct {
			kind, name string
			port       int
			valid      bool
		}{{"node", "ZKas", cfg.GRPC, ce == nil}, {"kaspa", "Kaspa", kc.GRPC, ke == nil}} {
			if !x.valid {
				continue
			}
			s := telegrambot.NodeSample{Running: serviceActive(m.root, x.kind), ExpectedStop: expectedStop(m.root, x.kind)}
			running[x.kind] = s.Running
			if o.Stops || o.Sync || o.Daily {
				if s.Running {
					info, e := monitors[x.kind].Get(ctx, x.port)
					s.RPC = e == nil
					s.Synced = info.Synced
					if e == nil {
						p, e := monitors[x.kind].Progress(ctx)
						s.ProgressOK = e == nil
						s.Progress = fmt.Sprint(p)
					}
					lastRunning[x.kind] = now.Add(-2 * time.Second)
				} else {
					monitors[x.kind].Close()
					s.Reason = stopReason(m.root, x.kind, lastRunning[x.kind])
				}
				events = append(events, trackers[x.kind].Observe(x.name, s, now, o)...)
				status := "stopped"
				if s.Running {
					status = "running, RPC unavailable"
					if s.RPC {
						status = "syncing"
						if s.Synced {
							status = "synced"
						}
					}
				}
				if x.kind == "node" {
					state.Summary.ZKasStatus = status
				} else {
					state.Summary.KaspaStatus = status
				}
			}
		}
		mining := telegrambot.MiningSample{}
		kind, mode, port := "", "", 0
		if o.Blocks || o.MiningDown || o.Daily {
			if serviceActive(m.root, "mining") {
				kind = "mining"
				mode = "zkas"
				port = 18888
			}
			if serviceActive(m.root, "dual") && ke == nil {
				kind = "dual"
				mode = kc.Mode
				port = 18889
			}
			mining.Running = kind != ""
			if lastBridge != "" {
				mining.ExpectedStop = expectedStop(m.root, lastBridge)
			}
			if kind == "" {
				blocks = telegrambot.BlockTracker{}
			} else {
				lastBridge = kind
				if stats, e := node.ReadMiningStatsPort(ctx, port); e == nil {
					mining.Available = true
					mining.Workers = stats.ActiveWorkers
					for _, w := range stats.Workers {
						mining.Hashrate += w.Hashrate
					}
					host, e := serviceCommand(m.root, kind, "status")
					if e == nil {
						be := blocks.Observe(fmt.Sprintf("%s:%s:%d", kind, mode, host.PID), mode, stats.TotalBlocks, stats.TotalKasBlocks, stats.BridgeUptime)
						if o.Blocks {
							events = append(events, be...)
						}
						if o.Daily {
							if mode == "kaspa" {
								state.Summary.KaspaBlocks += blocks.DeltaMain
							} else {
								state.Summary.ZKasBlocks += blocks.DeltaMain
								state.Summary.KaspaBlocks += blocks.DeltaKas
							}
						}
					}
				}
			}
			events = append(events, miningTracker.Observe(mining, o, now)...)
		}
		if o.Daily {
			state.Summary.Observe(now, now.Sub(lastTick), mining, running["node"], running["kaspa"])
		}
		lastTick = now
		if (o.ReceiveZKas || o.ReceiveKaspa) && now.Sub(lastReceive) >= time.Minute {
			received := m.botReceiveEvents(ctx, &state, c, &rotation)
			durable = append(durable, telegrambot.ReceiptMessages(received)...)
			for _, e := range received {
				if o.Daily {
					if e.Chain == "zkas" {
						state.Summary.ZKasReceives++
					} else {
						state.Summary.KaspaReceives++
					}
				}
			}
			lastReceive = now
		}
		if o.Daily && state.SummaryDue(now, o.SummaryTime) {
			durable = append(durable, state.Summary.Text(now, o.ReceiveZKas && (c.Permissions.Wallets || c.Permissions.FVK), o.ReceiveKaspa && c.Permissions.KaspaWallets))
			state.LastSummaryDay = now.Format("2006-01-02")
			state.Summary = telegrambot.SummaryStats{Started: now}
		}
		if len(durable) > 0 || now.Sub(lastSave) >= time.Minute {
			if err := telegrambot.SaveMonitor(m.root, state); err != nil {
				m.botMonitorStatus(ctx, "Notification monitoring stopped: its state could not be saved. Check disk space/permissions, then restart the bot. Commands remain available.")
				return
			}
			lastSave = now
		}
		events = append(events, durable...)
		recipients := []int64{}
		m.botUIRun(ctx, func() {
			seen := map[int64]bool{}
			for _, id := range append([]int64{m.botConfig.OwnerID}, m.botConfig.Contacts...) {
				if !seen[id] && m.botConfig.Allows(id) {
					recipients = append(recipients, id)
					seen[id] = true
				}
			}
		})
		for _, event := range events {
			for _, id := range recipients {
				q, cancel := context.WithTimeout(ctx, 10*time.Second)
				err := api.Send(q, id, telegrambot.Reply{Text: event}, nil)
				cancel()
				if err != nil {
					m.botMonitorStatus(ctx, "A notification could not be delivered. Check internet access and open a private chat with the bot. Failed deliveries are not replayed.")
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
