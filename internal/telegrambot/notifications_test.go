package telegrambot

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNotificationSettingsSaveAndPermissions(t *testing.T) {
	e, _, now := fixture()
	e.Config.OwnerID = 42
	e.Config.Alerts = AlertOptions{SummaryTime: "09:00", MinWorkers: 1, MiningDelayMinutes: 3}
	e.Config.Permissions.Wallets = true
	e.Config.Permissions.KaspaWallets = true
	saved := e.Config.Alerts
	calls := 0
	e.SaveAlerts = func(_ context.Context, id int64, o AlertOptions) error {
		if id != 42 {
			t.Fatal(id)
		}
		saved = o
		calls++
		return nil
	}
	for _, key := range []string{"blocks", "stops", "sync", "mining", "zkas", "kaspa", "daily"} {
		r, err := e.Process(context.Background(), msg(42, "/notifications "+key+" on", now), now)
		if err != nil || !strings.Contains(r.Text, "Saved.") {
			t.Fatal(key, r, err)
		}
	}
	if calls != 7 || !saved.Blocks || !saved.Stops || !saved.Sync || !saved.MiningDown || !saved.ReceiveZKas || !saved.ReceiveKaspa || !saved.Daily {
		t.Fatal(saved, calls)
	}
	// Replaying an old explicit 'on' command cannot toggle the setting off.
	e.Process(context.Background(), msg(42, "/notifications daily on", now), now)
	if !e.Config.Alerts.Daily {
		t.Fatal("stale command reversed setting")
	}
	e.Process(context.Background(), msg(42, "/notifications daily off", now), now)
	if saved.Daily || e.Config.Alerts.Daily {
		t.Fatal("disable not saved")
	}
	before := e.Config.Alerts
	e.SaveAlerts = func(context.Context, int64, AlertOptions) error { return errors.New("disk full") }
	r, _ := e.Process(context.Background(), msg(42, "/notifications daily on", now), now)
	if e.Config.Alerts != before || !strings.Contains(r.Text, "no changes made") {
		t.Fatal(r, e.Config.Alerts)
	}
	e.Config.OwnerOnly = false
	e.Config.Allowed = []int64{99}
	r, _ = e.Process(context.Background(), msg(99, "/notifications blocks off", now), now)
	if !strings.Contains(r.Text, "Only the paired owner") || e.Config.Alerts != before {
		t.Fatal(r)
	}
}
func TestNotificationAccessAndValidation(t *testing.T) {
	c := Config{Alerts: AlertOptions{SummaryTime: "09:00", MinWorkers: 1, MiningDelayMinutes: 3}}
	for _, key := range []string{"zkas", "kaspa", "unknown"} {
		if _, e := SetNotification(c, key, true); e == nil {
			t.Fatal(key)
		}
	}
	c.Alerts.SummaryTime = "invalid"
	if _, e := SetNotification(c, "daily", true); e == nil {
		t.Fatal("invalid schedule accepted")
	}
	c.Alerts.MiningDelayMinutes = 0
	if _, e := SetNotification(c, "mining", true); e == nil {
		t.Fatal("invalid threshold accepted")
	}
}
func TestNotificationButtonsAndSenderBinding(t *testing.T) {
	e, _, now := fixture()
	e.Config.OwnerID = 42
	e.Config.Alerts = AlertOptions{SummaryTime: "09:00"}
	calls := 0
	e.SaveAlerts = func(context.Context, int64, AlertOptions) error { calls++; return nil }
	reply, err := e.Process(context.Background(), msg(42, "/notifications", now), now)
	if err != nil || len(reply.Choices) != 8 {
		t.Fatal(reply, err)
	}
	keys, err := e.Keyboard(42, reply.Choices, now)
	if err != nil {
		t.Fatal(err)
	}
	e.Config.OwnerOnly = false
	e.Config.Allowed = []int64{99}
	e.ProcessCallback(context.Background(), callback(99, keys[0][0].Data), now)
	if calls != 0 {
		t.Fatal("another user used owner's button")
	}
	r, err := e.ProcessCallback(context.Background(), callback(42, keys[0][0].Data), now)
	if err != nil || calls != 1 || !e.Config.Alerts.Blocks || !strings.Contains(r.Choices[0][0].Text, "ON") {
		t.Fatal(r, err, calls)
	}
}
