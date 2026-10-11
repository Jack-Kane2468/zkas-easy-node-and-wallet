package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	"path/filepath"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

func (m *manager) checkBridgeUpdate(mode string) {
	if m.busy {
		return
	}
	m.chainAction("Checking bridge updates", "Bridge update check completed.", func() error {
		c, e := chains.Read(m.root)
		if e != nil {
			return e
		}
		if mode == "zkas" {
			mc, e := node.ReadMining(m.root)
			if e != nil {
				return e
			}
			if !componentPresent(chains.Binary{Path: mc.Executable}) && componentPresent(c.DualBridge) {
				mode = "merged"
			} else {
				return m.updateOriginalBridge(mc)
			}
		}
		dual := mode == "merged"
		old := c.KaspaBridge
		if dual {
			old = c.DualBridge
		}
		r, e := chains.LatestBridge(context.Background(), dual)
		if e != nil {
			return e
		}
		newer, e := chains.BridgeNewer(r.Release.Tag, old.Version)
		if e != nil {
			return e
		}
		if !newer {
			if e := old.Check(); e != nil {
				return fmt.Errorf("The saved bridge is missing or damaged. Use this mining page's install/repair control: %w", e)
			}
			m.onUI(func() {
				walk.MsgBox(m.window, "Bridge is current", fmt.Sprintf("Installed: %s\nLatest stable: %s\n\nNo update needed.", old.Version, r.Release.Tag), walk.MsgBoxIconInformation)
			})
			return nil
		}
		running := serviceActive(m.root, "dual") && ((dual && c.Mode != "kaspa") || (!dual && c.Mode == "kaspa"))
		yes := false
		m.onUI(func() {
			yes = walk.MsgBox(m.window, "Update mining bridge", fmt.Sprintf("Installed: %s\nAvailable: %s\n\nDownload from %s and verify its SHA-256 before stopping mining? If this bridge is running, it will be stopped and restarted with your saved addresses, worker and LAN settings. Nodes and wallets stay running.\n\nIf upgrading for ZKrise, update the ZKas node and wallet backend from Overview too.", old.Version, r.Release.Tag, chains.BridgeRepository(dual)), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes
		})
		if !yes {
			return nil
		}
		binary, e := chains.InstallBridgeRelease(context.Background(), m.root, r, m.setProgress)
		if e != nil {
			return e
		}
		if running {
			m.setProgress("Download verified. Restarting only this mining bridge…")
			if e = stopOne(m.root, "dual"); e != nil {
				return e
			}
		}
		// Read again so the update changes only the selected binary and host.
		next, e := chains.Read(m.root)
		if e != nil {
			return e
		}
		if dual {
			next.DualBridge = binary
		} else {
			next.KaspaBridge = binary
		}
		next.Host, e = chainHostPath(m.root)
		if e != nil {
			return e
		}
		if e = chains.Save(m.root, next); e != nil {
			return e
		}
		if running {
			if e = startDual(m.root, m.cfg, next); e != nil {
				return fmt.Errorf("Bridge %s installed, but mining could not restart: %w. Check Overview and start mining again; data and settings are retained", r.Release.Tag, e)
			}
		}
		m.onUI(func() { m.chainCompletion = "Bridge updated to " + r.Release.Tag + "." })
		return nil
	})
}

func (m *manager) updateOriginalBridge(mc node.MiningConfig) error {
	c, e := node.ReadConfig(m.root)
	if e != nil {
		return e
	}
	if c.Version == "" {
		return fmt.Errorf("Install the ZKas node first")
	}
	latest, e := node.Latest(context.Background())
	if e != nil {
		return e
	}
	if latest.Tag != c.Version {
		return fmt.Errorf("ZKas node %s is installed; latest is %s. Use Overview → Check node updates first (this also updates the enabled wallet backend), then check this bridge again. The original bridge must match the node release", c.Version, latest.Tag)
	}
	if mc.Version == c.Version && node.CheckMiningBinary(mc) == nil {
		m.onUI(func() {
			walk.MsgBox(m.window, "Bridge is current", "Installed bridge: "+mc.Version+"\nThis matches your current ZKas node.", walk.MsgBoxIconInformation)
		})
		return nil
	}
	yes := false
	m.onUI(func() {
		yes = walk.MsgBox(m.window, "Update ZKas-only bridge", fmt.Sprintf("Installed: %s\nAvailable: %s\n\nVerify the download, then briefly restart this mining bridge if it is running? Payout address, worker name, difficulty and node/wallet data are preserved.", mc.Version, c.Version), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes
	})
	if !yes {
		return nil
	}
	exe, _, e := node.Install(context.Background(), filepath.Join(m.root, "mining"), latest, false, m.setProgress, "stratum-bridge.exe")
	if e != nil {
		return e
	}
	next := mc
	next.Executable = filepath.Join(filepath.Dir(exe), "stratum-bridge.exe")
	next.SHA256, e = node.FileHash(next.Executable)
	if e != nil {
		return e
	}
	next.Version = latest.Tag
	next.HostExecutable, e = miningHostPath(m.root)
	if e != nil {
		return e
	}
	running := serviceActive(m.root, "mining")
	if running {
		if e = stopOne(m.root, "mining"); e != nil {
			return e
		}
	}
	if e = node.SaveMining(m.root, next); e != nil {
		return e
	}
	m.onUI(func() { m.mining = next })
	if running {
		if e = allowMiningLAN(next); e != nil {
			return fmt.Errorf("Bridge installed but firewall setup failed; mining remains stopped: %w", e)
		}
		if e = startMining(m.root, c, next); e != nil {
			return fmt.Errorf("Bridge installed but mining could not restart: %w", e)
		}
	}
	m.onUI(func() { m.chainCompletion = "ZKas-only bridge updated to " + next.Version + "." })
	return nil
}
