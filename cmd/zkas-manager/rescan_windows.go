//go:build windows

package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	"zkas-node-manager/internal/node"
)

func (m *manager) rescanZKasWallets() {
	if m.busy {
		return
	}
	tokens, err := node.RegisteredWalletTokens(m.root)
	if err != nil {
		walk.MsgBox(m.window, "Wallet rescan", err.Error(), walk.MsgBoxIconError)
		return
	}
	if len(tokens) == 0 {
		walk.MsgBox(m.window, "Wallet rescan", "No registered ZKas wallets were found on this node.", walk.MsgBoxIconInformation)
		return
	}
	if walk.MsgBox(m.window, "Rescan all ZKas wallets", fmt.Sprintf("Restart scans from the beginning for all %d registered ZKas wallets, including wallets used by bots and other applications?\n\nWallet registrations, keys and access tokens are kept. Balances and history may be incomplete until scanning finishes. Pause bot payments and avoid sending while scans rebuild. This can take hours.\n\nThe backend must be running and able to serve history from genesis.", len(tokens)), walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
		return
	}
	port := m.cfg.WalletPort
	m.action(func() error {
		if err := node.WalletHealth(context.Background(), port); err != nil {
			return fmt.Errorf("Start the ZKas wallet backend before requesting a rescan")
		}
		m.setProgress("Requesting ZKas wallet rescans…")
		count, err := node.RescanRegisteredWallets(context.Background(), port, tokens, func(done, total int) { m.setProgress(fmt.Sprintf("Rescan requests accepted: %d / %d", done, total)) })
		message := fmt.Sprintf("Rescan requests accepted: %d / %d. Scanning continues when wallets are loaded by the manager, bots or other clients. This count is not completed scans.", count, len(tokens))
		m.onUI(func() { m.rescanStatus.SetText(message) })
		if err != nil {
			return fmt.Errorf("%s\n\n%s", message, err)
		}
		m.onUI(func() { m.chainCompletion = message })
		return nil
	})
}
