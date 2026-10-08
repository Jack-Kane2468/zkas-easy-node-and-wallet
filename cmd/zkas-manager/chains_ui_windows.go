package main

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"time"
	"zkas-node-manager/internal/node"
)

type chainsUI struct {
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

func (m *manager) kaspaOverviewTab() TabPage {
	return TabPage{Title: "Kaspa", Layout: VBox{MarginsZero: true}, Children: []Widget{ScrollView{Layout: VBox{Spacing: 10}, Children: []Widget{
		GroupBox{Title: "Kaspa node", Layout: VBox{Spacing: 6}, Children: []Widget{Label{AssignTo: &m.kaspaState, Text: "Not checked yet", Font: Font{PointSize: 12, Bold: true}}, Label{Text: "A separate Kaspa mainnet node. Download once, then let it sync."}, PushButton{AssignTo: &m.kaspaInstall, Text: "Install Kaspa + start syncing", OnClicked: func() {
			m.chainAction("Installing Kaspa", "Kaspa started; blockchain synchronization continues in the background.", func() error {
				if e := installKaspa(m.root, m.setProgress); e != nil {
					return e
				}
				return startKaspa(m.root, m.cfg)
			})
		}}, Label{AssignTo: &m.kaspaDetail, Text: "Checking installed components…"}, Composite{Layout: HBox{}, Children: []Widget{PushButton{AssignTo: &m.kaspaStart, Visible: false, Text: "Start Kaspa", OnClicked: func() {
			m.chainAction("Starting Kaspa", "Kaspa started. Sync status appears above.", func() error { return startKaspa(m.root, m.cfg) })
		}}, PushButton{AssignTo: &m.kaspaStop, Visible: false, Text: "Stop Kaspa", OnClicked: func() {
			m.chainAction("Stopping Kaspa", "Kaspa and shared mining are stopped. Data is retained.", func() error { return stopKaspa(m.root) })
		}}, PushButton{Text: "Refresh status", OnClicked: m.refreshChains}}}}},
		GroupBox{Title: "Local connections", Layout: VBox{}, Children: []Widget{Label{AssignTo: &m.kaspaEndpoints}, PushButton{Text: "Copy API addresses", OnClicked: func() { m.copyText("Kaspa API addresses", m.kaspaEndpointText()) }}, Label{Text: "Use Wallet → Kaspa for the built-in wallet. Sharing configures access from other devices."}}},
	}}}}
}
