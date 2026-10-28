// Package telegrambot implements an opt-in, private-chat-only manager controller.
package telegrambot

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Permissions struct {
	Status       bool `json:"status"`
	Mining       bool `json:"mining"`
	Nodes        bool `json:"nodes"`
	Bridges      bool `json:"bridges"`
	Wallets      bool `json:"wallets"` // ZKas access (legacy field retained)
	KaspaWallets bool `json:"kaspa_wallets"`
	RemoteLock   bool `json:"remote_lock"`
	FVK          bool `json:"fvk"`
	Payments     bool `json:"payments"`
	Spend        bool `json:"spend"`
}
type WatchWallet struct{ ID, Name, FVK, Token, Address string }
type Config struct {
	KeepRunning bool // false means stop on close, including older configurations
	Token       string
	OwnerOnly   bool
	OwnerID     int64
	Allowed     []int64
	Permissions Permissions
	Watches     []WatchWallet
	Contacts    []int64
	Alerts      AlertOptions
	Limits      SpendingLimits
}

func (c Config) Allows(id int64) bool {
	if id <= 0 {
		return false
	}
	if id == c.OwnerID {
		return true
	}
	if c.OwnerOnly {
		return false
	}
	for _, v := range c.Allowed {
		if v == id {
			return true
		}
	}
	return false
}
func ParseIDs(s string) ([]int64, error) {
	var out []int64
	seen := map[int64]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' || r == '\t' }) {
		n, e := strconv.ParseInt(f, 10, 64)
		if e != nil || n <= 0 {
			return nil, errors.New("Use positive numeric Telegram user IDs, separated by commas or spaces; not usernames or group IDs")
		}
		if !seen[n] {
			out = append(out, n)
			seen[n] = true
		}
	}
	if len(out) > 32 {
		return nil, errors.New("At most 32 allowed Telegram IDs")
	}
	return out, nil
}

var tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]{20,}$`)

func (c Config) Validate() error {
	if e := c.Limits.Validate(); e != nil {
		return e
	}
	if e := c.Alerts.Validate(); e != nil {
		return e
	}
	if c.Alerts.ReceiveZKas && !c.Permissions.Wallets && !c.Permissions.FVK {
		return errors.New("Enable vault wallet viewing or FVK viewing for ZKas receive notifications")
	}
	if c.Alerts.ReceiveKaspa && !c.Permissions.KaspaWallets {
		return errors.New("Enable vault wallet viewing for Kaspa receive notifications")
	}
	if !tokenPattern.MatchString(c.Token) {
		return errors.New("Enter the bot token from Telegram's @BotFather")
	}
	if c.OwnerID < 0 {
		return errors.New("Owner ID must be a positive Telegram user ID")
	}
	for _, id := range c.Allowed {
		if id <= 0 {
			return errors.New("Allowed IDs must be positive Telegram user IDs")
		}
	}
	if (c.Permissions.Payments && !c.Permissions.Wallets) || (c.Permissions.Spend && !c.Permissions.Wallets && !c.Permissions.KaspaWallets) {
		return errors.New("Enable vault wallet viewing before enabling payment requests")
	}
	return nil
}
func Secret() (string, error) {
	var b [16]byte
	_, e := rand.Read(b[:])
	return hex.EncodeToString(b[:]), e
}

type User struct {
	ID    int64 `json:"id"`
	IsBot bool  `json:"is_bot"`
}
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}
type Message struct {
	ID   int64  `json:"message_id"`
	From *User  `json:"from"`
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
	Date int64  `json:"date"`
}
type Callback struct {
	ID      string   `json:"id"`
	From    *User    `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}
type Update struct {
	Callback *Callback `json:"callback_query"`
	ID       int64     `json:"update_id"`
	Message  *Message  `json:"message"`
}
type Request struct {
	UserID  int64
	Command string
	Args    []string
}
type Handler func(context.Context, Request) (string, error)
type pending struct {
	Code    string
	Request Request
	Expires time.Time
}
type Engine struct {
	WalletAccess func(context.Context, string) (bool, string, error)
	Prepare      func(context.Context, PaymentInput) (PaymentReview, error)
	Touch        func(int64)
	Working      func(int64, string)
	payments     map[int64]*paymentFlow
	Menu         func(context.Context, Request) [][]Choice
	buttons      map[string]menuAction
	Config       Config
	PairCode     string
	PairExpires  time.Time
	Started      time.Time
	Handle       Handler
	Pair         func(int64) error
	pending      map[int64]pending
	unlocks      map[int64]unlockPrompt
	unlockNext   map[int64]time.Time
	DeleteSecret func(int64, int64) error
}

