package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"zkas-node-manager/internal/telegrambot"
	"zkas-node-manager/internal/wallet"
)

type botUI struct {
	botSession
	botTray                                                              *walk.NotifyIcon
	botExit                                                              bool
	botShutdown                                                          *walk.CheckBox
	botMonitorMu                                                         sync.Mutex
	botMiningDown, botDaily, botReceiveZKas, botReceiveKaspa, botLimits  *walk.CheckBox
	botMinWorkers, botMinHash, botMiningDelay                            *walk.NumberEdit
	botSummaryTime, botLimitZTx, botLimitZDay, botLimitKTx, botLimitKDay *walk.LineEdit
	botMonitorState                                                      *walk.TextLabel
	botBlockAlerts, botStopAlerts, botSyncAlerts                         *walk.CheckBox
	botStallMinutes                                                      *walk.NumberEdit
	botCopyPair                                                          *walk.PushButton
	botPairCode                                                          string
	botAdd, botRemove                                                    *walk.PushButton
	botConfig                                                            telegrambot.Config
	botToken, botOwner, botIDs                                           *walk.LineEdit
	botOwnerOnly                                                         *walk.CheckBox
	botPermissions                                                       []*walk.CheckBox
	botState, botPairLabel                                               *walk.TextLabel
	botWatchList                                                         *walk.ComboBox
	botStart, botStop                                                    *walk.PushButton
	botCancel                                                            context.CancelFunc
	botGeneration                                                        uint64
}

