package main

import "time"

func (m *manager) rememberWalletOpen(id string) {
	if m.openedWallets == nil {
		m.openedWallets = map[string]time.Time{}
	}
	m.openedWallets[id] = time.Now()
}
func (m *manager) idleWalletMessage() string {
	if !serviceActive(m.root, "wallet") {
		return "Run the ZKas node to sync this wallet."
	}
	return "Click Refresh / sync to open and sync this wallet."
}