func (e *Engine) permitted(cmd string) bool {
	p := e.Config.Permissions
	switch cmd {
	case "status":
		return p.Status
	case "mining":
		return p.Mining
	case "node", "nodes":
		return p.Nodes
	case "bridge", "bridges":
		return p.Bridges
	case "wallet", "wallets", "balance", "history", "receive", "sync":
		return p.Wallets
	case "fvk", "fvks", "fvk_balance", "fvk_history", "fvk_sync":
		return p.FVK
	case "send":
		return p.Spend && (p.Wallets || p.KaspaWallets)
	case "kaspa", "kaspa_wallets", "kaspa_open", "kaspa_account":
		return p.KaspaWallets
	case "unlock", "lock":
		return p.RemoteLock && (p.Wallets || p.KaspaWallets)
	case "payment":
		return p.Payments && p.Wallets
	}
	return false
}
func (e *Engine) help() string {
	lines := []string{"Use the buttons below, or keep using these optional commands.\n/menu — open the main menu"}
	for _, x := range []struct{ cmd, help string }{
		{"status", "/status — node and backend status"}, {"mining", "/mining — workers, shares and hashrate"},
		{"node", "/node start|stop zkas|kaspa — confirmation required; stopping a node also stops dependent services"},
		{"bridge", "/bridge start zkas|kaspa|merged or /bridge stop — uses saved mining settings"},
		{"wallets", "/wallets — available unlocked ZKas vault wallets\n/balance ID\n/history ID\n/receive ID\n/sync ID"},
		{"unlock", "/unlock zkas|kaspa — unlock only the Telegram session\n/lock zkas|kaspa — lock only the Telegram session"},
		{"kaspa_wallets", "/kaspa_wallets — Telegram Kaspa wallets\n/kaspa — selected Telegram account"},
		{"fvks", "/fvks — separately configured view-only wallets\n/fvk_sync ID\n/fvk_balance ID\n/fvk_history ID"},
		{"send", "/send zkas WALLET_ID or /send kaspa WALLET_ID ACCOUNT_ID — guided payment with in-chat confirmation"},
		{"payment", "/payment ID — request the local ZKas send dialog; approval and payment details stay in the manager"},
	} {
		if e.permitted(x.cmd) {
			lines = append(lines, x.help)
		}
	}
	return strings.Join(lines, "\n\n") + "\n\n/cancel — discard your pending action. Never send recovery phrases or private keys. Vault passwords are accepted only after choosing Unlock when remote locking is enabled."
}

