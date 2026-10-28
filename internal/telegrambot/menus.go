package telegrambot

import (
	"context"
	"strings"
	"time"
)

type Choice struct{ Text, Command string }
type Reply struct {
	Text    string
	Choices [][]Choice
}
type Button struct {
	Text     string    `json:"text"`
	Data     string    `json:"callback_data,omitempty"`
	CopyText *CopyText `json:"copy_text,omitempty"`
}
type CopyText struct {
	Text string `json:"text"`
}
type menuAction struct {
	UserID  int64
	Command string
	Expires time.Time
}

func privateSender(u *User, c Chat) bool {
	return u != nil && !u.IsBot && u.ID > 0 && c.Type == "private" && c.ID == u.ID
}
func homeRow() []Choice { return []Choice{{"Main menu", "menu"}} }
func (e *Engine) mainMenu() [][]Choice {
	var rows [][]Choice
	for _, x := range []struct{ permission, label, command string }{
		{"status", "Node status", "status"}, {"mining", "Mining dashboard", "mining"},
		{"node", "Node controls", "nodes"}, {"bridge", "Mining controls", "bridges"},
		{"wallets", "ZKas wallets", "wallets"}, {"kaspa_wallets", "Kaspa wallets", "kaspa_wallets"}, {"fvks", "FVK-only wallets", "fvks"},
	} {
		if e.permitted(x.permission) {
			rows = append(rows, []Choice{{x.label, x.command}})
		}
	}
	return append(rows, []Choice{{"Help", "help"}})
}
func (e *Engine) nodeMenu() [][]Choice {
	return [][]Choice{
		{{"Start ZKas", "node start zkas"}, {"Stop ZKas", "node stop zkas"}},
		{{"Start Kaspa", "node start kaspa"}, {"Stop Kaspa", "node stop kaspa"}}, homeRow(),
	}
}
func (e *Engine) bridgeMenu() [][]Choice {
	return [][]Choice{
		{{"Start ZKas mining", "bridge start zkas"}}, {{"Start Kaspa mining", "bridge start kaspa"}},
		{{"Start merged mining", "bridge start merged"}}, {{"Stop mining", "bridge stop"}}, homeRow(),
	}
}
func (e *Engine) resultMenu(ctx context.Context, r Request) [][]Choice {
	switch r.Command {
	case "node":
		return e.nodeMenu()
	case "bridge":
		return e.bridgeMenu()
	case "kaspa_wallets", "kaspa_open", "kaspa_account":
		if e.Menu != nil {
			return e.walletNavigation(e.Menu(ctx, r), "kaspa")
		}
		return [][]Choice{homeRow()}
	case "wallets", "fvks":
		var rows [][]Choice
		if e.Menu != nil {
			rows = e.Menu(ctx, r)
		}
		if r.Command == "wallets" && e.permitted("kaspa_wallets") {
			rows = append(rows, []Choice{{"Kaspa wallets", "kaspa_wallets"}})
		}
		if r.Command == "wallets" {
			return e.walletNavigation(rows, "zkas")
		}
		return append(rows, homeRow())
	case "wallet", "balance", "history", "receive", "sync", "payment", "fvk", "fvk_balance", "fvk_history", "fvk_sync":
		if len(r.Args) != 1 {
			return [][]Choice{homeRow()}
		}
		id := r.Args[0]
		var rows [][]Choice
		if strings.HasPrefix(r.Command, "fvk") {
			rows = [][]Choice{{{"Balance", "fvk_balance " + id}, {"History", "fvk_history " + id}}, {{"Sync wallet", "fvk_sync " + id}}, {{"FVK wallets", "fvks"}}}
		} else {
			rows = [][]Choice{{{"Balance", "balance " + id}, {"History", "history " + id}}, {{"Receive address", "receive " + id}, {"Sync wallet", "sync " + id}}}
			if e.permitted("send") {
				rows = append(rows, []Choice{{"Send ZKAS", "send zkas " + id}})
			}
			if e.permitted("payment") {
				rows = append(rows, []Choice{{"Request payment (local review)", "payment " + id}})
			}
			rows = append(rows, []Choice{{"Wallets", "wallets"}})
		}
		if !strings.HasPrefix(r.Command, "fvk") {
			return e.walletNavigation(rows, "zkas")
		}
		return append(rows, homeRow())
	case "status", "mining", "kaspa":
		return [][]Choice{{{"Refresh", r.Command}}, homeRow()}
	}
	return e.mainMenu()
}
func describe(r Request) string {
	chain := func(s string) string {
		switch s {
		case "zkas":
			return "ZKas"
		case "kaspa":
			return "Kaspa"
		case "merged":
			return "merged ZKas + Kaspa"
		}
		return s
	}
	if r.Command == "node" && len(r.Args) == 2 {
		if r.Args[0] == "stop" {
			return "Stop the " + chain(r.Args[1]) + " node and its dependent services?"
		}
		if r.Args[0] == "start" {
			return "Start the " + chain(r.Args[1]) + " node?"
		}
	}
	if r.Command == "bridge" && len(r.Args) > 0 {
		if r.Args[0] == "stop" {
			return "Stop mining?"
		}
		if len(r.Args) == 2 && r.Args[0] == "start" {
			return "Start " + chain(r.Args[1]) + " mining using saved settings?"
		}
	}
	if (r.Command == "sync" || r.Command == "fvk_sync") && len(r.Args) == 1 {
		return "Start or refresh the scan for wallet " + r.Args[0] + "?"
	}
	if r.Command == "payment" && len(r.Args) == 1 {
		return "Request the local send dialog for wallet " + r.Args[0] + "? Payment still requires review on the PC."
	}
	return "Run " + r.Command + " " + strings.Join(r.Args, " ") + "?"
}

