package main

import (
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"strings"
	"zkas-node-manager/internal/chains"
)

type kaspaSettingsUI struct {
	kMode                      *walk.ComboBox
	kData                      *walk.LineEdit
	kGRPC, kJSON, kBorsh, kP2P *walk.NumberEdit
	kAuto                      *walk.CheckBox
}

func (m *manager) kaspaEndpointText() string {
	c, e := chains.Read(m.root)
	if e != nil {
		return e.Error()
	}
	return fmt.Sprintf("Kaspa mainnet\ngRPC: 127.0.0.1:%d\nJSON wRPC: ws://127.0.0.1:%d\nBorsh wRPC: ws://127.0.0.1:%d\nNode purpose: %s", c.GRPC, c.JSON, c.Borsh, c.NodeMode)
}
func (m *manager) kaspaSettingsTab() TabPage {
	c, _ := chains.Read(m.root)
	idx := 0
	if c.NodeMode == "basic" {
		idx = 1
	} else if c.NodeMode == "archive" {
		idx = 2
	}
	return TabPage{Title: "Kaspa", Layout: VBox{MarginsZero: true}, Children: []Widget{ScrollView{Layout: VBox{Spacing: 8}, Children: []Widget{
		Label{Text: "Kaspa setup and settings", Font: Font{PointSize: 12, Bold: true}}, Label{Text: "Stop Kaspa before changing node settings. Existing data and wallets are retained."},
		Label{Text: "What do you want to use this node for?"}, ComboBox{AssignTo: &m.kMode, Model: []string{"Wallet / application backend (recommended)", "Basic pruned node / mining", "Archive + wallet index (empty data folder required)"}, CurrentIndex: idx},
		TextLabel{Text: "Wallet: pruned blockchain with an address/UTXO index for wallets and bots. Basic: validates and mines without the wallet index. Archive: keeps block data instead of pruning it, with much higher disk use. Kaspa has no shielded-history mode or separate ZKas-style wallet REST daemon.", MinSize: Size{Width: 240}},
		Label{Text: "Blockchain data folder"}, Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &m.kData, Text: chains.DataDir(m.root)}, PushButton{Text: "Browse…", OnClicked: func() {
			d := walk.FileDialog{Title: "Choose Kaspa blockchain folder", FilePath: m.kData.Text()}
			if ok, _ := d.ShowBrowseFolder(m.window); ok {
				m.kData.SetText(d.FilePath)
			}
		}}}},
		Composite{Layout: HBox{}, Children: []Widget{Label{Text: "gRPC port"}, NumberEdit{AssignTo: &m.kGRPC, MinValue: 1024, MaxValue: 65535, Decimals: 0, Value: float64(c.GRPC)}}},
		Composite{Layout: HBox{}, Children: []Widget{Label{Text: "JSON wRPC port"}, NumberEdit{AssignTo: &m.kJSON, MinValue: 1024, MaxValue: 65535, Decimals: 0, Value: float64(c.JSON)}}},
		Composite{Layout: HBox{}, Children: []Widget{Label{Text: "Borsh wRPC port"}, NumberEdit{AssignTo: &m.kBorsh, MinValue: 1024, MaxValue: 65535, Decimals: 0, Value: float64(c.Borsh)}}},
		Composite{Layout: HBox{}, Children: []Widget{Label{Text: "Peer port"}, NumberEdit{AssignTo: &m.kP2P, MinValue: 1024, MaxValue: 65535, Decimals: 0, Value: float64(c.P2P)}}},
		Label{Text: "Cache memory scale (0.3 recommended on an 8 GB PC; 1.0 = upstream default)"}, NumberEdit{AssignTo: &m.kaspaRAM, MinValue: 0.1, MaxValue: 2, Decimals: 1, Value: c.RAMScale},
		Label{Text: "This adjusts caches; it is not a hard memory limit."}, CheckBox{AssignTo: &m.kAuto, Text: "Start Kaspa when I sign in", Checked: c.AutoStart},
		PushButton{AssignTo: &m.kaspaSave, Text: "Save Kaspa settings", OnClicked: m.saveKaspaSettings},
		GroupBox{Title: "Components", Layout: VBox{}, Children: []Widget{
			Label{Text: "Supported node: " + chains.KaspaVersion},
			PushButton{AssignTo: &m.kaspaRepair, Text: "Install / repair Kaspa components", OnClicked: func() {
				m.chainAction("Installing Kaspa components", "Kaspa components installed. Start Kaspa in Overview.", func() error { return installKaspa(m.root, m.setProgress) })
			}},
			PushButton{Text: "View Kaspa release updates", OnClicked: func() { openExternal("https://github.com/kaspanet/rusty-kaspa/releases") }},
			Label{Text: "Install / repair uses the supported version above. Manager updates bring newer supported versions."},
			PushButton{AssignTo: &m.kaspaRemove, Text: "Uninstall Kaspa components…", OnClicked: func() {
				if walk.MsgBox(m.window, "Uninstall Kaspa", "Stop Kaspa, its sharing and dependent mining, and remove Kaspa/merged programs? Both blockchains and all wallet files are retained.", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes {
					m.chainAction("Uninstalling Kaspa", "Kaspa programs removed. Blockchain and wallet data retained.", func() error { return uninstallChains(m.root, false) })
				}
			}},
			PushButton{Text: "Open Kaspa data folder", OnClicked: func() { m.open(chains.DataDir(m.root)) }},
		}},
	}}}}
}
func (m *manager) saveKaspaSettings() {
	c, e := chains.Read(m.root)
	if e != nil {
		m.walletError(e)
		return
	}
	old := c
	c.NodeMode = []string{"wallet", "basic", "archive"}[m.kMode.CurrentIndex()]
	c.DataPath = strings.TrimSpace(m.kData.Text())
	c.GRPC = int(m.kGRPC.Value())
	c.JSON = int(m.kJSON.Value())
	c.Borsh = int(m.kBorsh.Value())
	c.P2P = int(m.kP2P.Value())
	c.RAMScale = m.kaspaRAM.Value()
	c.AutoStart = m.kAuto.Checked()
	m.chainAction("Saving Kaspa settings", "Kaspa settings saved. Start Kaspa in Overview.", func() error {
		if serviceActive(m.root, "kaspa") || serviceActive(m.root, "dual") || serviceActive(kaspaAccessRoot(m.root), "sharing") {
			return fmt.Errorf("Stop Kaspa, Kaspa sharing and dependent mining before changing settings")
		}
		if e := c.ValidateNode(); e != nil {
			return e
		}
		if e := validateKaspaData(m.root, c.DataPath, m.cfg.DataDir); e != nil {
			return e
		}
		if c.NodeMode == "archive" && old.NodeMode != "archive" {
			entries, e := os.ReadDir(c.DataPath)
			if e != nil && !os.IsNotExist(e) {
				return e
			}
			if len(entries) > 0 {
				return fmt.Errorf("Archive mode needs an empty data folder. Choose a new folder; existing data will be retained")
			}
		}
		for _, p := range []int{c.GRPC, c.JSON, c.Borsh, c.P2P} {
			if p == m.cfg.GRPC || p == m.cfg.WebSocket || p == m.cfg.WalletPort || p == 16811 {
				return fmt.Errorf("Kaspa port %d conflicts with ZKas; choose a different port", p)
			}
		}
		if c.AutoStart && !componentPresent(c.Kaspa) {
			return fmt.Errorf("Install Kaspa before enabling automatic startup")
		}
		if e := setKaspaAutoStart(m.root, c.AutoStart); e != nil {
			return e
		}
		if e := chains.Save(m.root, c); e != nil {
			setKaspaAutoStart(m.root, old.AutoStart)
			return e
		}
		m.onUI(func() {
			if m.kwProcess != nil {
				m.kwProcess.stop()
				m.kwProcess = nil
			}
			m.kwApply(kaspaWalletState{})
		})
		return nil
	})
}
func validateKaspaData(root, path, zkas string) error {
	if !filepath.IsAbs(path) || strings.HasPrefix(path, `\\`) {
		return fmt.Errorf("Choose an absolute local data folder")
	}
	overlap := func(a, b string) bool {
		a = strings.ToLower(filepath.Clean(a))
		b = strings.ToLower(filepath.Clean(b))
		return a == b || strings.HasPrefix(a, b+string(os.PathSeparator)) || strings.HasPrefix(b, a+string(os.PathSeparator))
	}
	if overlap(path, zkas) {
		return fmt.Errorf("ZKas and Kaspa need separate data folders")
	}
	for _, v := range []string{"components", "versions", "manager-versions", "service-hosts", "manager-updates", "wallet-runtime", "wallets", "kaspa-wallets", "kaspa-vault", "personal-wallets", "sharing", "kaspa-access"} {
		if overlap(path, filepath.Join(root, v)) {
			return fmt.Errorf("Choose a blockchain folder outside manager program and wallet folders")
		}
	}
	return nil
}
func setKaspaAutoStart(root string, enabled bool) error {
	k, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	if !enabled {
		e = k.DeleteValue("KaspaEasyNode")
		if e == registry.ErrNotExist {
			return nil
		}
		return e
	}
	exe, e := chainHostPath(root)
	if e != nil {
		return e
	}
	return k.SetStringValue("KaspaEasyNode", `"`+exe+`" --kaspa-autostart`)
}
func (m *manager) allWalletTab() TabPage {
	z := m.walletTab()
	z.Title = "ZKas"
	k := m.kaspaWalletTab()
	k.Title = "Kaspa"
	return TabPage{Title: "Wallet", Layout: VBox{MarginsZero: true}, Children: []Widget{TabWidget{AssignTo: &m.walletTabs, Pages: []TabPage{z, k}}}}
}
func (m *manager) allSharingTab() TabPage {
	z := m.sharingTab()
	z.Title = "ZKas"
	return TabPage{Title: "Sharing", Layout: VBox{MarginsZero: true}, Children: []Widget{TabWidget{Pages: []TabPage{z, m.kaspaSharingTab()}}}}
}
