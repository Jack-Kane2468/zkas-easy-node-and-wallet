package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	"io"
	"path/filepath"
	"time"
	"zkas-node-manager/internal/node"
	"zkas-node-manager/internal/wallet"
)

type stuckTarget struct{ token, cursor, address string }

func (m *manager) verifyStuck(ctx context.Context, token, cursor string) (stuckTarget, error) {
	t := stuckTarget{token: token, cursor: cursor}
	s, e := (wallet.Client{Port: m.cfg.WalletPort, Token: token}).Status(ctx)
	if e != nil {
		return t, fmt.Errorf("Run the ZKas node and wallet backend before rescanning")
	}
	if !s.HasWallet || !s.Loading {
		return t, fmt.Errorf("This wallet is not currently stuck opening; no rescan requested")
	}
	t.address = s.Address
	return t, node.VerifyRejectedCheckpoint(ctx, m.root, token, cursor, m.cfg.GRPC)
}
func (m *manager) rescanLogWallet(cursor, expectedToken string) {
	if m.busy {
		return
	}
	m.action(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		token, e := node.UniqueCheckpointOwner(m.root, cursor)
		if e != nil {
			return e
		}
		if token != expectedToken {
			return fmt.Errorf("The wallet matched to this log button has changed. No rescan requested")
		}
		t, e := m.verifyStuck(ctx, token, cursor)
		if e != nil {
			return e
		}
		yes := false
		m.onUI(func() {
			name := "External wallet"
			if m.vault != nil {
				for _, w := range m.vault.Wallets {
					if w.Token == token {
						name = w.Name
						break
					}
				}
			}
			yes = walk.MsgBox(m.window, "Rescan this stuck wallet", fmt.Sprintf("Rescan only %s?\n%s\n\nThis registration still has the checkpoint rejected in the log. Its keys and access token are kept. Balance and history will rebuild from the beginning. Pause payments for this wallet until it finishes.", name, t.address), walk.MsgBoxYesNo|walk.MsgBoxIconWarning) == walk.DlgCmdYes
		})
		if !yes {
			return nil
		}
		current, e := node.UniqueCheckpointOwner(m.root, cursor)
		if e != nil {
			return e
		}
		if current != token {
			return fmt.Errorf("Wallet registration changed; no rescan requested")
		}
		if _, e = m.verifyStuck(ctx, token, cursor); e != nil {
			return e
		}
		_, e = node.RescanRegisteredWallets(ctx, m.cfg.WalletPort, []string{token}, nil)
		if e != nil {
			return e
		}
		m.onUI(func() {
			m.chainCompletion = "Rescan accepted for the single wallet matched to this log entry."
			m.logOwnersChecked = time.Time{}
			m.logs.disableCursor(cursor)
		})
		return nil
	})
}
func (m *manager) rescanStuckWallets() {
	if m.busy {
		return
	}
	m.action(func() error {
		f, e := openSharedLog(filepath.Join(m.root, "logs", "wallet-console.log"))
		if e != nil {
			return fmt.Errorf("Cannot read the wallet backend log")
		}
		defer f.Close()
		st, e := f.Stat()
		if e != nil {
			return e
		}
		offset := st.Size() - (512 << 10)
		if offset < 0 {
			offset = 0
		}
		if _, e = f.Seek(offset, 0); e != nil {
			return e
		}
		b, e := io.ReadAll(io.LimitReader(f, 512<<10))
		if e != nil {
			return e
		}
		loops := node.RecentLoopCursors(string(b), time.Now())
		owners, e := node.CheckpointOwners(m.root)
		if e != nil {
			return e
		}
		targets := []stuckTarget{}
		for cursor := range loops {
			for _, token := range owners[cursor] {
				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				t, err := m.verifyStuck(ctx, token, cursor)
				cancel()
				if err == nil {
					targets = append(targets, t)
				}
			}
		}
		if len(targets) == 0 {
			m.onUI(func() {
				m.chainCompletion = "No currently stuck wallets confirmed. Checks use repeated checkpoint failures from the last 10 minutes."
			})
			return nil
		}
		yes := false
		m.onUI(func() {
			yes = walk.MsgBox(m.window, "Rescan stuck ZKas wallets", fmt.Sprintf("Rescan %d confirmed stuck wallet registrations, including bot/application wallets?\n\nEach has repeated recent checkpoint failures and is currently stuck opening. Keys and tokens are kept. Pause payments for these wallets while balances and history rebuild.", len(targets)), walk.MsgBoxYesNo|walk.MsgBoxIconWarning) == walk.DlgCmdYes
		})
		if !yes {
			return nil
		}
		accepted, skipped := 0, 0
		for _, t := range targets {
			ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
			_, err := m.verifyStuck(ctx, t.token, t.cursor)
			if err != nil {
				cancel()
				skipped++
				continue
			}
			_, err = node.RescanRegisteredWallets(ctx, m.cfg.WalletPort, []string{t.token}, nil)
			cancel()
			if err != nil {
				return fmt.Errorf("Rescans accepted: %d. Stopped: %s", accepted, err)
			}
			accepted++
		}
		m.onUI(func() {
			m.chainCompletion = fmt.Sprintf("Rescans accepted: %d. Skipped after rechecking: %d. Scans are not yet complete.", accepted, skipped)
			m.rescanStatus.SetText(m.chainCompletion)
			m.logOwnersChecked = time.Time{}
		})
		return nil
	})
}
func (m *manager) refreshLogOwners() {
	if m.logOwnersBusy || time.Since(m.logOwnersChecked) < 30*time.Second {
		return
	}
	m.logOwnersBusy = true
	go func() {
		owners, e := node.CheckpointOwners(m.root)
		if m.closed.Load() {
			return
		}
		m.onUI(func() {
			m.logOwnersBusy = false
			m.logOwnersChecked = time.Now()
			if e != nil {
				owners = nil
			}
			m.logOwners = owners
			m.logs.owners = owners
			m.logs.layoutActions()
		})
	}()
}
