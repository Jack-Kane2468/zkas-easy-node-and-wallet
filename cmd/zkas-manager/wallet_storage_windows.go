package main

import (
	"path/filepath"
	"zkas-node-manager/internal/node"
)

func (m *manager) openWalletData() { m.open(filepath.Join(m.root, "wallets")) }
func (m *manager) reviewWalletStorage() {
	if m.busy {
		return
	}
	m.chainAction("Reading wallet disk usage", "Wallet disk review finished. No files were changed.", func() error {
		report, e := node.WalletStorageReport(m.root)
		if e != nil {
			return e
		}
		m.onUI(func() {
			m.showText("Wallet disk usage", "Read-only inventory. For cleanup, use Open wallet data folder after stopping ZKas services. A running daemon may change file sizes during this review.", report, false)
		})
		return nil
	})
}
