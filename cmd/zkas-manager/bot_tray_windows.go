package main

import (
	"github.com/lxn/walk"
)

func (m *manager) botBackground() error {
	if m.botTray == nil {
		tray, err := walk.NewNotifyIcon(m.window)
		if err != nil {
			return err
		}
		icon, err := walk.NewIconFromSysDLL("shell32.dll", 0)
		if err != nil {
			tray.Dispose()
			return err
		}
		if err = tray.SetIcon(icon); err != nil {
			tray.Dispose()
			icon.Dispose()
			return err
		}
		tray.SetToolTip("ZKas — Telegram bot running")
		open := walk.NewAction()
		open.SetText("Open node manager")
		open.Triggered().Attach(func() { m.window.Show(); m.window.SetFocus() })
		quit := walk.NewAction()
		quit.SetText("Stop bot and quit manager")
		quit.Triggered().Attach(func() { m.botExit = true; m.window.Close(); m.botExit = false })
		tray.ContextMenu().Actions().Add(open)
		tray.ContextMenu().Actions().Add(quit)
		tray.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
			if button == walk.LeftButton {
				m.window.Show()
				m.window.SetFocus()
			}
		})
		m.botTray = tray
	}
	if err := m.botTray.SetVisible(true); err != nil {
		return err
	}
	// Closing the desktop window locks only desktop credentials.
	m.lockVault()
	m.kwLock()
	m.window.Hide()
	return nil
}
