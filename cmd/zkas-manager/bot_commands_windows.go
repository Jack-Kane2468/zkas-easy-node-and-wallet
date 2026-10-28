package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	"strconv"
	"strings"
	"time"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
	"zkas-node-manager/internal/telegrambot"
	"zkas-node-manager/internal/wallet"
)

// Only these read-only fields leave the UI thread. No seed, password or recovery secret.
type botWallet struct{ ID, Name, Token, FVK, Address string }

func shortBotID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
func (m *manager) botWallets(ctx context.Context) ([]botWallet, error) {
	var result []botWallet
	busy := false
	err := m.botUIRun(ctx, func() {
		busy = m.busy
		if m.botZVault == nil || busy {
			return
		}
		for _, w := range m.botZVault.Wallets {
			if !w.SignedOut {
				result = append(result, botWallet{w.ID, w.Name, w.Token, w.FVK, w.Address})
			}
		}
	})
	if busy {
		return nil, fmt.Errorf("Manager busy")
	}
	return result, err
}
func (m *manager) botHandle(ctx context.Context, c telegrambot.Config, r telegrambot.Request) (string, error) {
	switch r.Command {
	case "unlock", "lock":
		return m.botVaultAction(ctx, c, r)
	case "status":
		cfg, err := node.ReadConfig(m.root)
		if err != nil {
			return "ZKas is not configured.", nil
		}
		kc, err := chains.Read(m.root)
		if err != nil {
			return "Kaspa settings could not be read.", nil
		}
		rows := []string{"Checked " + time.Now().Format(time.RFC3339)}
		for _, x := range []struct {
			name, service string
			port          int
		}{{"ZKas", "node", cfg.GRPC}, {"Kaspa", "kaspa", kc.GRPC}} {
			if !serviceActive(m.root, x.service) {
				rows = append(rows, x.name+": stopped")
				continue
			}
			q, cancel := context.WithTimeout(ctx, 4*time.Second)
			info, e := node.GetInfo(q, x.port)
			cancel()
			state := "running; RPC unavailable"
			if e == nil {
				state = "running; syncing"
				if info.Synced {
					state = "running; synced"
				}
				state += "; " + info.Version
			}
			rows = append(rows, x.name+": "+state)
		}
		rows = append(rows, fmt.Sprintf("ZKas wallet backend running: %t", serviceActive(m.root, "wallet")))
		return strings.Join(rows, "\n"), nil
	case "mining":
		mode, port := "", 0
		if serviceActive(m.root, "mining") {
			mode = "zkas"
			port = 18888
		}
		if serviceActive(m.root, "dual") {
			kc, e := chains.Read(m.root)
			if e != nil {
				return "Bridge settings unavailable.", nil
			}
			mode = kc.Mode
			port = 18889
		}
		if mode == "" {
			return "Mining is stopped.", nil
		}
		stats, e := node.ReadMiningStatsPort(ctx, port)
		if e != nil {
			return "Mining mode: " + mode + ". The bridge host is active, but its statistics API is unavailable. This does not establish whether miners are connected; check the bridge log.", nil
		}
		return "Checked " + time.Now().Format(time.RFC3339) + "\n" + stats.Dashboard(mode), nil
	case "node", "bridge":
		return m.botServiceAction(ctx, r)
	case "kaspa_wallets":
		if len(m.botKaspaWallets(ctx)) == 0 {
			return "Unlock Kaspa in Telegram, or use Unlock Kaspa for bot in the Bot tab.", nil
		}
		return "Choose a Kaspa wallet.", nil
	case "kaspa_open", "kaspa_account":
		return m.botKaspaOpen(ctx, r)
	case "kaspa":
		return m.botKaspa(ctx)
	case "wallet", "wallets", "balance", "history", "receive", "sync", "payment", "fvk", "fvks", "fvk_balance", "fvk_history", "fvk_sync":
		view := strings.HasPrefix(r.Command, "fvk")
		var wallets []botWallet
		var err error
		if view {
			for _, w := range c.Watches {
				wallets = append(wallets, botWallet{w.ID, w.Name, w.Token, w.FVK, w.Address})
			}
		} else {
			wallets, err = m.botWallets(ctx)
			if err != nil {
				return "Manager busy or closing. Try again after the current operation.", nil
			}
		}
		if r.Command == "wallets" || r.Command == "fvks" {
			if len(wallets) == 0 {
				if view {
					return "No FVK wallets configured in the Bot tab.", nil
				}
				return "No signed-in wallets available. Unlock ZKas in Telegram, or use Unlock ZKas for bot in the Bot tab.", nil
			}
			_, _, page := botWalletPage(r.Args, len(wallets))
			return fmt.Sprintf("Choose a wallet (page %d).", page+1), nil
		}
		if len(r.Args) != 1 {
			return "Supply one wallet ID from /wallets or /fvks.", nil
		}
		var w botWallet
		matches := 0
		for _, v := range wallets {
			if len(r.Args[0]) >= 8 && strings.HasPrefix(v.ID, r.Args[0]) {
				w = v
				matches++
			}
		}
		if matches != 1 {
			return "Wallet unavailable or ID ambiguous. Use /wallets or /fvks again.", nil
		}
		if r.Command == "wallet" || r.Command == "fvk" {
			return "Wallet: " + w.Name + "\nChoose an action below.", nil
		}
		if r.Command == "payment" {
			return m.botPaymentRequest(ctx, w, r.UserID)
		}
		cfg, err := node.ReadConfig(m.root)
		if err != nil || !serviceActive(m.root, "wallet") {
			return "Start the ZKas node and wallet backend in the manager first.", nil
		}
		client := wallet.Client{Port: cfg.WalletPort, Token: w.Token}
		q, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if r.Command == "sync" || r.Command == "fvk_sync" {
			if !view {
				if err = m.botUIRun(ctx, func() { m.rememberWalletOpen(w.ID) }); err != nil {
					return "", err
				}
			}
			s, e := client.Status(q)
			if e != nil {
				return "Wallet backend unavailable. Check its log in the manager.", nil
			}
			if !s.HasWallet {
				if e = client.Watch(q, w.FVK); e != nil {
					return "Could not register this viewing key. Check the key and wallet backend in the manager.", nil
				}
			}
		}
		s, e := client.Status(q)
		if e != nil {
			return "Wallet backend unavailable. Check its log in the manager.", nil
		}
		if !s.HasWallet {
			return "Wallet is not registered. Use /sync ID or /fvk_sync ID first.", nil
		}
		if s.Address != "" && w.Address != "" && s.Address != w.Address {
			return "Wallet address mismatch; request refused.", nil
		}
		var result string
		if strings.HasSuffix(r.Command, "history") {
			if s.Loading {
				return "Wallet is opening/scanning; try history later.", nil
			}
			p, e := client.HistoryPage(q, 0)
			if e != nil {
				return "History could not be read from the wallet backend.", nil
			}
			lines := []string{w.Name + ": latest history (up to 10 entries)"}
			for i, row := range p.Rows {
				if i == 10 {
					break
				}
				lines = append(lines, fmt.Sprintf("%s | amount (sompi): %s\nTX: %s\nRecipient: %s\nMemo: %s", row.Kind, row.Amount, row.TxID, row.Recipient, row.DisplayMemo()))
			}
			if len(p.Rows) == 0 {
				lines = append(lines, "No history entries returned.")
			}
			result = strings.Join(lines, "\n\n")
		} else if r.Command == "receive" {
			result = w.Name + "\nReceive address: " + s.Address
		} else {
			if s.Error != "" {
				result = w.Name + ": wallet backend reports an error; inspect this wallet in the manager. Balance may be incomplete."
			} else {
				result = w.Name + "\n" + s.Display()
			}
		}
		// Do not return vault data if the user locked or signed out during the request.
		if !view {
			current, e := m.botWallets(ctx)
			if e != nil {
				return "Vault unavailable.", nil
			}
			found := false
			for _, v := range current {
				if v.ID == w.ID && v.Token == w.Token {
					found = true
				}
			}
			if !found {
				return "Wallet access was revoked or the vault was locked.", nil
			}
		}
		return result, nil
	}
	return "Command unavailable.", nil
}
func (m *manager) botServiceAction(ctx context.Context, r telegrambot.Request) (string, error) {
	if len(r.Args) < 1 {
		return "Specify start or stop and a chain/mining mode.", nil
	}
	action := r.Args[0]
	valid := false
	if r.Command == "node" {
		valid = len(r.Args) == 2 && (action == "start" || action == "stop") && (r.Args[1] == "zkas" || r.Args[1] == "kaspa")
	} else {
		valid = (len(r.Args) == 1 && action == "stop") || (len(r.Args) == 2 && action == "start" && (r.Args[1] == "zkas" || r.Args[1] == "kaspa" || r.Args[1] == "merged"))
	}
	if !valid {
		return "Use /node start|stop zkas|kaspa, /bridge start zkas|kaspa|merged, or /bridge stop.", nil
	}
	claimed := false
	if err := m.botUIRun(ctx, func() {
		if !m.busy {
			m.busy = true
			claimed = true
			m.progress.SetText("Telegram service request in progress…")
		}
	}); err != nil {
		return "", err
	}
	if !claimed {
		return "Manager busy. Wait for the current operation to finish.", nil
	}
	// Finish any started graceful stop, even if Telegram disconnects. Never force-kill.
	defer func() {
		if !m.closed.Load() {
			m.window.Synchronize(func() {
				m.busy = false
				m.progress.SetText("Telegram service request finished. Check Overview for current status.")
				m.refreshChains()
				m.updateChainControls()
			})
		}
	}()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	cfg, err := node.ReadConfig(m.root)
	if err != nil {
		return "Node settings unavailable. Complete setup locally first.", nil
	}
	if r.Command == "node" {
		if r.Args[1] == "zkas" {
			if action == "start" {
				err = startNode(m.root, cfg)
			} else {
				err = stopNode(m.root)
			}
		} else {
			if action == "start" {
				err = startKaspa(m.root, cfg)
			} else {
				err = stopKaspa(m.root)
			}
		}
	} else if action == "stop" {
		err = stopOne(m.root, "mining")
		if err == nil {
			err = stopOne(m.root, "dual")
		}
	} else if r.Args[1] == "zkas" {
		var mc node.MiningConfig
		mc, err = node.ReadMining(m.root)
		if err == nil {
			err = startMining(m.root, cfg, mc)
		}
	} else {
		var kc chains.Config
		kc, err = chains.Read(m.root)
		if err == nil {
			kc.Mode = r.Args[1]
			err = startDual(m.root, cfg, kc)
		}
	}
	if err != nil {
		m.botUIRun(ctx, func() { m.botState.SetText("Service request failed: " + err.Error()) })
		return "Service action could not complete. See the Bot tab for the error. Required nodes must be running and synced before starting mining.", nil
	}
	return "Service request completed. Use /status or /mining to check the result.", nil
}
func (m *manager) botKaspa(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var p *kaspaWalletProcess
	var revision uint64
	err := m.botUIRun(ctx, func() {
		if m.botKAvailable(m.botKState.Filename) && m.botKState.Open && !m.busy {
			p = m.botKProcess
			revision = m.botKRevision
		}
	})
	if err != nil {
		return "", err
	}
	if p == nil {
		return "Open a Kaspa wallet/account from the bot menu first.", nil
	}
	var s kaspaWalletState
	if err = p.callContext(ctx, map[string]any{"op": "status"}, &s); err != nil {
		return "Kaspa wallet backend unavailable.", nil
	}
	valid := false
	m.botUIRun(ctx, func() {
		valid = m.botKAvailable(m.botKState.Filename) && m.botKState.Open && m.botKProcess == p && m.botKRevision == revision
	})
	if !valid {
		return "Kaspa wallet changed or was locked. Request again.", nil
	}
	if s.Error != "" {
		return "Kaspa wallet reports an error; check the manager.", nil
	}
	result := fmt.Sprintf("Kaspa: connected=%t, synced=%t\nBalance: %s", s.Connected, s.Synced, s.Balance)
	for _, a := range s.Accounts {
		if a.ID == s.Selected {
			result += "\nAccount: " + a.Name + "\nReceive: " + a.Address
		}
	}
	return result, nil
}
func (m *manager) botPaymentRequest(ctx context.Context, w botWallet, user int64) (string, error) {
	// Queue an explicit local workflow; Telegram never gets seed/signing data.
	accepted := false
	err := m.botUIRun(ctx, func() {
		if m.busy || m.vault == nil {
			return
		}
		current := m.vault.Find(w.ID)
		if current == nil || current.SignedOut || current.Seed == "" {
			return
		}
		if walk.MsgBox(m.window, "Telegram payment request", fmt.Sprintf("Allowed Telegram user %d requests the send dialog for wallet %s.\n\nOpen it? You will enter the recipient and amount here, then review the fee before any signing or broadcast.", user, current.Name), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
			return
		}
		if ctx.Err() != nil || m.vault == nil {
			return
		}
		m.selectedWallet = w.ID
		m.rebuildWalletList()
		accepted = true
		m.sendWallet()
	})
	if err != nil {
		return "", err
	}
	if !accepted {
		return "Payment request declined or wallet unavailable. No payment was initiated by the bot.", nil
	}
	return "The local send dialog was opened. This is not a payment receipt; check wallet history for the outcome.", nil
}

