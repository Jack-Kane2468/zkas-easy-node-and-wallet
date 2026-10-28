package telegrambot

import (
	"context"
	"strings"
	"time"
)

type unlockPrompt struct {
	Chain   string
	Expires time.Time
	Resume  Request
}

func (p Permissions) Chain(chain string) bool {
	switch chain {
	case "zkas":
		return p.Wallets
	case "kaspa":
		return p.KaspaWallets
	}
	return false
}
func WalletChain(r Request) string {
	switch r.Command {
	case "wallets", "wallet", "balance", "history", "receive", "sync", "payment":
		return "zkas"
	case "kaspa_wallets", "kaspa_open", "kaspa_account", "kaspa":
		return "kaspa"
	case "unlock", "lock", "send":
		if len(r.Args) > 0 && (r.Args[0] == "kaspa" || r.Args[0] == "zkas") {
			return r.Args[0]
		}
	}
	return ""
}
func (e *Engine) askUnlock(id int64, chain, desktop string, resume Request, now time.Time) Reply {
	if !e.permitted("unlock") {
		return Reply{Text: "This Telegram wallet is locked. Unlock it locally in the Bot tab, or enable remote lock/unlock.\n" + desktop, Choices: [][]Choice{homeRow()}}
	}
	if now.Before(e.unlockNext[id]) {
		return Reply{Text: "Please wait 30 seconds before another password attempt.\n" + desktop, Choices: [][]Choice{homeRow()}}
	}
	delete(e.payments, id)
	delete(e.pending, id)
	if e.unlocks == nil {
		e.unlocks = map[int64]unlockPrompt{}
	}
	e.unlocks[id] = unlockPrompt{Chain: chain, Expires: now.Add(2 * time.Minute), Resume: resume}
	return Reply{Text: "Enter your " + chain + " vault password to open these wallets in Telegram.\n" + desktop + "\n\nThis does not unlock the manager. Send the password within 2 minutes, or Cancel. Telegram cloud chats are not end-to-end encrypted; the password message will be deleted if Telegram allows it. Never send a recovery phrase or private key.", Choices: [][]Choice{{{"Cancel", "cancel"}}}}
}
func (e *Engine) processUnlock(ctx context.Context, m Message, cmd string, f []string, now time.Time) (Reply, bool, error) {
	id := m.From.ID
	home := func(s string) Reply { return Reply{Text: s, Choices: e.mainMenu()} }
	// A password is raw text: preserve spaces and do not confuse it with a typed
	// command. Slash commands (and menu callbacks) abandon the password prompt.
	if p, ok := e.unlocks[id]; ok {
		delete(e.unlocks, id)
		if !strings.HasPrefix(m.Text, "/") && cmd != "cancel" && cmd != "menu" {
			deleted := false
			if e.DeleteSecret != nil {
				deleted = e.DeleteSecret(id, m.ID) == nil
			}
			if !now.Before(p.Expires) {
				return home("Password prompt expired. Open Wallets again."), true, nil
			}
			if !e.permitted("unlock") || !e.Config.Permissions.Chain(p.Chain) {
				return home("Wallet permission was revoked."), true, nil
			}
			if e.unlockNext == nil {
				e.unlockNext = map[int64]time.Time{}
			}
			e.unlockNext[id] = now.Add(30 * time.Second)
			if e.Handle == nil {
				return home("Controller unavailable."), true, nil
			}
			s, err := e.Handle(ctx, Request{UserID: id, Command: "unlock", Args: []string{p.Chain, m.Text}})
			reply := home(s)
			if err == nil && p.Resume.Command != "" && e.WalletAccess != nil {
				unlocked, _, checkErr := e.WalletAccess(ctx, p.Chain)
				if checkErr != nil {
					return Reply{}, true, checkErr
				}
				if unlocked {
					// Re-enter normal dispatch so permissions, sync confirmation and payment
					// review rules are checked again before resuming the original action.
					command := "/" + p.Resume.Command
					if len(p.Resume.Args) > 0 {
						command += " " + strings.Join(p.Resume.Args, " ")
					}
					reply, err = e.Process(ctx, Message{From: m.From, Chat: m.Chat, Text: command, Date: now.Unix()}, now)
				}
			}
			if !deleted {
				reply.Text += "\nThe password message could not be deleted automatically. Delete it from the chat."
			}
			return reply, true, err
		}
	}
	if cmd == "unlock" || cmd == "lock" {
		delete(e.payments, id)
		delete(e.pending, id)
		if !e.permitted(cmd) {
			return home("Remote vault locking is disabled in the Bot tab."), true, nil
		}
		if len(f) != 2 || !e.Config.Permissions.Chain(f[1]) {
			return home("Choose an enabled chain: /" + cmd + " zkas or /" + cmd + " kaspa."), true, nil
		}
		if cmd == "lock" {
			if e.Handle == nil {
				return home("Controller unavailable."), true, nil
			}
			s, err := e.Handle(ctx, Request{UserID: id, Command: cmd, Args: f[1:]})
			return home(s), true, err
		}
		desktop := ""
		if e.WalletAccess != nil {
			_, desktop, _ = e.WalletAccess(ctx, f[1])
		}
		return e.askUnlock(id, f[1], desktop, Request{}, now), true, nil
	}
	r := Request{UserID: id, Command: cmd, Args: f[1:]}
	chain := WalletChain(r)
	if chain != "" && e.permitted(cmd) && e.Config.Permissions.Chain(chain) && e.WalletAccess != nil {
		unlocked, desktop, err := e.WalletAccess(ctx, chain)
		if err != nil {
			return Reply{}, true, err
		}
		if !unlocked {
			return e.askUnlock(id, chain, desktop, r, now), true, nil
		}
	}
	return Reply{}, false, nil
}
