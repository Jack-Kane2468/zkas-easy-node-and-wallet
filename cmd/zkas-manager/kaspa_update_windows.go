package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	"zkas-node-manager/internal/chains"
)

func (m *manager) checkKaspaNodeUpdate() {
	m.chainAction("Checking Kaspa node updates", "Kaspa node update check completed.", func() error {
		c, e := chains.Read(m.root)
		if e != nil {
			return e
		}
		if !componentPresent(c.Kaspa) {
			return fmt.Errorf("Install Kaspa first")
		}
		r, e := chains.LatestKaspaNode(context.Background())
		if e != nil {
			return e
		}
		newer, e := chains.BridgeNewer(r.Release.Tag, c.Kaspa.Version)
		if e != nil {
			return e
		}
		if !newer {
			if e = c.Kaspa.Check(); e != nil {
				return e
			}
			m.onUI(func() {
				walk.MsgBox(m.window, "Kaspa node is current", fmt.Sprintf("Installed: %s\nLatest stable: %s\n\nNo update needed.", c.Kaspa.Version, r.Release.Tag), walk.MsgBoxIconInformation)
			})
			return nil
		}
		yes := false
		m.onUI(func() {
			yes = walk.MsgBox(m.window, "Update Kaspa node", fmt.Sprintf("Installed: %s\nAvailable: %s\n\nDownload from kaspanet/rusty-kaspa and verify SHA-256 before stopping the node? The database, wallets, ports and sharing settings are retained.\n\nIf Kaspa is running, it will restart. Kaspa or merged mining will be paused; start mining again once the updated node is ready. A stopped node stays stopped. ZKas services and ZKas-only mining stay running.", c.Kaspa.Version, r.Release.Tag), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes
		})
		if !yes {
			return nil
		}
		if e = chainPathGuard(m.root); e != nil {
			return e
		}
		binary, e := chains.InstallKaspaNodeRelease(context.Background(), m.root, r, m.setProgress)
		if e != nil {
			return e
		}
		next, e := chains.Read(m.root)
		if e != nil {
			return e
		}
		next.Host, e = chainHostPath(m.root)
		if e != nil {
			return e
		}
		running := serviceActive(m.root, "kaspa")
		mining := next.Mode != "zkas" && serviceActive(m.root, "dual")
		if mining {
			if e = stopOne(m.root, "dual"); e != nil {
				return e
			}
		}
		if running {
			m.setProgress("Download verified. Restarting Kaspa with the existing database…")
			if e = stopOne(m.root, "kaspa"); e != nil {
				return e
			}
		}
		next.Kaspa = binary
		if e = chains.Save(m.root, next); e != nil {
			return fmt.Errorf("Could not save the new node selection: %w. Start Kaspa again from Overview; mining may be paused", e)
		}
		if running {
			if e = startKaspa(m.root, m.cfg); e != nil {
				return fmt.Errorf("Kaspa %s is installed but could not restart: %w. Data is retained; inspect Logs and start Kaspa from Overview", binary.Version, e)
			}
		}
		done := "Kaspa updated to " + binary.Version + "."
		if running {
			done += " Node restarted; checking sync status."
		} else {
			done += " Node remains stopped."
		}
		if mining {
			done += " Mining paused: start it again when Kaspa is ready."
		}
		m.onUI(func() {
			m.chainCompletion = done
			walk.MsgBox(m.window, "Kaspa node updated", done, walk.MsgBoxIconInformation)
		})
		return nil
	})
}