func (m *manager) botTab() TabPage {
	m.botPermissions = make([]*walk.CheckBox, 10)
	labels := []string{"Node / backend status", "Mining dashboard statistics", "Start / stop ZKas and Kaspa nodes", "Start / stop mining bridges", "Allow ZKas wallet access", "View / sync separately added FVK wallets", "Request a payment for local review", "Send from Telegram (unlocked wallets)", "Allow Kaspa wallet access", "Allow vault / wallet unlock and lock from Telegram"}
	checks := []Widget{}
	for i, s := range labels {
		checks = append(checks, CheckBox{AssignTo: &m.botPermissions[i], Text: s})
	}
	return TabPage{Title: "Bot", Layout: VBox{MarginsZero: true}, Children: []Widget{ScrollView{Layout: VBox{Spacing: 10}, Children: []Widget{
		Label{Text: "Your Telegram bot", Font: Font{PointSize: 14, Bold: true}},
		TextLabel{Text: "Create a dedicated bot with @BotFather in Telegram, paste its token below, choose permissions, then Start bot. Choose whether the bot stops when this window closes below. No router ports need to be opened.", MinSize: Size{Width: 240}},
		GroupBox{Title: "1 · Connect your bot", Layout: VBox{}, Children: []Widget{
			Label{Text: "Bot token"}, LineEdit{AssignTo: &m.botToken, PasswordMode: true},
			PushButton{Text: "Open BotFather", OnClicked: func() { openExternal("https://t.me/BotFather") }},
		}},
		GroupBox{Title: "2 · Who can use it?", Layout: VBox{}, Children: []Widget{
			CheckBox{AssignTo: &m.botOwnerOnly, Text: "Owner only (ignore additional IDs)", Checked: true},
			Label{Text: "Owner Telegram user ID (optional before pairing)"}, LineEdit{AssignTo: &m.botOwner},
			Label{Text: "Additional allowed Telegram user IDs (comma separated)"}, LineEdit{AssignTo: &m.botIDs},
			TextLabel{Text: "Use numeric user IDs, not @usernames. All allowed users receive the permissions selected below. Unknown users and group chats are ignored. Leave owner blank to pair using a one-time code after starting; approve the matching ID here.", MinSize: Size{Width: 240}},
			TextLabel{AssignTo: &m.botPairLabel, Text: "Pairing is available after starting with no owner ID.", MinSize: Size{Width: 240}},
			PushButton{AssignTo: &m.botCopyPair, Text: "Copy pairing command", Enabled: false, OnClicked: func() {
				if m.botPairCode != "" {
					m.copyText("Pairing command", "/pair "+m.botPairCode)
				}
			}},
		}},
		GroupBox{Title: "3 · What may it access?", Layout: VBox{}, Children: checks},
		TextLabel{Text: "Wallet permissions are off by default. For FVK-only access, enable only the FVK option and add viewing keys below; the vault can stay locked. Bot messages are Telegram cloud chats, not end-to-end encrypted. Never paste spending keys or recovery phrases into Telegram.", MinSize: Size{Width: 240}},
		CheckBox{AssignTo: &m.botShutdown, Text: "Stop bot when manager closes", Checked: true},
		TextLabel{Text: "Uncheck to keep the bot running in the system tray after closing this window. Use the tray icon to reopen or quit. Telegram unlocks are separate from desktop unlocks.", MinSize: Size{Width: 240}},
		GroupBox{Title: "Alerts", Layout: VBox{}, Children: []Widget{
			CheckBox{AssignTo: &m.botBlockAlerts, Text: "Mining bridge reports a new block"},
			CheckBox{AssignTo: &m.botStopAlerts, Text: "Node stops unexpectedly (include available reason)"},
			CheckBox{AssignTo: &m.botSyncAlerts, Text: "Node loses sync, RPC stops responding, or chain progress stalls"},
			Label{Text: "No chain progress threshold (minutes)"}, NumberEdit{AssignTo: &m.botStallMinutes, MinValue: 1, MaxValue: 60, Value: float64(5), Decimals: 0},
			TextLabel{Text: "Alerts run while the bot is running, including in background mode. Each recipient must first open a private chat with the bot. A block report is not proof of a payout. Intentional manager stops do not trigger stopped-node alerts.", MinSize: Size{Width: 240}},
		}},
		Composite{Layout: HBox{}, Children: []Widget{
			PushButton{Text: "Unlock ZKas for bot…", OnClicked: func() { m.botLocalUnlock("zkas") }},
			PushButton{Text: "Unlock Kaspa for bot…", OnClicked: func() { m.botLocalUnlock("kaspa") }},
			PushButton{Text: "Lock bot wallets", OnClicked: func() {
				if !m.busy {
					m.botLockSessions()
				}
			}},
		}},
		m.botExtraSettings(),
		GroupBox{Title: "FVK-only wallets", Layout: VBox{}, Children: []Widget{
			ComboBox{AssignTo: &m.botWatchList},
			Composite{Layout: HBox{}, Children: []Widget{
				PushButton{AssignTo: &m.botAdd, Text: "Add FVK wallet…", OnClicked: m.botAddFVK},
				PushButton{AssignTo: &m.botRemove, Text: "Remove selected FVK", OnClicked: m.botRemoveFVK},
			}},
			TextLabel{Text: "Keys are entered here, encrypted for this Windows user, and never sent to Telegram. Use /fvk_sync in the bot to register and scan one on your local wallet backend. Removing it here revokes bot access; its backend scan registration remains.", MinSize: Size{Width: 240}},
		}},
		Composite{Layout: HBox{}, Children: []Widget{
			PushButton{AssignTo: &m.botStart, Text: "Save and start bot", OnClicked: m.botStartClicked},
			PushButton{AssignTo: &m.botStop, Text: "Stop bot / edit settings", Enabled: false, OnClicked: m.botStopClicked},
		}},
		TextLabel{AssignTo: &m.botState, Text: "Stopped — no Telegram access.", MinSize: Size{Width: 240}},
		TextLabel{Text: "Use the bot menus, or /help for optional commands. Service changes use Confirm / Cancel buttons or a simple confirm reply. Stopping a node also stops its dependent mining, sharing and wallet services. Telegram has its own vault lock state. This bot does not export private keys, change sharing rules, install updates or uninstall components.", MinSize: Size{Width: 240}},
	}}}}
}
func (m *manager) initBot() {
	c, err := telegrambot.Load(m.root)
	if err != nil {
		m.botState.SetText("Cannot load saved bot settings: " + err.Error())
		return
	}
	m.botConfig = c
	m.botShutdown.SetChecked(!c.KeepRunning)
	m.botInitExtra(c)
	m.botToken.SetText(c.Token)
	m.botOwnerOnly.SetChecked(c.OwnerOnly)
	if c.OwnerID > 0 {
		m.botOwner.SetText(strconv.FormatInt(c.OwnerID, 10))
	}
	var ids []string
	for _, id := range c.Allowed {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	m.botIDs.SetText(strings.Join(ids, ", "))
	p := c.Permissions
	values := []bool{p.Status, p.Mining, p.Nodes, p.Bridges, p.Wallets, p.FVK, p.Payments, p.Spend, p.KaspaWallets, p.RemoteLock}
	for i, b := range values {
		m.botPermissions[i].SetChecked(b)
	}
	m.botBlockAlerts.SetChecked(c.Alerts.Blocks)
	m.botStopAlerts.SetChecked(c.Alerts.Stops)
	m.botSyncAlerts.SetChecked(c.Alerts.Sync)
	if c.Alerts.StallMinutes > 0 {
		m.botStallMinutes.SetValue(float64(c.Alerts.StallMinutes))
	}
	m.botRenderWatches()
}
func (m *manager) botRenderWatches() {
	names := []string{}
	for _, w := range m.botConfig.Watches {
		names = append(names, w.Name)
	}
	m.botWatchList.SetModel(names)
	if len(names) > 0 {
		m.botWatchList.SetCurrentIndex(0)
	}
}
func (m *manager) botAddFVK() {
	if m.botCancel != nil || m.busy {
		return
	}
	if len(m.botConfig.Watches) >= 16 {
		m.walletError(fmt.Errorf("At most 16 FVK wallets"))
		return
	}
	v, ok := m.inputDialog("Add bot viewing wallet", "Enter an FVK, never a spending key. This grants viewing access only. It will scan only when explicitly requested.", []inputField{{"Name", "", false}, {"Full viewing key (FVK)", "", true}})
	if !ok {
		return
	}
	name, fvk := strings.TrimSpace(v[0]), strings.TrimSpace(v[1])
	if name == "" || len(name) > 80 {
		m.walletError(fmt.Errorf("Enter a wallet name up to 80 characters"))
		return
	}
	clean, err := wallet.CleanHex(fvk, 96)
	if err != nil {
		m.walletError(fmt.Errorf("Enter a 96-byte ZKas FVK in hexadecimal"))
		return
	}
	for _, w := range m.botConfig.Watches {
		if w.FVK == clean {
			m.walletError(fmt.Errorf("This FVK is already added"))
			return
		}
	}
	id, err := telegrambot.Secret()
	if err != nil {
		m.walletError(err)
		return
	}
	m.botConfig.Watches = append(m.botConfig.Watches, telegrambot.WatchWallet{ID: id, Name: name, FVK: clean, Token: "telegram-view-" + id})
	m.botRenderWatches()
}
func (m *manager) botRemoveFVK() {
	if m.botCancel != nil {
		return
	}
	i := m.botWatchList.CurrentIndex()
	if i < 0 || i >= len(m.botConfig.Watches) {
		return
	}
	m.botConfig.Watches = append(m.botConfig.Watches[:i], m.botConfig.Watches[i+1:]...)
	m.botRenderWatches()
}
func (m *manager) botReadConfig() (telegrambot.Config, error) {
	c := m.botConfig
	c.Token = strings.TrimSpace(m.botToken.Text())
	c.OwnerOnly = m.botOwnerOnly.Checked()
	c.OwnerID = 0
	if s := strings.TrimSpace(m.botOwner.Text()); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 {
			return c, fmt.Errorf("Owner must be a positive numeric Telegram user ID")
		}
		c.OwnerID = n
	}
	var err error
	c.Allowed, err = telegrambot.ParseIDs(m.botIDs.Text())
	if err != nil {
		return c, err
	}
	c.KeepRunning = !m.botShutdown.Checked()
	c.Permissions = telegrambot.Permissions{KaspaWallets: m.botPermissions[8].Checked(), RemoteLock: m.botPermissions[9].Checked(), Status: m.botPermissions[0].Checked(), Mining: m.botPermissions[1].Checked(), Nodes: m.botPermissions[2].Checked(), Bridges: m.botPermissions[3].Checked(), Wallets: m.botPermissions[4].Checked(), FVK: m.botPermissions[5].Checked(), Payments: m.botPermissions[6].Checked(), Spend: m.botPermissions[7].Checked()}
	c.Alerts = telegrambot.AlertOptions{Blocks: m.botBlockAlerts.Checked(), Stops: m.botStopAlerts.Checked(), Sync: m.botSyncAlerts.Checked(), StallMinutes: int(m.botStallMinutes.Value())}
	m.botReadExtra(&c)
	return c, c.Validate()
}
func (m *manager) botEnableEdit(edit bool) {
	m.botShutdown.SetEnabled(edit)
	m.botExtraEnable(edit)
	m.botBlockAlerts.SetEnabled(edit)
	m.botStopAlerts.SetEnabled(edit)
	m.botSyncAlerts.SetEnabled(edit)
	m.botStallMinutes.SetEnabled(edit)
	m.botAdd.SetEnabled(edit)
	m.botRemove.SetEnabled(edit)
	m.botToken.SetEnabled(edit)
	m.botOwner.SetEnabled(edit)
	m.botIDs.SetEnabled(edit)
	m.botOwnerOnly.SetEnabled(edit)
	m.botWatchList.SetEnabled(edit)
	for _, p := range m.botPermissions {
		p.SetEnabled(edit)
	}
	m.botStart.SetEnabled(edit)
	m.botStop.SetEnabled(!edit)
}

