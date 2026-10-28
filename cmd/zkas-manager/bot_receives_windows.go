package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
	"zkas-node-manager/internal/telegrambot"
	"zkas-node-manager/internal/wallet"
)

// Poll only available, already-synced wallets. Never switch Kaspa accounts or
// register/unlock a wallet in order to collect notifications.
func (m *manager) botReceiveEvents(ctx context.Context, state *telegrambot.MonitorState, c telegrambot.Config, rotation *int) []telegrambot.ReceiptEvent {
	var result []telegrambot.ReceiptEvent
	if c.Alerts.ReceiveZKas {
		var ws []botWallet
		if c.Permissions.Wallets {
			ws, _ = m.botWallets(ctx)
		}
		if c.Permissions.FVK {
			for _, w := range c.Watches {
				ws = append(ws, botWallet{w.ID, w.Name, w.Token, w.FVK, w.Address})
			}
		}
		// Same FVK in both lists is one wallet, even with different backend tokens.
		seen := map[string]bool{}
		unique := []botWallet{}
		for _, w := range ws {
			if !seen[w.FVK] {
				unique = append(unique, w)
				seen[w.FVK] = true
			}
		}
		ws = unique
		port := 0
		m.botUIRun(ctx, func() { port = m.cfg.WalletPort })
		for n := 0; n < 2 && n < len(ws); n++ {
			w := ws[(*rotation+n)%len(ws)]
			client := wallet.Client{Port: port, Token: w.Token}
			q, cancel := context.WithTimeout(ctx, 4*time.Second)
			s, e := client.Status(q)
			if e != nil || !s.HasWallet || !s.Synced || !s.NodeConnected || s.Loading || s.MissingHistory || s.Error != "" || (w.Address != "" && s.Address != w.Address) {
				cancel()
				continue
			}
			page, e := client.HistoryPage(q, 0)
			cancel()
			if e != nil {
				continue
			}
			// Recheck vault visibility after the network request before delivering data.
			allowed := false
			m.botUIRun(ctx, func() {
				if c.Permissions.Wallets && m.botZVault != nil {
					for _, x := range m.botZVault.Wallets {
						if !x.SignedOut && x.FVK == w.FVK {
							allowed = true
						}
					}
				}
				if c.Permissions.FVK {
					for _, x := range m.botConfig.Watches {
						if x.FVK == w.FVK {
							allowed = true
						}
					}
				}
			})
			if !allowed {
				continue
			}
			events := []telegrambot.ReceiptEvent{}
			for _, r := range page.Rows {
				if r.Kind != "received" {
					continue
				}
				address := r.Recipient
				if address == "" {
					var a []string
					for _, credit := range r.Credits {
						if credit.Recipient != "" {
							a = append(a, credit.Recipient)
						}
					}
					address = strings.Join(a, ", ")
				}
				events = append(events, telegrambot.ReceiptEvent{ID: r.TxID, Chain: "zkas", Wallet: w.Name, Amount: telegrambot.ReceiveAmount(r.Amount), Address: address, Memo: r.DisplayMemo(), Status: "Recorded in synced wallet history; confirmation count is not reported.", TimestampMS: telegrambot.TimestampMS(r.Timestamp)})
			}
			key := fmt.Sprintf("zkas:%x", sha256.Sum256([]byte(w.FVK)))
			result = append(result, state.NewReceipts(key, events, time.Now())...)
		}
		if len(ws) > 0 {
			*rotation = (*rotation + 2) % len(ws)
		}
	}
	if c.Alerts.ReceiveKaspa && c.Permissions.KaspaWallets {
		var p *kaspaWalletProcess
		var id, account, name string
		var revision uint64
		m.botUIRun(ctx, func() {
			if m.botKVault != nil && !m.busy && m.botKState.Open && m.botKState.Ready {
				if w := m.botKVault.Find(m.botKState.Filename); w != nil {
					p = m.botKProcess
					id = w.ID
					name = w.Name
					account = m.botKState.Selected
					revision = m.botKRevision
				}
			}
		})
		if p != nil {
			var data struct {
				Filename, Account string
				Rows              []telegrambot.ReceiptEvent
			}
			if e := p.call(map[string]any{"op": "alert_history"}, &data); e == nil && data.Filename == id && data.Account == account {
				valid := false
				m.botUIRun(ctx, func() {
					valid = m.botKVault != nil && m.botKProcess == p && m.botKRevision == revision && m.botKState.Filename == id && m.botKState.Selected == account
				})
				if valid {
					for i := range data.Rows {
						data.Rows[i].Chain = "kaspa"
						data.Rows[i].Wallet = name + " / " + account
					}
					result = append(result, state.NewReceipts("kaspa:"+id+":"+account, data.Rows, time.Now())...)
				}
			}
		}
	}
	return result
}
