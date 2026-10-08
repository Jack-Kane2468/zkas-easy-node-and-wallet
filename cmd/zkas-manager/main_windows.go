package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

func main() {
	runtime.LockOSThread()
	root := rootDir()
	os.MkdirAll(root, 0700)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--manager-self-test":
			if managerSelfTest(root) != nil {
				os.Exit(1)
			}
			return
		case "--activate-manager-update":
			if len(os.Args) != 3 {
				return
			}
			if err := activateManagerUpdate(root, os.Args[2]); err != nil {
				walk.MsgBox(nil, "Manager update", err.Error(), walk.MsgBoxIconError)
			}
			return
		case "--sharing-host":
			os.Remove(serviceErrorFile(root, "sharing"))
			if e := runSharingHost(root); e != nil {
				os.WriteFile(serviceErrorFile(root, "sharing"), []byte(e.Error()), 0600)
				os.Exit(1)
			}
			return
		case "--remove-sharing-firewall":
			if runPS("$ErrorActionPreference='Stop'; Get-NetFirewallRule -Name '"+peerRule+"','"+apiRule+"' -ErrorAction SilentlyContinue | Remove-NetFirewallRule") != nil {
				os.Exit(1)
			}
			return
		case "--peer-firewall":
			if peerFirewallHelper() != nil {
				os.Exit(1)
			}
			return
		case "--sharing-firewall":
			if sharingFirewallHelper() != nil {
				os.Exit(1)
			}
			return
		case "--kaspa-autostart":
			c, e := chains.Read(root)
			if e == nil && c.AutoStart && !serviceActive(root, "kaspa") {
				zc, _ := node.ReadConfig(root)
				if e = startKaspa(root, zc); e != nil {
					os.WriteFile(serviceErrorFile(root, "kaspa"), []byte(e.Error()), 0600)
				}
			}
			return
		case "--kaspa-sharing-host":
			if e := runKaspaSharingHost(root); e != nil {
				os.WriteFile(serviceErrorFile(kaspaAccessRoot(root), "sharing"), []byte(e.Error()), 0600)
				os.Exit(1)
			}
			return
		case "--kaspa-firewall":
			if kaspaFirewallHelper(false) != nil {
				os.Exit(1)
			}
			return
		case "--remove-kaspa-firewall":
			if kaspaFirewallHelper(true) != nil {
				os.Exit(1)
			}
			return
		case "--kaspa-host", "--dual-host":
			kind := strings.TrimSuffix(strings.TrimPrefix(os.Args[1], "--"), "-host")
			if runHost(root, kind) != nil {
				os.Exit(1)
			}
			return
		case "--allow-chain-lan", "--remove-chain-lan":
			if chainFirewallHelper(root, os.Args[1] == "--remove-chain-lan") != nil {
				os.Exit(1)
			}
			return
		case "--host":
			if runHost(root, "node") != nil {
				os.Exit(1)
			}
			return
		case "--mining-host":
			if runHost(root, "mining") != nil {
				os.Exit(1)
			}
			return
		case "--remove-mining-lan":
			if removeMiningFirewallHelper() != nil {
				os.Exit(1)
			}
			return
		case "--allow-mining-lan":
			if firewallHelper() != nil {
				os.Exit(1)
			}
			return
		case "--wallet-host":
			if runHost(root, "wallet") != nil {
				os.Exit(1)
			}
			return
		case "--cleanup":
			cleanup(root)
			return
		case "--autostart":
			c, e := node.ReadConfig(root)
			if e == nil && c.AutoStart && !hostActive(root) {
				e = startNode(root, c)
			}
			if e != nil {
				os.WriteFile(filepath.Join(root, "host-error.txt"), []byte(e.Error()), 0600)
			}
			return
		}
	}
	if len(os.Args) == 1 && redirectToUpdatedManager(root) {
		return
	}
	lock, e := mutex(`Local\ZKasNodeUI-` + identity(root))
	if e != nil {
		walk.MsgBox(nil, "ZKas Node Manager", "A manager window is already open.", walk.MsgBoxIconInformation)
		return
	}
	defer windows.CloseHandle(lock)
	cfg, e := node.ReadConfig(root)
	if e != nil {
		walk.MsgBox(nil, "Configuration error", e.Error(), walk.MsgBoxIconError)
		return
	}
	ui := &manager{root: root, cfg: cfg}
	if e = ui.create(); e != nil {
		walk.MsgBox(nil, "Unable to open manager", e.Error(), walk.MsgBoxIconError)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--uninstall" {
		ui.window.Synchronize(ui.remove)
	}
	ui.window.Synchronize(ui.compactWindow)
	if len(os.Args) == 3 && os.Args[1] == "--manager-update-ready" {
		ui.window.Synchronize(func() { markManagerReady(root, os.Args[2]) })
	}
	ui.window.Run()
}

