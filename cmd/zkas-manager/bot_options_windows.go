package main

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"strings"
	"zkas-node-manager/internal/telegrambot"
)

func (m *manager) botExtraSettings() Widget {
	return Composite{Layout: VBox{Spacing: 10}, Children: []Widget{
		GroupBox{Title: "Mining-down alerts (optional)", Layout: VBox{}, Children: []Widget{
			CheckBox{AssignTo: &m.botMiningDown, Text: "Alert when mining stays down or below these targets"},
			Label{Text: "Minimum recently active workers"}, NumberEdit{AssignTo: &m.botMinWorkers, Value: float64(1), MinValue: 1, MaxValue: 100000, Decimals: 0},
			Label{Text: "Minimum estimated hashrate (GH/s; 0 disables this check)"}, NumberEdit{AssignTo: &m.botMinHash, MinValue: 0, MaxValue: 1000000000000, Decimals: 3},
			Label{Text: "Alert after this many minutes; also allows time to start mining"}, NumberEdit{AssignTo: &m.botMiningDelay, Value: float64(3), MinValue: 1, MaxValue: 60, Decimals: 0},
			TextLabel{Text: "Checks bridge stops, statistics outages, worker count and estimated hashrate. Sends a recovery notice too. Workers mean recent share activity, not live TCP connections. Intentional manager stops are ignored.", MinSize: Size{Width: 240}},
		}},
		GroupBox{Title: "Daily Telegram summary (optional)", Layout: VBox{}, Children: []Widget{
			CheckBox{AssignTo: &m.botDaily, Text: "Send one daily summary"},
			Label{Text: "Delivery time — this computer's local time (24-hour HH:MM)"}, LineEdit{AssignTo: &m.botSummaryTime, Text: "09:00"},
			TextLabel{Text: "Reports observed node uptime, average worker count / estimated hashrate, and bridge-reported blocks. Includes incoming-transaction counts when receive alerts are enabled. Coverage gaps are shown; the bot must stay running. First delivery is at the next scheduled time.", MinSize: Size{Width: 240}},
		}},
		GroupBox{Title: "Receive-payment notifications (optional)", Layout: VBox{}, Children: []Widget{
			CheckBox{AssignTo: &m.botReceiveZKas, Text: "Notify for new ZKas receipts (vault wallets and/or configured FVKs)"},
			CheckBox{AssignTo: &m.botReceiveKaspa, Text: "Notify for new Kaspa receipts in the open wallet/account"},
			TextLabel{Text: "Requires the corresponding wallet-viewing permission. Only available, synced history is reported. Checking status may wake registered ZKas backend wallets. Shows amount, wallet, transaction, receiving addresses and ZKas memo when reported. Existing history is baselined; wallets are never unlocked or switched in the background. Confirmation counts are not reported by these history APIs.", MinSize: Size{Width: 240}},
		}},
		GroupBox{Title: "Telegram spending limits (optional)", Layout: VBox{}, Children: []Widget{
			CheckBox{AssignTo: &m.botLimits, Text: "Enforce spending limits for Telegram sends"},
			Label{Text: "ZKAS — maximum per payment (including fee)"}, LineEdit{AssignTo: &m.botLimitZTx},
			Label{Text: "ZKAS — maximum over a rolling 24 hours (including fees)"}, LineEdit{AssignTo: &m.botLimitZDay},
			Label{Text: "KAS — maximum per payment (including fee)"}, LineEdit{AssignTo: &m.botLimitKTx},
			Label{Text: "KAS — maximum over a rolling 24 hours (including fees)"}, LineEdit{AssignTo: &m.botLimitKDay},
			TextLabel{Text: "Blank means no cap for that field. Limits are shared across all Telegram users and wallets on this manager, separately per coin; desktop sends are unaffected. Totals survive restarts. Confirmed attempts reserve allowance before signing; failed or uncertain attempts remain counted for 24 hours. Disabling limits does not erase reservations; sends made while disabled are not counted.", MinSize: Size{Width: 240}},
		}},
		TextLabel{AssignTo: &m.botMonitorState, Text: "Optional notifications: stopped.", MinSize: Size{Width: 240}},
	}}
}
func (m *manager) botInitExtra(c telegrambot.Config) {
	o := c.Alerts
	m.botMiningDown.SetChecked(o.MiningDown)
	m.botDaily.SetChecked(o.Daily)
	m.botReceiveZKas.SetChecked(o.ReceiveZKas)
	m.botReceiveKaspa.SetChecked(o.ReceiveKaspa)
	if o.MinWorkers > 0 {
		m.botMinWorkers.SetValue(float64(o.MinWorkers))
	}
	m.botMinHash.SetValue(o.MinHashrate)
	if o.MiningDelayMinutes > 0 {
		m.botMiningDelay.SetValue(float64(o.MiningDelayMinutes))
	}
	if o.SummaryTime != "" {
		m.botSummaryTime.SetText(o.SummaryTime)
	}
	m.botLimits.SetChecked(c.Limits.Enabled)
	m.botLimitZTx.SetText(c.Limits.ZKas.Transaction)
	m.botLimitZDay.SetText(c.Limits.ZKas.Daily)
	m.botLimitKTx.SetText(c.Limits.Kaspa.Transaction)
	m.botLimitKDay.SetText(c.Limits.Kaspa.Daily)
}
func (m *manager) botReadExtra(c *telegrambot.Config) {
	c.Alerts.MiningDown = m.botMiningDown.Checked()
	c.Alerts.MinWorkers = int(m.botMinWorkers.Value())
	c.Alerts.MinHashrate = m.botMinHash.Value()
	c.Alerts.MiningDelayMinutes = int(m.botMiningDelay.Value())
	c.Alerts.Daily = m.botDaily.Checked()
	c.Alerts.SummaryTime = strings.TrimSpace(m.botSummaryTime.Text())
	c.Alerts.ReceiveZKas = m.botReceiveZKas.Checked()
	c.Alerts.ReceiveKaspa = m.botReceiveKaspa.Checked()
	c.Limits = telegrambot.SpendingLimits{Enabled: m.botLimits.Checked(), ZKas: telegrambot.ChainLimit{Transaction: strings.TrimSpace(m.botLimitZTx.Text()), Daily: strings.TrimSpace(m.botLimitZDay.Text())}, Kaspa: telegrambot.ChainLimit{Transaction: strings.TrimSpace(m.botLimitKTx.Text()), Daily: strings.TrimSpace(m.botLimitKDay.Text())}}
}
func (m *manager) botExtraEnable(edit bool) {
	for _, w := range []*walk.CheckBox{m.botMiningDown, m.botDaily, m.botReceiveZKas, m.botReceiveKaspa, m.botLimits} {
		w.SetEnabled(edit)
	}
	for _, w := range []*walk.LineEdit{m.botSummaryTime, m.botLimitZTx, m.botLimitZDay, m.botLimitKTx, m.botLimitKDay} {
		w.SetEnabled(edit)
	}
	for _, w := range []*walk.NumberEdit{m.botMinWorkers, m.botMinHash, m.botMiningDelay} {
		w.SetEnabled(edit)
	}
}
