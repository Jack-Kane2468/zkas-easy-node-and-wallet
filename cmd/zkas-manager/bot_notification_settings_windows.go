package main

import (
	"context"
	"fmt"
	"zkas-node-manager/internal/telegrambot"
)

func (m *manager) botSaveAlerts(ctx context.Context, id int64, o telegrambot.AlertOptions) error {
	var saveErr error
	err := m.botUIRun(ctx, func() {
		if id != m.botConfig.OwnerID || id <= 0 {
			saveErr = fmt.Errorf("Owner access required")
			return
		}
		next := m.botConfig
		next.Alerts = o
		if saveErr = next.Validate(); saveErr != nil {
			return
		}
		if saveErr = telegrambot.Save(m.root, next); saveErr != nil {
			return
		}
		m.botConfig = next
		m.botBlockAlerts.SetChecked(o.Blocks)
		m.botStopAlerts.SetChecked(o.Stops)
		m.botSyncAlerts.SetChecked(o.Sync)
		m.botMiningDown.SetChecked(o.MiningDown)
		m.botDaily.SetChecked(o.Daily)
		m.botReceiveZKas.SetChecked(o.ReceiveZKas)
		m.botReceiveKaspa.SetChecked(o.ReceiveKaspa)
	})
	if err != nil {
		return err
	}
	return saveErr
}
