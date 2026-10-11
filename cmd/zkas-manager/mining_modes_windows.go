package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"os"
	"path/filepath"
	"strings"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

type chainMiningPage struct {
	update                                  *walk.PushButton
	m                                       *manager
	mode                                    string
	worker                                  *walk.LineEdit
	stats                                   *walk.TextEdit
	install, start, stop, remove, dashboard *walk.PushButton
	state, hint                             *walk.Label
	kaspa, zkas                             *walk.LineEdit
	lan                                     *walk.CheckBox
	details                                 *walk.TextEdit
}

func (m *manager) allMiningTab() TabPage {
	z := m.miningTab()
	z.Title = "ZKas"
	return TabPage{Title: "Mining", Layout: VBox{MarginsZero: true}, Children: []Widget{
		Label{AssignTo: &m.miningInventory},
		TextLabel{Text: "One active mining mode at a time: all three use Stratum TCP 5555. Installing one bridge does not replace the others.", MinSize: Size{Width: 240}},
		TabWidget{Pages: []TabPage{z, m.chainMiningTab("kaspa"), m.chainMiningTab("merged")}},
	}}
}
func (m *manager) chainMiningTab(mode string) TabPage {
	u := &chainMiningPage{m: m, mode: mode}
	m.minePages = append(m.minePages, u)
	c, _ := chains.Read(m.root)
	title, note := "Kaspa", "Mine Kaspa with your local Kaspa node. ZKas is not required."
	if mode == "merged" {
		title = "Merged"
		note = "Mine Kaspa and ZKas together. Both local nodes must be synced. The original ZKas bridge is for ZKas-only; merged mining needs its own bridge."
	}
	return TabPage{Title: title, Layout: VBox{MarginsZero: true}, Children: []Widget{ScrollView{Layout: VBox{Spacing: 10}, Children: []Widget{
		Label{Text: title + " mining", Font: Font{PointSize: 12, Bold: true}}, TextLabel{Text: note, MinSize: Size{Width: 240}},
		Label{AssignTo: &u.state, Text: "Checking installed bridge…", Font: Font{Bold: true}},
		GroupBox{Title: "Live mining statistics", Layout: VBox{}, Children: []Widget{TextEdit{AssignTo: &u.stats, ReadOnly: true, VScroll: true, MinSize: Size{Height: 200}, Text: "Mining is stopped."}, TextLabel{Text: "Updates about every 6 seconds while Mining is visible. Workers mean recent activity; TCP sessions are shown separately. Hashrate is estimated from shares. Counters can reset when the bridge restarts; reported blocks are not guaranteed paid rewards.", MinSize: Size{Width: 240}}}},
		PushButton{AssignTo: &u.install, Text: "Install " + title + " bridge", OnClicked: func() {
			m.chainAction("Installing "+title+" bridge", "Bridge installed. Start mining once the required node(s) are synced.", func() error {
				if mode == "merged" {
					return installDual(m.root, m.setProgress)
				}
				return installKaspaBridge(m.root, m.setProgress)
			})
		}},
		PushButton{AssignTo: &u.update, Text: "Check for bridge updates…", OnClicked: func() { m.checkBridgeUpdate(mode) }},
		GroupBox{Title: "Receiving addresses", Layout: VBox{}, Children: []Widget{
			Label{Text: "Kaspa receiving address"}, LineEdit{AssignTo: &u.kaspa, Text: c.KaspaAddress, OnTextChanged: u.detailsChanged},
			Label{Text: "ZKas receiving address", Visible: mode == "merged"}, LineEdit{AssignTo: &u.zkas, Text: c.ZKasAddress, Visible: mode == "merged", OnTextChanged: u.detailsChanged},
			Label{Text: "Worker name (optional)", Visible: mode == "merged"}, LineEdit{AssignTo: &u.worker, Text: c.MergedWorker, Visible: mode == "merged", OnTextChanged: u.detailsChanged},
			TextLabel{Text: "For merged mining, Username is zkas:ADDRESS.workername; Password stays your Kaspa address. Give each miner its own name. Letters, numbers, hyphens and underscores only.", Visible: mode == "merged", MinSize: Size{Width: 240}},
			Label{Text: "Use receiving addresses only. Never enter recovery phrases or private keys."},
		}},
		CheckBox{AssignTo: &u.lan, Text: "Allow miners on my private local network (TCP 5555)", Checked: c.LAN, OnCheckedChanged: u.detailsChanged},
		Label{AssignTo: &u.hint, Text: "Checking requirements…"},
		Composite{Layout: HBox{}, Children: []Widget{
			PushButton{AssignTo: &u.start, Text: "Start " + title + " mining", OnClicked: u.startClicked},
			PushButton{AssignTo: &u.stop, Text: "Stop mining", Visible: false, OnClicked: func() {
				m.chainAction("Stopping mining", "Mining stopped. Both nodes are unchanged.", func() error { return stopOne(m.root, "dual") })
			}},
		}},
		GroupBox{Title: "Miner connection settings", Layout: VBox{}, Children: []Widget{
			TextEdit{AssignTo: &u.details, ReadOnly: true, VScroll: true, MinSize: Size{Height: 105}}, PushButton{Text: "Copy miner settings", OnClicked: func() {
				c, e := u.input()
				if e == nil {
					e = c.ValidateMining()
				}
				if e != nil {
					m.walletError(e)
					return
				}
				u.detailsChanged()
				m.copyText("Miner settings", u.details.Text())
			}},
		}},

		PushButton{AssignTo: &u.dashboard, Text: "Open full mining dashboard", OnClicked: func() { openExternal("http://127.0.0.1:18889") }},
		TextLabel{Text: "The local dashboard reports worker activity, estimated hashrate and found blocks. Found blocks are not guaranteed confirmed rewards.", MinSize: Size{Width: 240}},
		PushButton{Text: "Show mining log", OnClicked: func() { m.mainTabs.SetCurrentIndex(5); m.logSelect.SetCurrentIndex(5); m.latestLog() }},
		PushButton{AssignTo: &u.remove, Text: "Uninstall " + title + " bridge…", OnClicked: func() {
			if walk.MsgBox(m.window, "Uninstall bridge", "Stop this mining bridge and remove its programs? Nodes, wallet files and the other mining bridges are retained.", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes {
				m.chainAction("Removing bridge", "Selected mining bridge removed; nodes and data retained.", func() error { return uninstallSelectedBridge(m.root, mode) })
			}
		}},
	}}}}
}
func (u *chainMiningPage) input() (chains.Config, error) {
	c, e := chains.Read(u.m.root)
	c.Mode = u.mode
	c.KaspaAddress = strings.TrimSpace(u.kaspa.Text())
	c.ZKasAddress = strings.TrimSpace(u.zkas.Text())
	c.LAN = u.lan.Checked()
	if u.mode == "merged" && u.worker != nil {
		c.MergedWorker = strings.TrimSpace(u.worker.Text())
	}
	return c, e
}
func (u *chainMiningPage) detailsChanged() {
	if u.details == nil || u.kaspa == nil || u.zkas == nil || u.lan == nil {
		return
	}
	user, password := u.kaspa.Text(), "x"
	if u.mode == "merged" {
		user, password = u.zkas.Text(), u.kaspa.Text()
		if u.worker != nil && strings.TrimSpace(u.worker.Text()) != "" {
			user += "." + strings.TrimSpace(u.worker.Text())
		}
	}
	urls := node.StratumURL("127.0.0.1")
	if u.lan.Checked() {
		urls = ""
		for _, a := range node.LANAddresses() {
			if a.IP != "127.0.0.1" {
				urls += node.StratumURL(a.IP) + "\r\n"
			}
		}
	}
	v := "Server: " + strings.TrimSpace(urls) + "\r\nUsername: " + user + "\r\nPassword: " + password
	if u.details.Text() != v {
		u.details.SetText(v)
	}
	u.refresh()
}
func (u *chainMiningPage) refresh() {
	if u.state == nil || u.details == nil || u.lan == nil || u.kaspa == nil || u.zkas == nil {
		return
	}
	m := u.m
	c, e := chains.Read(m.root)
	active := serviceActive(m.root, "dual")
	own := active && c.Mode == u.mode
	other := serviceActive(m.root, "mining") || (active && !own)
	c.Mode = u.mode
	installed := e == nil && componentPresent(c.Bridge())
	idle := !m.busy
	u.install.SetVisible(!installed)
	u.install.SetEnabled(idle && !active)
	u.remove.SetVisible(installed)
	u.remove.SetEnabled(idle && !other)
	u.start.SetVisible(installed && !own)
	u.stop.SetVisible(own)
	u.stop.SetEnabled(idle && own)
	u.dashboard.SetEnabled(own)
	u.update.SetEnabled(idle)
	hint := "Ready to start."
	ready := installed && idle && !active && !other && m.kaspaSynced && (u.mode != "merged" || m.zkasSynced)
	if !installed {
		hint = "Install this bridge first."
	} else if other {
		hint = "Another mining mode is running. Stop it in its tab first."
	} else if !m.kaspaSynced {
		hint = "Start Kaspa in Overview and wait for consensus sync."
	} else if u.mode == "merged" && !m.zkasSynced {
		hint = "Start ZKas in Overview and wait for consensus sync."
	}
	in, _ := u.input()
	if ready {
		if e := in.ValidateMining(); e != nil {
			ready = false
			hint = e.Error()
		}
	}
	u.start.SetEnabled(ready)
	state := "Bridge not installed"
	if installed {
		state = "Bridge installed: " + c.Bridge().Version
	}
	if own {
		state = "Mining running • " + c.Bridge().Version + " • TCP 5555"
		hint = "Stop mining before changing its settings."
	}
	u.state.SetText(state)
	u.hint.SetText(hint)
	for _, w := range []walk.Widget{u.kaspa, u.zkas, u.lan, u.worker} {
		w.SetEnabled(idle && !own)
	}
}
func (u *chainMiningPage) startClicked() {
	s, e := u.input()
	if e != nil {
		u.m.walletError(e)
		return
	}
	m := u.m
	c := m.cfg
	m.chainAction("Starting "+u.mode+" mining", "Mining started. Connection details appear in this tab.", func() error { return startDual(m.root, c, s) })
}
func installKaspaBridge(root string, progress func(string)) error {
	if serviceActive(root, "dual") {
		return fmt.Errorf("Stop Kaspa / merged mining before installing its bridge")
	}
	if e := chainPathGuard(root); e != nil {
		return e
	}
	c, e := chains.Read(root)
	if e != nil {
		return e
	}
	b, e := chains.Install(context.Background(), root, false, progress, "stratum-bridge.exe")
	if e != nil {
		return e
	}
	if newer, _ := chains.BridgeNewer(c.KaspaBridge.Version, b["stratum-bridge.exe"].Version); !newer || c.KaspaBridge.Check() != nil {
		c.KaspaBridge = b["stratum-bridge.exe"]
	}
	c.Host, e = chainHostPath(root)
	if e != nil {
		return e
	}
	return chains.Save(root, c)
}
func uninstallSelectedBridge(root, mode string) error {
	c, e := chains.Read(root)
	if e != nil {
		return e
	}
	if serviceActive(root, "dual") {
		if c.Mode != mode {
			return fmt.Errorf("Stop the active mining mode first")
		}
		if e = stopOne(root, "dual"); e != nil {
			return e
		}
	}
	if c.LAN {
		if e = chainFirewall(root, true); e != nil {
			return e
		}
	}
	target := c.KaspaBridge.Path
	if mode == "merged" {
		target = c.DualBridge.Path
	}
	if target == "" {
		return nil
	}
	relative, err := filepath.Rel(filepath.Join(root, "components"), target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("Bridge path is outside managed components")
	}

	if e = os.Remove(target); e != nil && !os.IsNotExist(e) {
		return e
	}
	if mode == "merged" {
		c.DualBridge = chains.Binary{}
	} else {
		c.KaspaBridge = chains.Binary{}
	}
	return chains.Save(root, c)
}
