package telegrambot

import (
	"context"
	"fmt"
	"strings"
)

// Notification settings are shared by this manager, so only its paired owner
// may change them. Commands set an explicit state: stale buttons are idempotent.
func SetNotification(c Config, key string, enabled bool) (AlertOptions, error) {
	o := c.Alerts
	switch key {
	case "blocks":
		o.Blocks = enabled
	case "stops":
		o.Stops = enabled
	case "sync":
		o.Sync = enabled
	case "mining":
		o.MiningDown = enabled
	case "daily":
		o.Daily = enabled
	case "zkas":
		if enabled && !c.Permissions.Wallets && !c.Permissions.FVK {
			return o, fmt.Errorf("Enable ZKas wallet or FVK access in the manager first")
		}
		o.ReceiveZKas = enabled
	case "kaspa":
		if enabled && !c.Permissions.KaspaWallets {
			return o, fmt.Errorf("Enable Kaspa wallet access in the manager first")
		}
		o.ReceiveKaspa = enabled
	default:
		return o, fmt.Errorf("Unknown notification setting")
	}
	if err := o.Validate(); err != nil {
		return c.Alerts, err
	}
	return o, nil
}
func (e *Engine) notificationMenu(note string) Reply {
	o := e.Config.Alerts
	rows := [][]Choice{}
	for _, x := range []struct {
		key, label string
		on         bool
	}{
		{"blocks", "Block alerts", o.Blocks}, {"stops", "Node-stop alerts", o.Stops},
		{"sync", "Sync / RPC alerts", o.Sync}, {"mining", "Mining-down alerts", o.MiningDown},
		{"zkas", "ZKas receive alerts", o.ReceiveZKas}, {"kaspa", "Kaspa receive alerts", o.ReceiveKaspa},
		{"daily", "Daily summary", o.Daily},
	} {
		state, next := "OFF", "on"
		if x.on {
			state, next = "ON", "off"
		}
		rows = append(rows, []Choice{{x.label + ": " + state, "notifications " + x.key + " " + next}})
	}
	text := "Notifications\nTap a setting to switch it on or off. These settings apply to all bot recipients; only the paired owner can change them."
	if o.SummaryTime != "" {
		text += "\nDaily summary: " + o.SummaryTime + " (manager computer's local time)."
	}
	text += "\nThresholds and summary time are set in the manager. Changes are saved immediately and monitoring picks them up on its next cycle."
	if note != "" {
		text += "\n\n" + note
	}
	return Reply{Text: text, Choices: append(rows, homeRow())}
}
func (e *Engine) processNotifications(ctx context.Context, id int64, args []string) (Reply, error) {
	if id != e.Config.OwnerID {
		return Reply{Text: "Only the paired owner can change the manager's notification settings.", Choices: [][]Choice{homeRow()}}, nil
	}
	if len(args) == 0 {
		return e.notificationMenu(""), nil
	}
	if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
		return e.notificationMenu("Choose a toggle below."), nil
	}
	next, err := SetNotification(e.Config, strings.ToLower(args[0]), args[1] == "on")
	if err != nil {
		return e.notificationMenu(err.Error()), nil
	}
	if e.SaveAlerts == nil {
		return e.notificationMenu("Settings could not be saved; no changes made."), nil
	}
	if err = e.SaveAlerts(ctx, id, next); err != nil {
		return e.notificationMenu("Settings could not be saved; no changes made. " + err.Error()), nil
	}
	e.Config.Alerts = next
	return e.notificationMenu("Saved."), nil
}