// Paginate names without making wallet backend requests or opening wallets.
func botWalletPage(args []string, total int) (int, int, int) {
	page := 0
	if len(args) == 1 {
		n, err := strconv.Atoi(args[0])
		if err == nil && n >= 0 && n <= total/10 {
			page = n
		}
	}
	start := page * 10
	if start >= total && total > 0 {
		start = 0
		page = 0
	}
	end := start + 10
	if end > total {
		end = total
	}
	return start, end, page
}
func (m *manager) botWalletMenu(ctx context.Context, c telegrambot.Config, r telegrambot.Request) [][]telegrambot.Choice {
	if strings.HasPrefix(r.Command, "kaspa_") {
		return m.botKaspaMenu(ctx, c, r)
	}
	var wallets []botWallet
	command := "wallet"
	if r.Command == "fvks" {
		command = "fvk"
		for _, w := range c.Watches {
			wallets = append(wallets, botWallet{ID: w.ID, Name: w.Name})
		}
	} else {
		var err error
		wallets, err = m.botWallets(ctx)
		if err != nil {
			return nil
		}
	}
	start, end, page := botWalletPage(r.Args, len(wallets))
	var rows [][]telegrambot.Choice
	for _, w := range wallets[start:end] {
		rows = append(rows, []telegrambot.Choice{{Text: w.Name, Command: command + " " + w.ID}})
	}
	var nav []telegrambot.Choice
	if page > 0 {
		nav = append(nav, telegrambot.Choice{Text: "Previous wallets", Command: fmt.Sprintf("%s %d", r.Command, page-1)})
	}
	if end < len(wallets) {
		nav = append(nav, telegrambot.Choice{Text: "More wallets", Command: fmt.Sprintf("%s %d", r.Command, page+1)})
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	return rows
}