// botUIRun cancels queued UI work if the bot is stopped or the manager closes.
func (m *manager) botUIRun(ctx context.Context, f func()) error {
	if ctx.Err() != nil || m.closed.Load() {
		return context.Canceled
	}
	done := make(chan struct{})
	var state atomic.Int32 // 0 queued, 1 running, 2 cancelled before dispatch
	m.window.Synchronize(func() {
		defer close(done)
		if !state.CompareAndSwap(0, 1) {
			return
		}
		if ctx.Err() == nil && !m.closed.Load() {
			f()
		}
	})
	select {
	case <-done:
		return ctx.Err()
	case <-ctx.Done():
		if state.CompareAndSwap(0, 2) {
			return ctx.Err()
		}
		<-done // A started UI callback owns its result until it returns.
		return ctx.Err()
	}
}
func (m *manager) botStartClicked() {
	if m.botCancel != nil || m.busy {
		return
	}
	c, err := m.botReadConfig()
	if err != nil {
		m.walletError(err)
		return
	}
	if err = telegrambot.Save(m.root, c); err != nil {
		m.walletError(err)
		return
	}
	m.botConfig = c
	code, err := telegrambot.Secret()
	if err != nil {
		m.walletError(err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.botCancel = cancel
	m.botGeneration++
	gen := m.botGeneration
	m.botEnableEdit(false)
	m.botState.SetText("Connecting to Telegram…")
	m.botPairCode = ""
	m.botCopyPair.SetEnabled(false)
	m.botPairLabel.SetText("Owner ID is configured.")
	if c.OwnerID == 0 {
		m.botPairCode = code
		m.botCopyPair.SetEnabled(true)
		m.botPairLabel.SetText("After connecting, send this privately to your bot within 10 minutes:\r\n/pair " + code + "\r\nKeep this code private; approve your ID in this manager.")
	}
	go func() {
		m.botLog("INFO", "Bot starting; checking Telegram connection.")
		defer m.botLog("INFO", "Bot stopped.")
		api := telegrambot.NewAPI(c.Token)
		api.Trace = func(event string) { m.botLog("INFO", event) }
		username, err := api.Check(ctx)
		if err == nil {
			m.botUIRun(ctx, func() {

				m.botState.SetText("Connected: @" + username + " — tap Menu, send /menu, or pair first.")
			})
			engine := &telegrambot.Engine{Config: c, PairCode: code, PairExpires: time.Now().Add(10 * time.Minute), Started: time.Now()}
			engine.Pair = func(id int64) error {
				approved := false
				var saveErr error
				err := m.botUIRun(ctx, func() {
					if walk.MsgBox(m.window, "Pair Telegram owner", fmt.Sprintf("Telegram user ID %d supplied the pairing code.\n\nIs this your Telegram account? Approve only if you just sent the code yourself.", id), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
						return
					}
					if ctx.Err() != nil {
						return
					}
					next := m.botConfig
					next.OwnerID = id
					saveErr = telegrambot.Save(m.root, next)
					if saveErr != nil {
						return
					}
					m.botConfig = next
					m.botPairCode = ""
					m.botCopyPair.SetEnabled(false)
					m.botOwner.SetText(strconv.FormatInt(id, 10))
					m.botPairLabel.SetText("Owner paired: " + strconv.FormatInt(id, 10))
					approved = true
				})
				if err != nil {
					return err
				}
				if saveErr != nil {
					return saveErr
				}
				if !approved {
					return fmt.Errorf("Pairing declined")
				}
				return nil
			}
			engine.WalletAccess = m.botWalletAccess
			engine.Prepare = m.botPreparePayment
			engine.Touch = func(id int64) { m.botTouch(ctx, id) }
			go m.botAlerts(ctx, api, c)
			engine.Menu = func(ctx context.Context, r telegrambot.Request) [][]telegrambot.Choice {
				return m.botWalletMenu(ctx, c, r)
			}
			engine.Handle = func(ctx context.Context, r telegrambot.Request) (string, error) {
				m.botLog("INFO", "Authorized manager request started.")
				start := time.Now()
				done := make(chan struct{})
				defer close(done)
				go func() {
					ticker := time.NewTicker(10 * time.Second)
					defer ticker.Stop()
					for {
						select {
						case <-done:
							return
						case <-ctx.Done():
							return
						case <-ticker.C:
							m.botLog("INFO", "Manager request still running.")
						}
					}
				}()
				if (r.Command == "kaspa_open" || r.Command == "kaspa_account") && engine.Working != nil {
					engine.Working(r.UserID, "Opening Kaspa wallet…")
				}
				result, err := m.botHandle(ctx, c, r)
				if chain := telegrambot.WalletChain(r); chain != "" && c.Permissions.Chain(chain) {
					_, desktop, statusErr := m.botWalletAccess(ctx, chain)
					if statusErr == nil {
						result += "\n\n" + desktop
					}
				}
				if err != nil {
					m.botLog("WARN", "Manager request failed; inspect the relevant wallet or service panel.")
				} else {
					m.botLog("INFO", fmt.Sprintf("Manager request returned after %s.", time.Since(start).Round(time.Millisecond)))
				}
				return result, err
			}
			if menuErr := api.SetupMenu(ctx); menuErr != nil {
				m.botUIRun(ctx, func() {
					m.botState.SetText("Connected; Telegram shortcut setup failed. Send /menu to open the buttons.")
				})
			}
			err = api.Run(ctx, engine, func(s string) { m.botUIRun(ctx, func() { m.botState.SetText("@" + username + ": " + s) }) })
		}
		if ctx.Err() != nil {
			return
		}
		m.botUIRun(ctx, func() {
			if m.botGeneration != gen {
				return
			}
			m.botState.SetText("Bot stopped: " + fmt.Sprint(err))
			m.botCancel = nil

			m.botPairCode = ""
			m.botCopyPair.SetEnabled(false)
			m.botPairLabel.SetText("Pairing inactive.")
			m.botEnableEdit(true)
		})
		cancel()
	}()
}
func (m *manager) botStopClicked() {
	if m.busy {
		m.botState.SetText("Wait for the current wallet operation before stopping the bot.")
		return
	}
	if m.botCancel != nil {
		m.botCancel()
		m.botCancel = nil
	}
	m.botGeneration++
	m.botLockSessions()
	m.botEnableEdit(true)
	m.botPairCode = ""
	m.botCopyPair.SetEnabled(false)
	m.botPairLabel.SetText("Pairing inactive.")
	m.botState.SetText("Stopped — no Telegram access. Existing node and mining services continue.")
	m.botMonitorState.SetText("Optional notifications: stopped.")
}
