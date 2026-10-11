package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	"time"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

// Called by the existing background poller, never from a UI callback.
func (m *manager) readMiningDashboard() map[string]string {
	out := map[string]string{}
	mode, port := "", 0
	if serviceActive(m.root, "mining") {
		mode, port = "zkas", 18888
	}
	if serviceActive(m.root, "dual") {
		c, e := chains.Read(m.root)
		if e == nil {
			mode, port = c.Mode, 18889
		}
	}
	if mode == "" {
		return out
	}
	sessions := "Connected TCP sessions: unavailable\r\n"
	if n, e := stratumConnections(); e == nil {
		sessions = fmt.Sprintf("Connected TCP sessions: %d\r\n", n)
	}
	s, e := node.ReadMiningStatsPort(context.Background(), port)
	if e != nil {
		out[mode] = sessions + fmt.Sprintf("Statistics unavailable (checked %s).\r\nThe bridge is running, but its local stats API on port %d did not respond with valid data. Check the mining log or full dashboard. No cached statistics are shown.", time.Now().Format("15:04:05"), port)
		return out
	}
	out[mode] = "Updated " + time.Now().Format("15:04:05") + "\r\n" + sessions + s.Dashboard(mode)
	return out
}
func setMiningStatsText(w *walk.TextEdit, text string) {
	if w != nil && w.Text() != text {
		w.SetText(text)
	}
}
func (m *manager) applyMiningDashboard(texts map[string]string) {
	mode := ""
	if serviceActive(m.root, "mining") {
		mode = "zkas"
	}
	if serviceActive(m.root, "dual") {
		if c, e := chains.Read(m.root); e == nil {
			mode = c.Mode
		}
	}
	show := func(key string, w *walk.TextEdit) {
		text := "Mining is stopped. Start this mode to see live statistics."
		if mode == key {
			text = texts[key]
			if text == "" {
				text = "Mining is running. Waiting for its next statistics update…"
			}
		}
		setMiningStatsText(w, text)
	}
	show("zkas", m.miningStats)
	for _, u := range m.minePages {
		show(u.mode, u.stats)
	}
}