type manager struct {
	chainsUI
	kaspaSettingsUI
	kaspaSharingUI
	kaspaWalletUI
	setupSection    *walk.GroupBox
	logContainer    *walk.Composite
	logPause        *walk.CheckBox
	mainTabs        *walk.TabWidget
	settingsPage    *walk.TabPage
	managerUpdate   *walk.PushButton
	managerPreviews *walk.CheckBox
	viewToolsUI
	walletUI

	miningStats *walk.TextEdit
	sharingUI

	mining                                               node.MiningConfig
	miningStatus                                         *walk.Label
	miningURL, miningAddress, miningWorker               *walk.LineEdit
	miningDiff                                           *walk.NumberEdit
	miningLAN                                            *walk.ComboBox
	miningInstall, miningStart, miningStop, miningRemove *walk.PushButton
	lanAddresses                                         []node.LANAddress
	walletPort, resident                                 *walk.NumberEdit
	enableWallet                                         *walk.CheckBox
	walletStatus                                         *walk.Label
	logSelect                                            *walk.ComboBox
	root                                                 string
	cfg                                                  node.Config
	window                                               *walk.MainWindow
	data                                                 *walk.LineEdit
	grpc                                                 *walk.NumberEdit
	ws                                                   *walk.NumberEdit
	enableWS, auto                                       *walk.CheckBox
	mode                                                 *walk.ComboBox
	status, version, progress, endpoints                 *walk.Label
	logs                                                 *logView
	install, start, stop, update, browse, removeButton   *walk.PushButton
	busy                                                 bool
	closed                                               atomic.Bool
}

