package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"net"
	"os"
	"time"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

func (m *manager) miningTab() TabPage {
	names := []string{}
	for _, a := range m.lanAddresses {
		names = append(names, a.Label)
	}
	return TabPage{Title: "Mining", Layout: VBox{MarginsZero: true}, Children: []Widget{
		ScrollView{Layout: VBox{Spacing: 8}, Children: []Widget{
			Label{Text: "ZKas-only mining · Stratum TCP 5555", Font: Font{PointSize: 12, Bold: true}},
			sharingPanel("MINING SOFTWARE", "Install the connection for your miners", "The mining bridge connects ASIC / Stratum miners to your ZKas node. It is an extra program, installed separately from the node.", walk.RGB(29, 78, 140), walk.RGB(236, 244, 255), []Widget{
				Label{AssignTo: &m.miningStatus, Text: "Checking mining software…", Font: Font{Bold: true}},
				PushButton{AssignTo: &m.miningInstall, Text: "Install mining bridge", OnClicked: m.installMiningClicked},
				PushButton{AssignTo: &m.miningRemove, Text: "Uninstall mining bridge…", OnClicked: m.uninstallMiningClicked},
				TextLabel{Text: "You can install while the node is syncing. Installation downloads the matching bridge; it does not start mining or open a port. Progress appears in the manager's status area.", MinSize: Size{Width: 240}},
			}),
			PushButton{Text: "Open live mining dashboard", OnClicked: func() {
				kc, _ := chains.Read(m.root)
				if serviceActive(m.root, "dual") && kc.Mode == "zkas" {
					openExternal("http://127.0.0.1:18889")
				} else if serviceActive(m.root, "mining") {
					openExternal("http://127.0.0.1:18888")
				} else {
					m.walletError(fmt.Errorf("Start ZKas mining first"))
				}
			}},
			TextEdit{AssignTo: &m.miningStats, ReadOnly: true, VScroll: true, MinSize: Size{Height: 150}, Text: "Waiting for bridge statistics…"},
			TextLabel{MinSize: Size{Width: 240}, Text: "Workers reflect recent share activity (up to 5 minutes), not a live TCP count. Hashrate is a share-based estimate. Counters reset with the bridge; reported blocks are not guaranteed paid rewards."},
			Label{Text: "For Kaspa-compatible kHeavyHash ASICs / Stratum miners."},
			Label{Text: "Choose the PC address on the same network as your miners:"},
			ComboBox{AssignTo: &m.miningLAN, Model: names, CurrentIndex: 0, OnCurrentIndexChanged: func() {
				if m.miningURL != nil {
					m.updateMiningURL()
				}
			}},
			LineEdit{AssignTo: &m.miningURL, ReadOnly: true, Text: node.StratumURL(m.lanAddresses[0].IP)},
			Label{Text: "Payout address (optional here; required in each miner's username)"},
			LineEdit{AssignTo: &m.miningAddress, Text: m.mining.Address, CueBanner: "zkas:…"},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: "Worker"}, LineEdit{AssignTo: &m.miningWorker, Text: m.mining.Worker},
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: "Minimum share difficulty"}, NumberEdit{AssignTo: &m.miningDiff, MinValue: 1, MaxValue: 4294967295, Decimals: 0, Value: float64(m.mining.MinimumDifficulty)},
			}},
			Label{Text: "Automatic difficulty adjustment is enabled. Password: x"},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &m.miningStart, Text: "Start mining (open LAN port 5555)", OnClicked: m.startMiningClicked},
				PushButton{AssignTo: &m.miningStop, Text: "Stop mining", OnClicked: func() {
					m.action(func() error {
						kc, _ := chains.Read(m.root)
						if serviceActive(m.root, "dual") && kc.Mode == "zkas" {
							return stopOne(m.root, "dual")
						}
						return stopOne(m.root, "mining")
					})
				}},
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: "Copy Stratum URL", OnClicked: func() { m.copyText("Stratum URL", m.miningURL.Text()) }},
				PushButton{Text: "Copy miner settings", OnClicked: m.copyMinerSettings},
			}},
			PushButton{Text: "Close LAN port…", OnClicked: func() {
				mc := m.mining
				m.action(func() error {
					if e := stopOne(m.root, "mining"); e != nil {
						return e
					}
					return closeMiningLAN(mc)
				})
			}},
			Label{Text: "Setup asks Windows to allow TCP 5555 on Private / local-subnet only."},
			Label{Text: "If miners cannot connect, check the PC's network is marked Private."},
			Label{Text: "Solo rewards go to the solving miner's address; this is not a pool."},
			Label{Text: "Your node must finish consensus sync. Router ports are unchanged."},
		}},
	}}
}
func (m *manager) updateMiningURL() {
	index := m.miningLAN.CurrentIndex()
	if index >= 0 && index < len(m.lanAddresses) {
		m.miningURL.SetText(node.StratumURL(m.lanAddresses[index].IP))
	}
}
func (m *manager) miningSettings() (node.MiningConfig, error) {
	c := m.mining
	c.Address = m.miningAddress.Text()
	c.Worker = m.miningWorker.Text()
	c.MinimumDifficulty = uint32(m.miningDiff.Value())
	return c, c.Validate()
}
func (m *manager) copyMinerSettings() {
	c, e := m.miningSettings()
	if e != nil {
		walk.MsgBox(m.window, "Check miner settings", e.Error(), walk.MsgBoxIconWarning)
		return
	}
	m.copyText("Miner settings", "Pool URL: "+m.miningURL.Text()+"\r\nWorker / username: "+c.Username()+"\r\nPassword: x")
}
func (m *manager) refreshMining() {
	if m.miningStatus == nil {
		return
	}
	active := serviceActive(m.root, "mining")
	installed := m.mining.Executable != ""
	if info, e := os.Stat(m.mining.Executable); e != nil || info.IsDir() {
		installed = false
	}
	kc, _ := chains.Read(m.root)
	alternative := !installed && componentPresent(kc.DualBridge)
	matching := (installed && m.mining.Version == m.cfg.Version) || alternative
	dualActive := serviceActive(m.root, "dual")
	if dualActive && kc.Mode == "zkas" {
		active = true
	}
	text := "Installed — ready to start after the node finishes syncing"
	button := "Repair / check installation"
	if !installed {
		text = "Not installed — click Install mining bridge below"
		button = "Install mining bridge"
	}
	if installed && !matching {
		text = "Update needed to match your installed node"
		button = "Update mining bridge"
	}
	if m.cfg.Version == "" {
		text = "Install the node from Overview first"
	}
	m.miningInstall.SetText(button)
	m.miningInstall.SetEnabled(!m.busy && !active && m.cfg.Version != "")
	if alternative {
		text = "ZKas-capable merged bridge installed — ready for ZKas-only mining"
		button = "Install original ZKas bridge (optional)"
	}
	if dualActive && kc.Mode != "zkas" {
		text = "Kaspa / merged bridge is using port 5555. Stop it before starting ZKas-only mining."
	}
	if active {
		text = "Mining bridge running · TCP 5555"
	}
	m.miningStatus.SetText(text)
	m.miningStart.SetVisible((installed || alternative) && !active)
	m.miningStart.SetEnabled(!m.busy && !active && !dualActive && matching && m.cfg.Version != "")
	m.miningInstall.SetVisible(!matching)
	m.miningStop.SetVisible(active)
	m.miningRemove.SetVisible(installed)
	m.miningStop.SetEnabled(!m.busy && active)
	m.miningRemove.SetEnabled(!m.busy && (installed || active || m.mining.HostExecutable != ""))
	for _, w := range []walk.Widget{m.miningAddress, m.miningWorker, m.miningDiff} {
		w.SetEnabled(!m.busy && !active)
	}
}
func (m *manager) startMiningClicked() {
	mc, e := m.miningSettings()
	if e != nil {
		walk.MsgBox(m.window, "Check mining setup", e.Error(), walk.MsgBoxIconWarning)
		return
	}
	c := m.cfg
	kc, err := chains.Read(m.root)
	if err != nil {
		m.walletError(err)
		return
	}
	if !componentPresent(chains.Binary{Path: mc.Executable}) && componentPresent(kc.DualBridge) {
		kc.Mode = "zkas"
		kc.ZKasAddress = mc.Address
		kc.LAN = true
		m.chainAction("Starting ZKas mining", "ZKas-only mining started using the installed merged-capable bridge.", func() error { return startDual(m.root, c, kc) })
		return
	}

	m.action(func() error {
		if !serviceActive(m.root, "node") {
			return fmt.Errorf("Start the existing node first from Overview")
		}
		info, e := node.GetInfo(context.Background(), c.GRPC)
		if e != nil {
			return fmt.Errorf("Node RPC is not ready: %w", e)
		}
		if !info.Synced {
			return fmt.Errorf("The node is still syncing. Start mining after consensus sync finishes")
		}
		if e = node.CheckMiningPort(); e != nil {
			return e
		}
		installed, e := installMining(m.root, c, mc, m.setProgress)
		if e != nil {
			return e
		}
		m.onUI(func() { m.mining = installed })
		m.setProgress("Allow the Windows prompt to open TCP 5555 for local miners…")
		if e = allowMiningLAN(installed); e != nil {
			return fmt.Errorf("LAN firewall setup did not complete. Mining has not started: %w", e)
		}
		m.setProgress("Starting the Stratum bridge…")
		if e = startMining(m.root, c, installed); e != nil {
			return e
		}
		// A one-time listener probe; no periodic mining connections are created.
		for i := 0; i < 15; i++ {
			conn, e := net.DialTimeout("tcp", "127.0.0.1:5555", time.Second)
			if e == nil {
				conn.Close()
				return nil
			}
			if !serviceActive(m.root, "mining") {
				return fmt.Errorf("Mining bridge exited; see Mining bridge log")
			}
			time.Sleep(time.Second)
		}
		return fmt.Errorf("Mining process started but TCP 5555 is not listening yet; see Mining bridge log")
	})
}

// Installation is independent of node sync, mining startup and firewall changes.
func (m *manager) installMiningClicked() {
	c, mc := m.cfg, m.mining
	m.action(func() error {
		if c.Version == "" {
			return fmt.Errorf("Install the node from Overview first")
		}
		installed, e := installMining(m.root, c, mc, m.setProgress)
		if e != nil {
			return e
		}
		m.onUI(func() { m.mining = installed })
		m.setProgress("Mining bridge installed. When the node finishes syncing, click Start mining.")
		return nil
	})
}

func (m *manager) uninstallMiningClicked() {
	if walk.MsgBox(m.window, "Uninstall mining bridge", "Stop mining, remove the bridge software and remove its Windows port 5555 firewall rule?\n\nYour node, wallets and sharing services stay running. Saved payout address, worker name and difficulty are kept for reinstalling. Miners will disconnect.", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	m.action(func() error {
		if e := uninstallMining(m.root); e != nil {
			return e
		}
		mc, e := node.ReadMining(m.root)
		if e == nil {
			m.onUI(func() { m.mining = mc })
		}
		return e
	})
}
