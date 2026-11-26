package main

import (
	"context"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"time"
	"zkas-node-manager/internal/node"
)

type chainsUI struct {
	kaspaSetup                                                    *walk.GroupBox
	kaspaNodeUpdate, kaspaManagerUpdate                           *walk.PushButton
	kaspaManagerPreviews                                          *walk.CheckBox
	overviewTabs, walletTabs                                      *walk.TabWidget
	kaspaRAM                                                      *walk.NumberEdit
	kaspaSave                                                     *walk.PushButton
	kaspaInstall, kaspaStart, kaspaStop, kaspaRepair, kaspaRemove *walk.PushButton
	kaspaDetail, kaspaState, kaspaEndpoints                       *walk.Label
	kaspaSynced, zkasSynced                                       bool
	chainOperation, chainCompletion                               string
	chainsPolling                                                 bool
	chainsPolled                                                  time.Time
	kaspaMonitor, zkasMiningMonitor                               node.InfoMonitor
	minePages                                                     []*chainMiningPage
	miningInventory                                               *walk.Label
}

// Both chains share the same hierarchy; only chain-specific content differs.
func overviewPage(title string, nodeWidgets, updates, connections []Widget) TabPage {
	return TabPage{Title: title, Layout: VBox{MarginsZero: true}, Children: []Widget{
		ScrollView{Layout: VBox{Spacing: 10}, Children: []Widget{
			GroupBox{Title: title + " node", Layout: VBox{Spacing: 6}, Children: nodeWidgets},
			GroupBox{Title: "Updates", Layout: VBox{Spacing: 6}, Children: updates},
			GroupBox{Title: "Local connections", Layout: VBox{Spacing: 6}, Children: connections},
			Label{Text: "Closing this window leaves node services running."},
		}}}}
}
func (m *manager) openChainSettings(index int) {
	m.mainTabs.SetCurrentIndex(m.mainTabs.Pages().Index(m.settingsPage))
	m.settingsTabs.SetCurrentIndex(index)
}
func (m *manager) zkasOverviewTab() TabPage {
	return overviewPage("ZKas", []Widget{
		Label{AssignTo: &m.status, Text: "Checking…", Font: Font{PointSize: 12, Bold: true}},
		Label{AssignTo: &m.walletStatus, Text: "Wallet backend: checking…"},
		Label{AssignTo: &m.version},
		GroupBox{AssignTo: &m.setupSection, Visible: m.cfg.Version == "", Title: "Setup", Layout: VBox{}, Children: []Widget{
			PushButton{Text: "Set up ZKas…", OnClicked: func() { m.openChainSettings(0) }},
			Label{Text: "Choose your data folder and settings, then install components."},
		}},
		Composite{Layout: HBox{}, Children: []Widget{
			PushButton{AssignTo: &m.start, Text: "Start ZKas services", OnClicked: m.startClicked},
			PushButton{AssignTo: &m.stop, Text: "Stop ZKas services", OnClicked: func() {
				m.chainAction("Stopping ZKas services", "ZKas services stopped. Data is retained.", func() error { return stopNode(m.root) })
			}},
			PushButton{Text: "Refresh status", OnClicked: m.refreshOverviewNow},
		}},
		Label{Text: "Consensus sync and wallet history scans finish separately."},
	}, []Widget{
		PushButton{AssignTo: &m.update, Text: "Check node updates", OnClicked: m.checkUpdate},
		Label{Text: "Updates the ZKas node and enabled wallet backend. Bridge updates are in Mining."},
		PushButton{AssignTo: &m.managerUpdate, Text: "Check manager updates", OnClicked: m.checkManagerUpdate},
		CheckBox{AssignTo: &m.managerPreviews, Text: "Include preview releases", Checked: true, OnCheckedChanged: func() {
			if m.kaspaManagerPreviews != nil {
				m.kaspaManagerPreviews.SetChecked(m.managerPreviews.Checked())
			}
		}},
	}, []Widget{
		Label{AssignTo: &m.endpoints},
		PushButton{Text: "Copy API addresses", OnClicked: func() { m.copyText("ZKas API addresses", m.endpointText()) }},
		Label{Text: "Use Wallet → ZKas for the built-in wallet. Sharing configures access from other devices."},
	})
}
func (m *manager) kaspaOverviewTab() TabPage {
	return overviewPage("Kaspa", []Widget{
		Label{AssignTo: &m.kaspaState, Text: "Checking…", Font: Font{PointSize: 12, Bold: true}},
		Label{AssignTo: &m.kaspaDetail, Text: "Checking installed components…"},
		GroupBox{AssignTo: &m.kaspaSetup, Title: "Setup", Layout: VBox{}, Children: []Widget{
			PushButton{AssignTo: &m.kaspaInstall, Text: "Set up Kaspa…", OnClicked: func() { m.openChainSettings(1) }},
			Label{Text: "Choose your data folder and settings, then install components."},
		}},
		Composite{Layout: HBox{}, Children: []Widget{
			PushButton{AssignTo: &m.kaspaStart, Visible: false, Text: "Start Kaspa", OnClicked: func() {
				m.chainAction("Starting Kaspa", "Kaspa started. Sync status appears above.", func() error { return startKaspa(m.root, m.cfg) })
			}},
			PushButton{AssignTo: &m.kaspaStop, Visible: false, Text: "Stop Kaspa", OnClicked: func() {
				m.chainAction("Stopping Kaspa", "Kaspa and shared mining are stopped. Data is retained.", func() error { return stopKaspa(m.root) })
			}},
			PushButton{Text: "Refresh status", OnClicked: m.refreshOverviewNow},
		}},
	}, []Widget{
		PushButton{AssignTo: &m.kaspaNodeUpdate, Text: "Check node updates", OnClicked: m.checkKaspaNodeUpdate},
		Label{Text: "Updates the Kaspa node. Bridge updates are in Mining."},
		PushButton{AssignTo: &m.kaspaManagerUpdate, Text: "Check manager updates", OnClicked: m.checkManagerUpdate},
		CheckBox{AssignTo: &m.kaspaManagerPreviews, Text: "Include preview releases", Checked: true, OnCheckedChanged: func() {
			if m.managerPreviews != nil {
				m.managerPreviews.SetChecked(m.kaspaManagerPreviews.Checked())
			}
		}},
	}, []Widget{
		Label{AssignTo: &m.kaspaEndpoints},
		PushButton{Text: "Copy API addresses", OnClicked: func() { m.copyText("Kaspa API addresses", m.kaspaEndpointText()) }},
		Label{Text: "Use Wallet → Kaspa for the built-in wallet. Sharing configures access from other devices."},
	})
}

func (m *manager) refreshOverviewNow() {
	if m.busy {
		return
	}
	port := m.cfg.GRPC
	m.chainAction("Refreshing node status", "Node status refreshed.", func() error {
		r, e := hostCommand(m.root, "status")
		var info node.NodeInfo
		var rpcErr error
		if e == nil && r.Running {
			info, rpcErr = node.GetInfo(context.Background(), port)
		}
		m.onUI(func() { m.refresh(r, e, info, rpcErr) })
		return nil
	})
}
