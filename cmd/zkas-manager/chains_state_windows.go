package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/logformat"
	"zkas-node-manager/internal/node"
)

func (m *manager) chainAction(operation, completion string, work func() error) {
	if m.busy {
		return
	}
	m.chainOperation = operation
	m.chainCompletion = completion
	m.action(work)
}
func componentPresent(b chains.Binary) bool {
	if b.Path == "" {
		return false
	}
	s, e := os.Stat(b.Path)
	return e == nil && !s.IsDir()
}
func (m *manager) updateChainControls() {
	if m.kaspaInstall == nil {
		return
	}
	c, e := chains.Read(m.root)
	installed := e == nil && componentPresent(c.Kaspa)
	active := serviceActive(m.root, "kaspa")
	mining := serviceActive(m.root, "dual")
	idle := !m.busy
	m.kaspaInstall.SetVisible(!installed)
	m.kaspaInstall.SetEnabled(idle && !active)
	m.kaspaStart.SetVisible(installed && !active)
	m.kaspaStart.SetEnabled(idle && installed && !active)
	m.kaspaStop.SetVisible(active)
	m.kaspaStop.SetEnabled(idle && active)
	m.kaspaRepair.SetVisible(true)
	m.kaspaRepair.SetEnabled(idle && !active && !mining)
	m.kaspaRemove.SetVisible(c.Kaspa.Path != "")
	m.kaspaRemove.SetEnabled(idle)
	if m.kaspaRAM != nil {
		m.kaspaRAM.SetEnabled(idle && !active)
		m.kaspaSave.SetEnabled(idle && !active)
	}
	if m.kaspaEndpoints != nil {
		m.kaspaEndpoints.SetText(m.kaspaEndpointText())
	}
	for _, u := range m.minePages {
		u.refresh()
	}
	if m.miningInventory != nil {
		legacy, _ := node.ReadMining(m.root)
		status := func(ok bool) string {
			if ok {
				return "installed"
			}
			return "not installed"
		}
		m.miningInventory.SetText("ZKas bridge: " + status(componentPresent(chains.Binary{Path: legacy.Executable}) || componentPresent(c.DualBridge)) + "  •  Kaspa bridge: " + status(componentPresent(c.KaspaBridge)) + "  •  Merged bridge: " + status(componentPresent(c.DualBridge)))
	}
	m.kwControls()
	m.refreshKaspaSharing()
	if m.kMode != nil {
		active := serviceActive(m.root, "kaspa") || serviceActive(m.root, "dual") || serviceActive(kaspaAccessRoot(m.root), "sharing")
		m.kaspaSave.SetEnabled(!m.busy && !active)
		m.kaspaRAM.SetEnabled(!m.busy && !active)
		for _, w := range []walk.Widget{m.kMode, m.kData, m.kGRPC, m.kJSON, m.kBorsh, m.kP2P, m.kAuto} {
			w.SetEnabled(!m.busy && !active)
		}
	}
}
func lastKaspaSync(root string) string {
	f, e := os.Open(filepath.Join(root, "logs", "kaspa-console.log"))
	if e != nil {
		return "Waiting for the first node log message."
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return ""
	}
	offset := st.Size() - (24 << 10)
	if offset < 0 {
		offset = 0
	}
	f.Seek(offset, 0)
	b, _ := io.ReadAll(io.LimitReader(f, 24<<10))
	lines := strings.Split(logformat.Normalize(string(b)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		lower := strings.ToLower(s)
		if strings.Contains(lower, "ibd") || strings.Contains(lower, "sync") || strings.Contains(lower, "processed") {
			if len(s) > 350 {
				s = s[:350] + "…"
			}
			return s
		}
	}
	return "Waiting for synchronization progress from the node."
}
func (m *manager) refreshChains() {
	if m.kaspaState == nil || m.chainsPolling {
		return
	}
	m.updateChainControls()
	if m.busy {
		return
	}
	m.chainsPolling = true
	m.chainsPolled = time.Now()
	port := m.cfg.GRPC
	go func() {
		c, e := chains.Read(m.root)
		state, detail := "Kaspa is not installed", "Install Kaspa to start downloading and verifying its blockchain."
		ks, zs := false, false
		color := walk.RGB(100, 100, 100)
		if e != nil {
			state = "Configuration error"
			detail = e.Error()
			color = walk.RGB(190, 40, 40)
		} else {
			if c.Kaspa.Path != "" {
				state = "Kaspa stopped"
				detail = "Installed: " + c.Kaspa.Version + " — data is retained. Click Start Kaspa."
				if !componentPresent(c.Kaspa) {
					state = "Kaspa needs repair"
					detail = "The installed executable is missing. Use Repair Kaspa components."
					color = walk.RGB(190, 40, 40)
				}
			}
			if serviceActive(m.root, "kaspa") {
				r, hostErr := serviceCommand(m.root, "kaspa", "status")
				info, rpcErr := m.kaspaMonitor.Get(context.Background(), c.GRPC)
				ks = rpcErr == nil && info.Synced
				state = "Kaspa starting"
				color = walk.RGB(170, 100, 0)
				detail = "Waiting for the node RPC to become ready."
				if hostErr == nil {
					detail = fmt.Sprintf("Process %d • Installed %s", r.PID, c.Kaspa.Version)
					if r.Stopping {
						state = "Kaspa stopping"
					} else if rpcErr == nil {
						if ks {
							state = "Kaspa synced — ready"
							color = walk.RGB(0, 125, 65)
						} else {
							state = "Kaspa syncing — keep it running"
						}
						detail += fmt.Sprintf("\nNode %s • Address index enabled: %t", info.Version, info.Indexed)
						if !ks {
							detail += "\n" + lastKaspaSync(m.root)
						}
					} else {
						detail += "\nRPC is not ready yet."
					}
				}
			} else if c.Kaspa.Path != "" {
				if b, err := os.ReadFile(serviceErrorFile(m.root, "kaspa")); err == nil && len(b) > 0 {
					if len(b) > 500 {
						b = b[:500]
					}
					state = "Kaspa stopped unexpectedly"
					detail = string(b)
					color = walk.RGB(190, 40, 40)
				}
			}
			if serviceActive(m.root, "node") {
				info, err := m.zkasMiningMonitor.Get(context.Background(), port)
				zs = err == nil && info.Synced
			}
		}
		if m.closed.Load() {
			m.kaspaMonitor.Close()
			m.zkasMiningMonitor.Close()
			return
		}
		m.onUI(func() {
			m.chainsPolling = false
			m.kaspaSynced = ks
			m.zkasSynced = zs
			if !m.busy {
				m.kaspaState.SetText(state)
				m.kaspaState.SetTextColor(color)
				m.kaspaDetail.SetText(detail + "\nChecked " + time.Now().Format("15:04:05"))
			}
			m.updateChainControls()
		})
	}()
}