// Process never invokes a handler for an unauthorized sender, a group, or an old update.
func (e *Engine) Process(ctx context.Context, m Message, now time.Time) (Reply, error) {
	if ctx.Err() != nil {
		return Reply{}, ctx.Err()
	}
	if !privateSender(m.From, m.Chat) {
		return Reply{}, nil
	}
	if m.Date < e.Started.Unix() || now.Unix()-m.Date > 120 || m.Date > now.Unix()+30 {
		return Reply{}, nil
	}
	f := strings.Fields(m.Text)
	if len(f) == 0 || len(m.Text) > 2048 {
		return Reply{}, nil
	}
	cmd := strings.ToLower(strings.TrimPrefix(strings.SplitN(f[0], "@", 2)[0], "/"))
	id := m.From.ID
	home := func(text string) Reply { return Reply{Text: text, Choices: e.mainMenu()} }
	if cmd == "pair" && e.Config.OwnerID == 0 && len(f) == 2 && now.Before(e.PairExpires) && e.PairCode != "" && subtle.ConstantTimeCompare([]byte(f[1]), []byte(e.PairCode)) == 1 {
		if e.Pair == nil {
			return Reply{}, nil
		}
		if err := e.Pair(id); err != nil {
			return Reply{Text: "Pairing was not approved or could not be saved. Check the manager."}, nil
		}
		e.Config.OwnerID = id
		e.PairCode = ""
		return home("Paired as owner. Your Telegram user ID: " + strconv.FormatInt(id, 10) + "\nChoose an option below."), nil
	}
	if !e.Config.Allows(id) {
		return Reply{}, nil
	}
	if e.Touch != nil {
		e.Touch(id)
	}
	if reply, handled, err := e.processUnlock(ctx, m, cmd, f, now); handled {
		return reply, err
	}
	if reply, handled, err := e.processPayment(ctx, m, cmd, f, now); handled {
		return reply, err
	}
	if cmd == "start" || cmd == "menu" {
		return home("Choose an option."), nil
	}
	if cmd == "help" {
		return home(e.help()), nil
	}
	if cmd == "cancel" {
		if len(f) > 1 {
			p, ok := e.pending[id]
			if !ok || len(f) != 2 || f[1] != p.Code {
				return home("That request is no longer current."), nil
			}
		}
		delete(e.pending, id)
		return home("Pending action cancelled."), nil
	}
	r := Request{UserID: id, Command: cmd, Args: f[1:]}
	if cmd == "confirm" {
		p, ok := e.pending[id]
		// Typed 'confirm' accepts this user's latest pending request. Buttons carry
		// its hidden ID so a stale button cannot approve a different/newer request.
		if !ok || len(f) > 2 || !now.Before(p.Expires) || (len(f) == 2 && subtle.ConstantTimeCompare([]byte(f[1]), []byte(p.Code)) != 1) {
			return home("No current request to confirm, or it expired. Choose the action again."), nil
		}
		delete(e.pending, id)
		r = p.Request
		if !e.permitted(r.Command) {
			return home("That permission is disabled."), nil
		}
	} else {
		if !e.permitted(cmd) {
			return home("That option is not enabled. Choose an available option below."), nil
		}
		if cmd == "nodes" {
			return Reply{Text: "Node controls. Stopping a node also stops its dependent services.", Choices: e.nodeMenu()}, nil
		}
		if cmd == "bridges" {
			return Reply{Text: "Mining controls use your saved settings. Required nodes must be synced.", Choices: e.bridgeMenu()}, nil
		}
		if cmd == "node" || cmd == "bridge" || cmd == "sync" || cmd == "fvk_sync" || cmd == "payment" {
			code, err := Secret()
			if err != nil {
				return Reply{}, err
			}
			if e.pending == nil {
				e.pending = map[int64]pending{}
			}
			e.pending[id] = pending{code, r, now.Add(2 * time.Minute)}
			return Reply{Text: describe(r) + "\n\nTap Confirm or reply confirm within 2 minutes. Cancel keeps things as they are.", Choices: [][]Choice{{{"Confirm", "confirm " + code}, {"Cancel", "cancel " + code}}}}, nil
		}
	}
	if e.Handle == nil {
		return home("Controller unavailable."), nil
	}
	text, err := e.Handle(ctx, r)
	if err != nil {
		return Reply{}, err
	}
	return Reply{Text: text, Choices: e.resultMenu(ctx, r)}, nil
}

type API struct {
	Trace  func(string)
	token  string
	base   string
	client *http.Client
}

