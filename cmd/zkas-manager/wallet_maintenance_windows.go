package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	"time"
	"zkas-node-manager/internal/node"
	"zkas-node-manager/internal/wallet"
)

func (m *manager) showSpendingKey() {
	w := m.activeWallet()
	if w == nil {
		return
	}
	if w.Seed == "" {
		m.walletError(fmt.Errorf("This is a view-only wallet; it has no spending key"))
		return
	}
	if !m.reauthenticate() {
		return
	}
	m.showText("Spending key and account index", "Anyone with this spending key can spend this wallet's funds. Keep it private.", fmt.Sprintf("Wallet: %s\r\nRecovery phrase account index: %d\r\n\r\nSpending key (64 hex characters):\r\n%s\r\n\r\nThis key is already derived for the account shown above. When importing this key directly, use account index 0. Use the account index above only when importing the original recovery phrase.", w.Name, w.Account, w.Seed), false)
}
func (m *manager) selectedWalletStatus(ctx context.Context, token string, s wallet.Status) wallet.Status {
	if !s.Loading || s.Error != "" {
		return s
	}
	c, e := node.ReadConfig(m.root)
	if e != nil {
		return s
	}
	if msg := m.checkpointProbe.Check(ctx, m.root, token, c.GRPC); msg != "" {
		s.Error = msg
	}
	return s
}
func (m *manager) rescanSelectedWallet() {
	w := m.activeWallet()
	if w == nil {
		return
	}
	m.rememberWalletOpen(w.ID)
	id, token, name := w.ID, w.Token, w.Name
	port := m.cfg.WalletPort
	if walk.MsgBox(m.window, "Rescan selected ZKas wallet", fmt.Sprintf("Rescan %s from the beginning?\n\nOnly this wallet registration will be reset for scanning. Its keys and access token are kept. Balances and history can be incomplete until scanning finishes. Avoid sending from this wallet while it rebuilds.\n\nThe official backend's history-availability check remains enabled.", name), walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
		return
	}
	m.action(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
		defer cancel()
		count, e := node.RescanRegisteredWallets(ctx, port, []string{token}, nil)
		if e != nil {
			return e
		}
		if count != 1 {
			return fmt.Errorf("Backend did not confirm the rescan")
		}
		m.onUI(func() {
			if m.vault != nil && m.selectedWallet == id {
				m.walletBalance.SetText("Rescan accepted for " + name + ". Balance and history are rebuilding; this is not a completed scan.")
				m.walletHistory.SetText(receiptText(m.vault.Find(id)))
			}
			m.chainCompletion = "Rescan accepted for " + name + ". Scanning will resume as the wallet loads."
		})
		return nil
	})
}