// Each button is tied to its recipient and this running bot. No keys/tokens in
// callback data, and an old confirmation button cannot approve a newer request.
func (e *Engine) Keyboard(id int64, choices [][]Choice, now time.Time) ([][]Button, error) {
	if !e.Config.Allows(id) {
		return nil, nil
	}
	if e.buttons == nil {
		e.buttons = map[string]menuAction{}
	}
	for key, a := range e.buttons {
		if !now.Before(a.Expires) {
			delete(e.buttons, key)
		}
	}
	// Bound memory even if allowed users request menus continually.
	if len(e.buttons) > 4096 {
		e.buttons = map[string]menuAction{}
	}
	var rows [][]Button
	for i, row := range choices {
		if i >= 32 {
			break
		}
		var buttons []Button
		for j, c := range row {
			if j >= 3 {
				break
			}
			if strings.HasPrefix(c.Command, "copy:") {
				value := strings.TrimPrefix(c.Command, "copy:")
				if len([]rune(value)) > 0 && len([]rune(value)) <= 256 {
					buttons = append(buttons, Button{Text: c.Text, CopyText: &CopyText{Text: value}})
				}
				continue
			}
			key, err := Secret()
			if err != nil {
				return nil, err
			}
			expiry := now.Add(30 * time.Minute)
			if strings.HasPrefix(c.Command, "pay_confirm ") || strings.HasPrefix(c.Command, "confirm ") || strings.HasPrefix(c.Command, "cancel ") {
				expiry = now.Add(2 * time.Minute)
			}
			e.buttons[key] = menuAction{id, c.Command, expiry}
			label := strings.Join(strings.Fields(c.Text), " ")
			chars := []rune(label)
			if len(chars) > 55 {
				label = string(chars[:52]) + "…"
			}
			buttons = append(buttons, Button{Text: label, Data: key})
		}
		if len(buttons) > 0 {
			rows = append(rows, buttons)
		}
	}
	return rows, nil
}
func (e *Engine) ProcessCallback(ctx context.Context, c Callback, now time.Time) (Reply, error) {
	if ctx.Err() != nil {
		return Reply{}, ctx.Err()
	}
	if c.Message == nil || !privateSender(c.From, c.Message.Chat) || !e.Config.Allows(c.From.ID) {
		return Reply{}, nil
	}
	action, ok := e.buttons[c.Data]
	if !ok || action.UserID != c.From.ID || !now.Before(action.Expires) {
		return Reply{Text: "This menu has expired. Open a fresh menu below.", Choices: e.mainMenu()}, nil
	}
	if strings.HasPrefix(action.Command, "pay_confirm ") {
		p := e.payments[c.From.ID]
		if p == nil || !now.Before(p.Expires) || action.Command != "pay_confirm "+p.Code {
			return Reply{}, nil
		}
	}
	// Consume action confirmations before dispatch. Navigation can be used again.
	if strings.HasPrefix(action.Command, "confirm ") || strings.HasPrefix(action.Command, "cancel ") {
		delete(e.buttons, c.Data)
	}
	return e.Process(ctx, Message{From: c.From, Chat: c.Message.Chat, Text: "/" + action.Command, Date: now.Unix()}, now)
}

func (e *Engine) walletNavigation(rows [][]Choice, chain string) [][]Choice {
	if e.permitted("lock") && e.Config.Permissions.Chain(chain) {
		rows = append(rows, []Choice{{"Lock Telegram wallet", "lock " + chain}})
	}
	return append(rows, homeRow())
}
