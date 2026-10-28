package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"zkas-node-manager/internal/telegrambot"
)

func (m *manager) botKaspaWallets(ctx context.Context) []botWallet {
	var out []botWallet
	m.botUIRun(ctx, func() {
		if m.botKVault == nil || m.busy {
			return
		}
		for _, w := range m.botKVault.Wallets {
			if !m.botKAvailable(w.ID) {
				continue
			}
			out = append(out, botWallet{ID: w.ID, Name: w.Name})
		}
	})
	return out
}
func (m *manager) botKaspaOpen(ctx context.Context, r telegrambot.Request) (string, error) {
	if len(r.Args) < 1 || len(r.Args) > 2 {
		return "Choose a Kaspa wallet from the menu.", nil
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	release, err := m.botAcquire(ctx)
	if err != nil {
		return err.Error(), nil
	}
	defer release()
	var p *kaspaWalletProcess
	var password, name string
	var setupErr error
	m.botUIRun(ctx, func() {
		if m.botKVault == nil {
			return
		}
		w := m.botKVault.Find(r.Args[0])
		if w == nil || !m.botKAvailable(w.ID) {
			return
		}
		setupErr = m.botKEnsure()
		if setupErr != nil {
			return
		}
		p = m.botKProcess
		password = w.Password
		name = w.Name
		m.botKRevision++
	})
	if setupErr != nil {
		return "Kaspa wallet could not open: " + setupErr.Error(), nil
	}
	if p == nil {
		return "This wallet is unavailable. Unlock Kaspa for Telegram and choose a wallet from a fresh menu.", nil
	}
	m.botLog("INFO", "Kaspa wallet refresh: reading wallet state.")
	changed := false
	var s kaspaWalletState
	err = p.callContext(ctx, map[string]any{"op": "status"}, &s)
	if err == nil && r.Command == "kaspa_account" {
		if len(r.Args) != 2 || !s.Open || s.Filename != r.Args[0] {
			return "The open wallet changed. Open it again from the menu.", nil
		}
		changed = true
		err = p.callContext(ctx, map[string]any{"op": "select", "account": r.Args[1]}, &s)
	} else if err == nil && (!s.Open || s.Filename != r.Args[0]) {
		changed = true
		err = p.callContext(ctx, map[string]any{"op": "open", "filename": r.Args[0], "password": password}, &s)
	}
	password = ""
	if err == nil && !s.Connected {
		changed = true
		m.botLog("INFO", "Kaspa wallet refresh: connecting and activating account (40 second request limit).")
		err = p.callContext(ctx, map[string]any{"op": "connect"}, &s)
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			m.botLog("WARN", "Kaspa wallet refresh deadline/cancellation reached; no automatic retry."+p.diagnostic.trace())
		} else if !p.alive() {
			m.botLog("ERROR", p.diagnostic.reason())
		} else {
			m.botLog("WARN", "Kaspa SDK rejected the wallet refresh request.")
		}
		uiCtx, uiCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer uiCancel()
		m.botUIRun(uiCtx, func() {
			if m.botKProcess == p {
				if !p.alive() {
					m.botKApply(kaspaWalletState{})
				}
				m.botState.SetText("Telegram Kaspa wallet refresh failed.")
			}
		})
		return "Kaspa wallet refresh failed or timed out. Check Logs > Bot and the Kaspa wallet panel. No payment was retried.", nil
	}
	if err = m.botUIRun(ctx, func() { m.botKApply(s) }); err != nil {
		m.botLog("WARN", "Kaspa wallet refresh could not update the manager before its deadline.")
		return "Wallet refresh could not update the manager. Try again when the manager is responsive.", nil
	}

	if changed {
		var saveErr error
		err = m.botUIRun(ctx, func() { saveErr = m.botKSnapshotState(s) })
		if err != nil || saveErr != nil {
			return "Wallet opened, but its backup could not be updated. Check the manager.", nil
		}
	}
	m.botLog("INFO", fmt.Sprintf("Kaspa wallet refresh finished: connected=%t, node-synced=%t, balance-ready=%t.", s.Connected, s.Synced, s.Ready))
	result := fmt.Sprintf("Kaspa wallet: %s\nConnected: %t | Node synced: %t\nBalance: %s", name, s.Connected, s.Synced, s.Balance)
	if s.Error != "" {
		result += "\n" + s.Error
	}
	for _, a := range s.Accounts {
		if a.ID == s.Selected {
			result += "\nAccount: " + a.Name + "\nReceive address: " + a.Address
		}
	}
	return result, nil
}
func (m *manager) botKaspaMenu(ctx context.Context, c telegrambot.Config, r telegrambot.Request) [][]telegrambot.Choice {
	var rows [][]telegrambot.Choice
	if r.Command == "kaspa_wallets" {
		ws := m.botKaspaWallets(ctx)
		start, end, page := botWalletPage(r.Args, len(ws))
		for _, w := range ws[start:end] {
			rows = append(rows, []telegrambot.Choice{{Text: w.Name, Command: "kaspa_open " + w.ID}})
		}
		if page > 0 {
			rows = append(rows, []telegrambot.Choice{{Text: "Previous wallets", Command: fmt.Sprintf("kaspa_wallets %d", page-1)}})
		}
		if end < len(ws) {
			rows = append(rows, []telegrambot.Choice{{Text: "More wallets", Command: fmt.Sprintf("kaspa_wallets %d", page+1)}})
		}
		return rows
	}
	var s kaspaWalletState
	m.botUIRun(ctx, func() {
		if m.botKAvailable(m.botKState.Filename) && !m.busy {
			s = m.botKState
		}
	})
	if s.Open && len(r.Args) > 0 && s.Filename == r.Args[0] {
		args := []string{}
		if r.Command == "kaspa_open" && len(r.Args) == 2 {
			args = r.Args[1:]
		}
		start, end, page := botWalletPage(args, len(s.Accounts))
		for _, a := range s.Accounts[start:end] {
			label := a.Name
			if a.ID == s.Selected {
				continue
			}
			rows = append(rows, []telegrambot.Choice{{Text: label, Command: "kaspa_account " + s.Filename + " " + a.ID}})
		}
		if page > 0 {
			rows = append(rows, []telegrambot.Choice{{Text: "Previous accounts", Command: fmt.Sprintf("kaspa_open %s %d", s.Filename, page-1)}})
		}
		if end < len(s.Accounts) {
			rows = append(rows, []telegrambot.Choice{{Text: "More accounts", Command: fmt.Sprintf("kaspa_open %s %d", s.Filename, page+1)}})
		}
		rows = append(rows, []telegrambot.Choice{{Text: "Refresh balance", Command: "kaspa_open " + s.Filename}})
		if c.Permissions.Spend && c.Permissions.KaspaWallets && s.Selected != "" {
			rows = append(rows, []telegrambot.Choice{{Text: "Send KAS", Command: strings.Join([]string{"send kaspa", s.Filename, s.Selected}, " ")}})
		}
	}
	return append(rows, []telegrambot.Choice{{Text: "Kaspa wallets", Command: "kaspa_wallets"}})
}
