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
	logs                                                 *walk.TextEdit
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
		AssignTo:  &m.window, Title: "ZKas Node Manager", MinSize: Size{Width: 360, Height: 280}, Size: Size{Width: 680, Height: 520},
		Font: Font{Family: "Segoe UI", PointSize: 9}, Layout: VBox{Margins: Margins{Left: 10, Top: 8, Right: 10, Bottom: 8}, Spacing: 6},
		OnKeyDown: func(key walk.Key) {
			if key == walk.KeyEscape {
				m.compactWindow()
			}
		},
		Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: "ZKas Node Manager", Font: Font{PointSize: 14, Bold: true}, EllipsisMode: EllipsisEnd}, HSpacer{},
				PushButton{Text: "Compact / restore", OnClicked: m.compactWindow},
			}},
			TabWidget{Pages: []TabPage{
				{Title: "Overview", Layout: VBox{MarginsZero: true}, Children: []Widget{
					ScrollView{Layout: VBox{Spacing: 10}, Children: []Widget{
						Label{AssignTo: &m.status, Text: "Checking…", Font: Font{PointSize: 12, Bold: true}},
						Label{AssignTo: &m.walletStatus, Text: "Wallet backend: checking…"},
						Label{AssignTo: &m.version},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							PushButton{AssignTo: &m.start, Text: "Start services", OnClicked: m.startClicked},
							PushButton{AssignTo: &m.stop, Text: "Stop services", OnClicked: func() {
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
				}},
				m.walletTab(),
				m.miningTab(),
				m.sharingTab(),
				{Title: "Settings", Layout: VBox{MarginsZero: true}, Children: []Widget{
					ScrollView{Layout: VBox{Spacing: 8}, Children: []Widget{
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
						CheckBox{AssignTo: &m.auto, Text: "Start services when I sign in", Checked: m.cfg.AutoStart},
						Label{Text: "Settings save when you install or start. Remote access is configured separately in Sharing."},
						PushButton{AssignTo: &m.install, Text: "Install / repair components", OnClicked: m.installAction},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							PushButton{Text: "Open data", OnClicked: func() { m.open(m.cfg.DataDir) }},
							PushButton{AssignTo: &m.removeButton, Text: "Uninstall…", OnClicked: m.remove},
						}},
					}},
				}},
				{Title: "Logs", Layout: VBox{}, Children: []Widget{
					Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						ComboBox{AssignTo: &m.logSelect, Model: []string{"Node log", "Wallet backend log", "Mining bridge log", "Tor log"}, CurrentIndex: 0},
						PushButton{Text: "Open log folder", OnClicked: func() { m.open(filepath.Join(m.root, "logs")) }},
					}},
					TextEdit{AssignTo: &m.logs, ReadOnly: true, VScroll: true, HScroll: true, Font: Font{Family: "Consolas", PointSize: 9}},
				}},
			}},
			Label{AssignTo: &m.progress, Text: "Connected to your existing installation.", EllipsisMode: EllipsisEnd},
		}}).Create()
	if e != nil {
		return e
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
		}
	})
	go func() {
		monitor := &node.InfoMonitor{}
		defer monitor.Close()
		for {
			time.Sleep(2 * time.Second)
			if m.closed.Load() {
				return
			}
			var walletID, walletToken string
			var personalPort int
			var snapshot node.Config
			m.onUI(func() { snapshot = m.cfg; walletID, walletToken, personalPort = m.walletPollSnapshot() })
			m.pollPersonalWallet(walletID, walletToken, personalPort)
			port := snapshot.GRPC
			r, e := hostCommand(m.root, "status")
			var info node.NodeInfo
			var rpcErr error
			if e == nil && r.Running {
				info, rpcErr = monitor.Get(context.Background(), port)
			}
			walletText := "Wallet backend: disabled"
			if snapshot.EnableWallet {
				walletText = "Wallet backend: stopped"
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
			if serviceActive(m.root, "mining") {
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
			m.onUI(func() {
				m.miningStats.SetText(statsText)
				m.walletStatus.SetText(walletText)
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
	m.status.SetText(state)
	m.refreshWalletControls()
	m.refreshMining()
	m.refreshSharing()
	m.version.SetText("Installed node: " + m.cfg.Version + "    Manager: " + node.ManagerVersion)
	m.endpoints.SetText(m.endpointText())
	for _, w := range []walk.Widget{m.data, m.grpc, m.ws, m.enableWS, m.mode, m.auto, m.browse, m.enableWallet, m.walletPort, m.resident} {
		w.SetEnabled(!m.busy && !active)
	}
	m.install.SetEnabled(!m.busy && !active)
	m.start.SetEnabled(!m.busy && installed && (!serviceActive(m.root, "node") || m.cfg.EnableWallet && !serviceActive(m.root, "wallet")))
	m.stop.SetEnabled(!m.busy && active)
	m.update.SetEnabled(!m.busy && installed)
	m.managerUpdate.SetEnabled(!m.busy)
	m.managerPreviews.SetEnabled(!m.busy)
	m.removeButton.SetEnabled(!m.busy && installed)
	logName := "console.log"
	if m.logSelect.CurrentIndex() == 3 {
		logName = "tor-console.log"
	}
	if m.logSelect.CurrentIndex() == 2 {
		logName = "mining-console.log"
	}
	if m.logSelect.CurrentIndex() == 1 {
		logName = "wallet-console.log"
	}
	path := filepath.Join(m.root, "logs", logName)
	if f, e := os.Open(path); e == nil {
		st, _ := f.Stat()
		if st != nil {
			offset := st.Size() - 16000
			if offset < 0 {
				offset = 0
			}
			f.Seek(offset, 0)
			b := make([]byte, 16000)
			n, _ := f.Read(b)
			m.logs.SetText(strings.ReplaceAll(strings.ReplaceAll(string(b[:n]), "\r", ""), "\n", "\r\n"))
			m.logs.SetTextSelection(len(m.logs.Text()), len(m.logs.Text()))
		}
		f.Close()
	}
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
				m.progress.SetText("Done.")
			}
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
		if e := ensureInstalled(m.root); e != nil {
			return e
		}
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
	if walk.MsgBox(m.window, "Uninstall ZKas Node Manager", "Stop all services, uninstall the mining bridge (including its port 5555 firewall rule), and remove the installed programs and startup entry?\n\nBlockchain data, wallet scan files, logs and settings will be kept. A small temporary cleanup executable may remain in your Windows temp folder.", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
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