func NewAPI(token string) *API {
	return &API{token: token, base: "https://api.telegram.org", client: &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (a *API) call(ctx context.Context, method string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return errors.New("Could not encode Telegram request")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", a.base+"/bot"+a.token+"/"+method, bytes.NewReader(b))
	if err != nil {
		return errors.New("Could not create Telegram request")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("Telegram connection failed; check internet access")
	}
	defer res.Body.Close()
	var env struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Code        int             `json:"error_code"`
		Description string          `json:"description"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&env); err != nil {
		return errors.New("Telegram returned an invalid response")
	}
	if !env.OK || res.StatusCode != 200 {
		return &apiError{method: method, status: res.StatusCode, code: env.Code, unchanged: strings.Contains(env.Description, "message is not modified"), missing: strings.Contains(env.Description, "message to edit not found") || strings.Contains(env.Description, "message can't be edited")}
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}
func (a *API) Check(ctx context.Context) (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := a.call(ctx, "getMe", map[string]any{}, &me); err != nil {
		return "", err
	}
	var hook struct {
		URL string `json:"url"`
	}
	if err := a.call(ctx, "getWebhookInfo", map[string]any{}, &hook); err != nil {
		return "", err
	}
	if hook.URL != "" {
		return "", errors.New("This token already has a webhook. Use a dedicated bot token; the manager will not take over another bot")
	}
	return me.Username, nil
}
func (a *API) Send(ctx context.Context, id int64, reply Reply, keyboard [][]Button) error {
	text := reply.Text
	r := []rune(text)
	if len(r) > 3500 {
		text = string(r[:3500]) + "\n…More detail is available in the manager."
	}
	keyboard = append(copyButtons(text), keyboard...)
	body := map[string]any{"chat_id": id, "text": text}
	if len(keyboard) > 0 {
		body["reply_markup"] = map[string]any{"inline_keyboard": keyboard}
	}
	// Normal Telegram messages: addresses/history can be copied and forwarded.
	return a.call(ctx, "sendMessage", body, nil)
}
func (a *API) SetupMenu(ctx context.Context) error {
	if err := a.call(ctx, "setMyCommands", map[string]any{"commands": []map[string]string{{"command": "menu", "description": "Open manager menu"}, {"command": "help", "description": "Show optional commands"}}}, nil); err != nil {
		return err
	}
	return a.call(ctx, "setChatMenuButton", map[string]any{"menu_button": map[string]string{"type": "commands"}}, nil)
}

func (a *API) Run(ctx context.Context, e *Engine, status func(string)) error {
	defer a.client.CloseIdleConnections()
	trace := func(message string) {
		if a.Trace != nil {
			a.Trace(message)
		}
	}
	trace("Telegram polling started.")
	e.DeleteSecret = func(id, mid int64) error {
		return a.call(ctx, "deleteMessage", map[string]any{"chat_id": id, "message_id": mid}, nil)
	}
	// Discard queued commands from before startup; never replay old service/payment actions.
	var old []Update
	if err := a.call(ctx, "getUpdates", map[string]any{"offset": -1, "timeout": 0, "allowed_updates": []string{"message", "callback_query"}}, &old); err != nil {
		return err
	}
	activeMenus := map[int64]int64{}
	e.Working = func(id int64, text string) {
		if mid, err := a.Publish(ctx, id, activeMenus[id], Reply{Text: text}, nil); err == nil {
			activeMenus[id] = mid
		}
	}
	var offset int64
	for _, u := range old {
		if u.ID >= offset {
			offset = u.ID + 1
		}
	}
	for ctx.Err() == nil {
		var updates []Update
		err := a.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 25, "allowed_updates": []string{"message", "callback_query"}}, &updates)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			trace("Telegram API request failed; retrying or continuing. Check connection and that no other manager uses this bot token.")
			status(err.Error())
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}
		status("Running — waiting for an allowed private message.")
		for _, u := range updates {
			if u.ID < offset {
				continue
			}
			offset = u.ID + 1
			var id int64
			var reply Reply
			var err error
			if u.Callback != nil {
				cb := *u.Callback
				if cb.Message == nil || !privateSender(cb.From, cb.Message.Chat) || !e.Config.Allows(cb.From.ID) {
					continue
				}
				id = cb.From.ID
				activeMenus[id] = cb.Message.ID
				// Acknowledge before potentially slow RPC/desktop actions.
				if err = a.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": cb.ID}, nil); err != nil {
					trace("Telegram API request failed; retrying or continuing. Check connection and that no other manager uses this bot token.")
					status(err.Error())
				}
				trace("Authorized menu button received.")
				reply, err = e.ProcessCallback(ctx, cb, time.Now())
			} else if u.Message != nil {
				id = u.Message.Chat.ID
				// Typed commands and payment inputs receive a fresh reply at the bottom.
				// Only callbacks edit the message on which their button was clicked.
				if privateSender(u.Message.From, u.Message.Chat) && e.Config.Allows(u.Message.From.ID) {
					activeMenus[id] = 0
				}
				trace("Telegram message received; checking authorization and command.")
				reply, err = e.Process(ctx, *u.Message, time.Now())
			} else {
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				reply = Reply{Text: "Action could not complete. Check the manager for details.", Choices: e.mainMenu()}
				status("A bot action could not complete; review the manager.")
			}
			if reply.Text != "" {
				keyboard, keyErr := e.Keyboard(id, reply.Choices, time.Now())
				if keyErr != nil {
					status("Menu buttons could not be created.")
				}
				mid, publishErr := a.Publish(ctx, id, activeMenus[id], reply, keyboard)
				if publishErr != nil {
					trace("Telegram reply delivery failed.")
					status(publishErr.Error())
				} else {
					activeMenus[id] = mid
					trace("Telegram reply delivered.")
					e.MarkDelivered(id, time.Now())
				}
			}
		}
	}
	return ctx.Err()
}