func (m *manager) create() error {
	// Mining settings are independent of the existing node configuration.
	var miningErr error
	m.mining, miningErr = node.ReadMining(m.root)
	if miningErr != nil {
		return miningErr
	}
	m.shared, miningErr = node.ReadSharing(m.root)
	if miningErr != nil {
		return miningErr
	}
	m.lanAddresses = node.LANAddresses()
	// AssignTo fields are populated before event handlers run.

	e := (MainWindow{
		MenuItems: []MenuItem{Menu{Text: "Window", Items: []MenuItem{Action{Text: "Compact / restore", Shortcut: Shortcut{Key: walk.KeyEscape}, OnTriggered: m.compactWindow}}}},
		AssignTo:  &m.window, Title: "ZKas + Kaspa Node Manager", MinSize: Size{Width: 360, Height: 280}, Size: Size{Width: 680, Height: 520},
		Font: Font{Family: "Segoe UI", PointSize: 9}, Layout: VBox{Margins: Margins{Left: 10, Top: 8, Right: 10, Bottom: 8}, Spacing: 6},
		OnKeyDown: func(key walk.Key) {
			if key == walk.KeyEscape {
				m.compactWindow()
			}
		},
		Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: "ZKas + Kaspa Node Manager", Font: Font{PointSize: 14, Bold: true}, EllipsisMode: EllipsisEnd}, HSpacer{},
				PushButton{Text: "Compact / restore", OnClicked: m.compactWindow},
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{PushButton{Text: "ZKas", OnClicked: func() { m.mainTabs.SetCurrentIndex(0); m.overviewTabs.SetCurrentIndex(0) }}, PushButton{Text: "Kaspa", OnClicked: func() { m.mainTabs.SetCurrentIndex(0); m.overviewTabs.SetCurrentIndex(1) }}, PushButton{Text: "Mining", OnClicked: func() { m.mainTabs.SetCurrentIndex(2) }}, HSpacer{}}},
			TabWidget{AssignTo: &m.mainTabs, OnCurrentIndexChanged: m.refreshVisiblePanel, Pages: []TabPage{
				{Title: "Overview", Layout: VBox{MarginsZero: true}, Children: []Widget{TabWidget{AssignTo: &m.overviewTabs, Pages: []TabPage{{Title: "ZKas", Layout: VBox{MarginsZero: true}, Children: []Widget{
					ScrollView{Layout: VBox{Spacing: 10}, Children: []Widget{
						GroupBox{AssignTo: &m.setupSection, Visible: m.cfg.Version == "", Title: "Setup", Layout: VBox{Spacing: 6}, Children: []Widget{
							PushButton{Text: "Install / setup", Font: Font{Bold: true}, OnClicked: func() {
								if err := m.mainTabs.SetCurrentIndex(m.mainTabs.Pages().Index(m.settingsPage)); err != nil {
									walk.MsgBox(m.window, "Open setup", err.Error(), walk.MsgBoxIconError)
								}
							}},
							Label{Text: "Open Settings to install components or review your setup."},
						}},
						Label{AssignTo: &m.status, Text: "Checking…", Font: Font{PointSize: 12, Bold: true}},
						Label{AssignTo: &m.walletStatus, Text: "Wallet backend: checking…"},
						Label{AssignTo: &m.version},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							PushButton{AssignTo: &m.start, Text: "Start ZKas services", OnClicked: m.startClicked},
							PushButton{AssignTo: &m.stop, Text: "Stop ZKas services", OnClicked: func() {
								m.action(func() error { m.setProgress("Waiting for clean shutdown…"); return stopNode(m.root) })
							}},
							PushButton{AssignTo: &m.update, Text: "Check node updates", OnClicked: m.checkUpdate},
						}},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							PushButton{AssignTo: &m.managerUpdate, Text: "Check manager updates", OnClicked: m.checkManagerUpdate},
							CheckBox{AssignTo: &m.managerPreviews, Text: "Include preview releases", Checked: true},
						}},
						Label{Text: "Local connections", Font: Font{Bold: true}}, Label{AssignTo: &m.endpoints},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							PushButton{Text: "Copy API addresses", OnClicked: func() { m.copyText("API addresses", m.endpointText()) }},
							PushButton{Text: "Copy wallet API URL", OnClicked: func() {
								if !m.cfg.EnableWallet {
									walk.MsgBox(m.window, "Wallet backend disabled", "Enable and install the wallet backend first.", walk.MsgBoxIconInformation)
									return
								}
								m.copyText("Wallet API URL", fmt.Sprintf("http://127.0.0.1:%d", m.cfg.WalletPort))
							}},
						}},
						Label{Text: "Consensus sync and wallet history scans finish separately."},
						Label{Text: "Closing this window leaves your services running."},
					}},
				}}, m.kaspaOverviewTab()}}}},
				m.allWalletTab(),
				m.allMiningTab(),
				m.allSharingTab(),
				{AssignTo: &m.settingsPage, Title: "Settings", Layout: VBox{MarginsZero: true}, Children: []Widget{TabWidget{Pages: []TabPage{{Title: "ZKas", Layout: VBox{MarginsZero: true}, Children: []Widget{
					ScrollView{Layout: VBox{Spacing: 8}, Children: []Widget{
						Label{Text: "Install and configure", Font: Font{PointSize: 12, Bold: true}},
						Label{Text: "The defaults suit most users. Review the options below, then install."},
						PushButton{AssignTo: &m.install, Text: "Install / repair components", OnClicked: m.installAction},
						Label{Text: "Stop services before changing settings."},
						Label{Text: "Blockchain data folder"},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							LineEdit{AssignTo: &m.data, Text: m.cfg.DataDir, MinSize: Size{Width: 220}},
							PushButton{AssignTo: &m.browse, Text: "Browse…", OnClicked: func() {
								d := walk.FileDialog{Title: "Choose blockchain data folder", FilePath: m.data.Text()}
								if ok, _ := d.ShowBrowseFolder(m.window); ok {
									m.data.SetText(d.FilePath)
								}
							}},
						}},
						Label{Text: "Storage mode"},
						ComboBox{AssignTo: &m.mode, Model: []string{"Wallet / application backend (recommended)", "Basic pruned node (no old wallet history)", "Archive (requires an empty folder)"}, CurrentIndex: modeIndex(m.cfg.Mode)},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							Label{Text: "gRPC port"}, NumberEdit{AssignTo: &m.grpc, MinValue: 1024, MaxValue: 65535, Decimals: 0, Value: float64(m.cfg.GRPC)},
						}},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							CheckBox{AssignTo: &m.enableWS, Text: "JSON wRPC", Checked: m.cfg.EnableWebSocket}, NumberEdit{AssignTo: &m.ws, MinValue: 1024, MaxValue: 65535, Decimals: 0, Value: float64(m.cfg.WebSocket)},
						}},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							CheckBox{AssignTo: &m.enableWallet, Text: "Wallet REST backend", Checked: m.cfg.EnableWallet}, NumberEdit{AssignTo: &m.walletPort, MinValue: 1024, MaxValue: 65535, Decimals: 0, Value: float64(m.cfg.WalletPort)},
						}},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							Label{Text: "Resident wallets (0 = automatic)"}, NumberEdit{AssignTo: &m.resident, MinValue: 0, MaxValue: 10000, Decimals: 0, Value: float64(m.cfg.ResidentWallets)},
						}},
						CheckBox{AssignTo: &m.auto, Text: "Start ZKas services when I sign in", Checked: m.cfg.AutoStart},
						Label{Text: "Settings save when you install or start. Remote access is configured separately in Sharing."},

						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							PushButton{Text: "Open data", OnClicked: func() { m.open(m.cfg.DataDir) }},
							PushButton{AssignTo: &m.removeButton, Text: "Uninstall…", OnClicked: m.remove},
						}},
					}},
				}}, m.kaspaSettingsTab()}}}},
				{Title: "Logs", Layout: VBox{}, Children: []Widget{
					Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						ComboBox{AssignTo: &m.logSelect, Model: []string{"ZKas node log", "Wallet backend log", "ZKas mining log", "Tor log", "Kaspa node log", "Kaspa / merged mining log", "Kaspa Tor log"}, CurrentIndex: 0, OnCurrentIndexChanged: m.refreshLogs},
						PushButton{Text: "Latest", OnClicked: m.latestLog},
						CheckBox{AssignTo: &m.logPause, Text: "Pause"},
						PushButton{Text: "Open log folder", OnClicked: func() { m.open(filepath.Join(m.root, "logs")) }},
					}},
					Label{Text: "Scroll up to hold your place. Latest resumes following. Colors: red errors, amber warnings, cyan sync.", EllipsisMode: EllipsisEnd},
					Composite{AssignTo: &m.logContainer, Layout: VBox{MarginsZero: true, Spacing: 0}, Children: []Widget{HSpacer{}, VSpacer{}}, MinSize: Size{Width: 100, Height: 120}, StretchFactor: 1},
				}},
			}},
			Label{AssignTo: &m.progress, Text: "Connected to your existing installation.", EllipsisMode: EllipsisEnd},
		}}).Create()
	if e != nil {
		return e
	}
	m.logs, e = newLogView(m.logContainer)
	if e != nil {
		return e
	}
	m.refreshLogs()
	for _, u := range m.minePages {
		u.detailsChanged()
	}
	m.window.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if m.busy {
			*canceled = true
			walk.MsgBox(m.window, "Operation in progress", "Please wait for the current operation to finish. A payment may still be preparing or broadcasting.", walk.MsgBoxIconInformation)
		} else {
			if m.toolsCancel != nil {
				m.toolsCancel()
			}
			m.closed.Store(true)
			if !m.chainsPolling {
				m.kaspaMonitor.Close()
				m.zkasMiningMonitor.Close()
			}
			if m.kwProcess != nil {
				m.kwProcess.stop()
			}
		}
	})
	go func() {
		monitor := &node.InfoMonitor{}
		defer monitor.Close()
		var lastWalletPoll time.Time
		for {
			time.Sleep(2 * time.Second)
			if m.closed.Load() {
				return
			}
			var walletID, walletToken string
			var personalPort int
			var snapshot node.Config
			tab := -1
			visible := false
			m.onUI(func() {
				snapshot = m.cfg
				visible = m.window.Visible() && !win.IsIconic(m.window.Handle())
				if visible {
					tab = m.mainTabs.CurrentIndex()
				}
				if tab == 1 && m.walletTabs.CurrentIndex() == 0 && time.Since(lastWalletPoll) >= 5*time.Second {
					walletID, walletToken, personalPort = m.walletPollSnapshot()
				}
			})
			if walletID != "" {
				m.pollPersonalWallet(walletID, walletToken, personalPort)
				lastWalletPoll = time.Now()
			}
			if !visible {
				continue
			}
			port := snapshot.GRPC
			r, e := hostCommand(m.root, "status")
			var info node.NodeInfo
			var rpcErr error
			if tab == 0 && e == nil && r.Running {
				info, rpcErr = monitor.Get(context.Background(), port)
			}
			walletText := "Wallet backend: disabled"
			if snapshot.EnableWallet {
				walletText = "Wallet backend: stopped"
				if b, err := os.ReadFile(serviceErrorFile(m.root, "wallet")); err == nil && len(b) > 0 {
					reason := strings.SplitN(string(b), "\n", 2)[0]
					if len(reason) > 240 {
						reason = reason[:240] + "…"
					}
					walletText += "\n" + reason
				}
				if snapshot.WalletExecutable == "" {
					walletText = "Wallet backend: not installed (see Settings)"
				}
				if serviceActive(m.root, "wallet") {
					walletText = "Wallet backend: running / waiting for REST"
					if node.WalletHealth(context.Background(), snapshot.WalletPort) == nil {
						walletText = "Wallet REST online (wallet scans may continue)"
					}
				}
			}
			statsText := "Mining bridge is stopped."
			if tab == 2 && serviceActive(m.root, "mining") {
				stats, statsErr := node.ReadMiningStats(context.Background())
				if statsErr != nil {
					statsText = "Statistics unavailable. If using the previous bridge host, stop ONLY mining, then click Set up / start mining once to enable statistics. Otherwise check mining logs and port 18888. No live data is being displayed."
				} else {
					statsText = "Updated " + time.Now().Format("15:04:05") + "\r\n" + stats.Display()
					if n, e := stratumConnections(); e == nil {
						statsText = fmt.Sprintf("Connected Stratum TCP sessions: %d\r\n", n) + statsText
					} else {
						statsText = "Connected sessions: unavailable\r\n" + statsText
					}
				}
			}
			if tab == 2 && serviceActive(m.root, "dual") {
				kc, _ := chains.Read(m.root)
				if kc.Mode == "zkas" {
					statsText = "ZKas-only mining is running through the merged-capable bridge. Open its live dashboard below for workers, hashrate and blocks."
				}
			}

			m.onUI(func() {
				if (tab == 0 || tab == 2) && time.Since(m.chainsPolled) > 2*time.Second {
					m.refreshChains()

				}
				if tab == 1 && m.walletTabs.CurrentIndex() == 1 {
					m.kwPoll()
				}
				if tab == 2 && m.miningStats.Text() != statsText {
					m.miningStats.SetText(statsText)
				}
				if m.walletStatus.Text() != walletText {
					m.walletStatus.SetText(walletText)
				}
				m.refresh(r, e, info, rpcErr)
			})
		}
	}()
	preventWheelChanges(m.window.Handle())
	m.refresh(hostReply{}, fmt.Errorf("initial"), node.NodeInfo{}, nil)
	return nil
}
func modeIndex(s string) int {
	switch s {
	case "pruned":
		return 1
	case "archive":
		return 2
	}
	return 0
}
func (m *manager) readSettings() (node.Config, error) {
	c := m.cfg
	c.DataDir = strings.TrimSpace(m.data.Text())
	c.GRPC = int(m.grpc.Value())
	c.WebSocket = int(m.ws.Value())
	c.EnableWebSocket = m.enableWS.Checked()
	c.EnableWallet = m.enableWallet.Checked()
	c.WalletPort = int(m.walletPort.Value())
	c.ResidentWallets = int(m.resident.Value())
	c.AutoStart = m.auto.Checked()
	c.Mode = []string{"wallet", "pruned", "archive"}[m.mode.CurrentIndex()]
	if e := validateData(m.root, c); e != nil {
		return c, e
	}
	if c.Mode == "archive" && (m.cfg.Mode != "archive" || c.DataDir != m.cfg.DataDir || m.cfg.Version == "") {
		entries, e := os.ReadDir(c.DataDir)
		if e != nil && !os.IsNotExist(e) {
			return c, e
		}
		if len(entries) > 0 {
			return c, fmt.Errorf("Choose an empty data folder for a new archive. Previously pruned blocks cannot be recovered by enabling archival mode")
		}
	}
	return c, nil
}
func (m *manager) endpointText() string {
	s := fmt.Sprintf("gRPC: 127.0.0.1:%d", m.cfg.GRPC)
	if m.cfg.EnableWebSocket {
		s += fmt.Sprintf("\r\nJSON wRPC: ws://127.0.0.1:%d", m.cfg.WebSocket)
	}
	if m.cfg.EnableWallet {
		s += fmt.Sprintf("\r\nWallet REST API: http://127.0.0.1:%d", m.cfg.WalletPort)
	}
	return s
}
func (m *manager) setProgress(s string) { m.onUI(func() { m.progress.SetText(s) }) }
func (m *manager) refresh(r hostReply, hostErr error, info node.NodeInfo, rpcErr error) {
	installed := m.cfg.Version != ""
	m.setupSection.SetVisible(!installed)
	running := hostErr == nil && r.Running
	active := running || hostActive(m.root)
	state := "Not installed"
	if installed {
		state = "Stopped"
	}
	if active {
		state = "Starting / waiting for node host…"
	}
	if running {
		state = "Running · waiting for RPC"
		if r.Stopping {
			state = "Shutting down…"
		} else if rpcErr == nil {
			state = "Syncing consensus…"
			if info.Synced {
				state = "Consensus synced (wallet scans may continue)"
			}
		}
	}
	if m.status.Text() != state {
		m.status.SetText(state)
	}
	m.refreshWalletControls()
	if m.mainTabs.CurrentIndex() == 2 {
		m.refreshMining()
	}
	if m.mainTabs.CurrentIndex() == 3 {
		m.refreshSharing()
	}
	versionText := "Installed node: " + m.cfg.Version + "    Manager: " + node.ManagerVersion
	if m.version.Text() != versionText {
		m.version.SetText(versionText)
	}
	endpointText := m.endpointText()
	if m.endpoints.Text() != endpointText {
		m.endpoints.SetText(endpointText)
	}
	for _, w := range []walk.Widget{m.data, m.grpc, m.ws, m.enableWS, m.mode, m.auto, m.browse, m.enableWallet, m.walletPort, m.resident} {
		w.SetEnabled(!m.busy && !active)
	}
	m.install.SetEnabled(!m.busy && !active)
	m.start.SetEnabled(!m.busy && installed && (!serviceActive(m.root, "node") || m.cfg.EnableWallet && !serviceActive(m.root, "wallet")))
	m.stop.SetEnabled(!m.busy && active)
	m.update.SetEnabled(!m.busy && installed)
	m.managerUpdate.SetEnabled(!m.busy)
	m.managerPreviews.SetEnabled(!m.busy)
	_, componentsErr := os.Stat(filepath.Join(m.root, "components"))
	m.removeButton.SetEnabled(!m.busy && (installed || componentsErr == nil))
	m.refreshLogs()
	m.updateChainControls()
}
func (m *manager) action(work func() error) {
	if m.busy {
		return
	}
	m.busy = true
	m.refresh(hostReply{}, fmt.Errorf("refresh"), node.NodeInfo{}, nil)
	go func() {
		e := work()
		if m.closed.Load() {
			return
		}
		m.onUI(func() {
			m.busy = false
			if e != nil {
				m.progress.SetText("Action did not complete. See the details below.")
				walk.MsgBox(m.window, "ZKas Node Manager", e.Error(), walk.MsgBoxIconError)
			} else {
				if m.chainCompletion != "" {
					m.progress.SetText(m.chainCompletion)
				} else {
					m.progress.SetText("Done.")
				}
			}
			m.chainOperation = ""
			m.chainCompletion = ""
			m.updateChainControls()
			m.refreshChains()
		})
	}()
}
func (m *manager) startClicked() {
	c, e := m.readSettings()
	if hostActive(m.root) {
		c = m.cfg
		e = nil
	}
	if e != nil {
		walk.MsgBox(m.window, "Check setup", e.Error(), walk.MsgBoxIconWarning)
		return
	}
	m.action(func() error {
		if e := node.CheckBinary(c); e != nil {
			return e
		}
		if e := node.Save(m.root, c); e != nil {
			return e
		}
		m.onUI(func() { m.cfg = c })
		if e := setAutoStart(m.root, c.AutoStart); e != nil {
			return e
		}
		m.setProgress("Starting services…")
		return startNode(m.root, c)
	})
}
func (m *manager) installAction() {
	c, e := m.readSettings()
	if e != nil {
		walk.MsgBox(m.window, "Check setup", e.Error(), walk.MsgBoxIconWarning)
		return
	}
	m.action(func() error {
		m.setProgress("Finding the latest stable Windows node release…")
		r, e := node.Latest(context.Background())
		if e != nil {
			return e
		}
		return m.applyRelease(c, r, true)
	})
}
func (m *manager) applyRelease(c node.Config, r node.Release, start bool) error {
	if e := ensureInstalled(m.root); e != nil {
		return e
	}
	exe, hash, e := node.Install(context.Background(), m.root, r, c.EnableWallet, m.setProgress)
	if e != nil {
		return e
	}
	if hostActive(m.root) {
		m.setProgress("Download verified. Stopping wallet backend, then node…")
		if e = stopNode(m.root); e != nil {
			return e
		}
	}
	old, readErr := node.ReadConfig(m.root)
	if readErr != nil {
		return readErr
	}
	if old.Version != "" {
		if e = node.AtomicJSON(filepath.Join(m.root, "previous-config.json"), old); e != nil {
			return e
		}
	}
	c.Version = r.Tag
	c.Executable = exe
	c.SHA256 = hash
	c.WalletExecutable = ""
	c.WalletSHA256 = ""
	if c.EnableWallet {
		c.WalletExecutable = filepath.Join(filepath.Dir(exe), "zkas-walletd.exe")
		c.WalletSHA256, e = node.FileHash(c.WalletExecutable)
		if e != nil {
			return e
		}
	}
	c.Published = r.Published
	if e = node.Save(m.root, c); e != nil {
		return e
	}
	m.onUI(func() { m.cfg = c })
	if e = setAutoStart(m.root, c.AutoStart); e != nil {
		return e
	}
	if start {
		m.setProgress("Starting the verified services…")
		return startNode(m.root, c)
	}
	return nil
}
func (m *manager) checkUpdate() {
	c := m.cfg
	m.action(func() error {
		m.setProgress("Checking official stable releases…")
		r, e := node.Latest(context.Background())
		if e != nil {
			return e
		}
		if r.Tag == c.Version {
			m.onUI(func() {
				walk.MsgBox(m.window, "Node is current", "You have the latest stable node release: "+r.Tag, walk.MsgBoxIconInformation)
			})
			return nil
		}
		if !c.Published.IsZero() && !r.Published.After(c.Published) {
			return fmt.Errorf("The latest release is not newer than your installation; refusing an automatic downgrade")
		}
		yes := false
		m.onUI(func() {
			yes = walk.MsgBox(m.window, "Update node", fmt.Sprintf("Update %s to %s?\n\nThe download will be verified before the node stops. Pause connected applications before updating so no payment is in flight. Services will briefly disconnect. Data and settings are kept.\n\nRelease notes:\n%s", c.Version, r.Tag, r.Body), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes
		})
		if !yes {
			return nil
		}
		return m.applyRelease(c, r, hostActive(m.root))
	})
}
func (m *manager) open(path string) {
	os.MkdirAll(path, 0700)
	exec.Command("explorer.exe", path).Start()
}
func (m *manager) remove() {
	if m.busy {
		return
	}
	if walk.MsgBox(m.window, "Uninstall ZKas Node Manager", "Stop ZKas, Kaspa and all mining bridges (including their firewall rules), and remove the installed programs and startup entry?\n\nBlockchain data, wallet scan files, logs and settings will be kept. A small temporary cleanup executable may remain in your Windows temp folder.", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	m.kwLock()
	m.action(func() error {
		if e := uninstall(m.root); e != nil {
			return e
		}
		m.onUI(func() { m.busy = false; m.window.Close() })
		return nil
	})
}

// Walk Synchronize queues work asynchronously. Wait here when callers need a
// returned decision or a consistent config snapshot before continuing.
func (m *manager) onUI(f func()) {
	done := make(chan struct{})
	m.window.Synchronize(func() { defer close(done); f() })
	<-done
}

// Reset to a manageable size within the current monitor's work area. Scrollable
// tab contents cannot inflate the top-level window's minimum size.
func (m *manager) compactWindow() {
	h := m.window.Handle()
	win.ShowWindow(h, win.SW_RESTORE)
	mi := win.MONITORINFO{CbSize: uint32(unsafe.Sizeof(win.MONITORINFO{}))}
	if !win.GetMonitorInfo(win.MonitorFromWindow(h, win.MONITOR_DEFAULTTONEAREST), &mi) {
		m.window.SetSize(walk.Size{Width: 680, Height: 520})
		return
	}
	area := mi.RcWork
	width, height := int(area.Right-area.Left), int(area.Bottom-area.Top)
	dpi := m.window.DPI()
	w, hgt := 680*dpi/96, 520*dpi/96
	if w > width*85/100 {
		w = width * 85 / 100
	}
	if hgt > height*85/100 {
		hgt = height * 85 / 100
	}
	m.window.SetBoundsPixels(walk.Rectangle{X: int(area.Left) + (width-w)/2, Y: int(area.Top) + (height-hgt)/2, Width: w, Height: hgt})
}

func (m *manager) refreshVisiblePanel() {
	if m.mainTabs == nil || m.logs == nil {
		return
	}
	switch m.mainTabs.CurrentIndex() {
	case 1:
		m.refreshWalletControls()
	case 2:
		m.refreshMining()
		m.refreshChains()
	case 3:
		m.refreshSharing()
	case 5:
		m.refreshLogs()
	case 0:
		m.refreshChains()
	}
}
