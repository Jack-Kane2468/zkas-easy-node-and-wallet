package main

import (
	"context"
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"strconv"
	"time"
	"zkas-node-manager/internal/wallet"
)

type selectedHistoryModel struct{ viewHistoryModel }

func (m *selectedHistoryModel) Value(row, col int) interface{} {
	r := m.rows[row]
	if col == 1 && r.Timestamp == 0 {
		return "Unknown"
	}
	if col == 6 && r.FeeKnown {
		if _, e := strconv.ParseUint(r.Fee, 10, 64); e != nil {
			return "Unknown"
		}
	}
	return m.viewHistoryModel.Value(row, col)
}

func (m *manager) showWalletHistory() {
	if !m.requireVault() {
		return
	}
	w := m.activeWallet()
	if w == nil {
		return
	}
	// Capture only this profile's public identity and API token, never its seed.
	name, address := w.Name, w.Address
	client := wallet.Client{Port: m.cfg.WalletPort, Token: w.Token}
	var dialog *walk.Dialog
	var status *walk.TextLabel
	var table *walk.TableView
	var details *walk.TextEdit
	var previous, next, refresh, closeButton *walk.PushButton
	model := &selectedHistoryModel{}
	offset, total := 0, 0
	back := []int{}
	loading, closed := false, false
	var cancel context.CancelFunc
	var load func()
	controls := func() {
		previous.SetEnabled(!loading && len(back) > 0)
		next.SetEnabled(!loading && len(model.rows) > 0 && offset+len(model.rows) < total)
		refresh.SetEnabled(!loading)
	}
	showRow := func() {
		if table == nil || details == nil {
			return
		}
		i := table.CurrentIndex()
		if i < 0 || i >= len(model.rows) {
			details.SetText("")
			return
		}
		r := model.rows[i]
		details.SetText(fmt.Sprintf("Type: %s\r\nTime: %v\r\nAmount: %v ZKAS\r\nAddress: %s\r\nTransaction ID: %s\r\nFee (when known): %v\r\nMemo: %s", r.Kind, model.Value(i, 1), model.Value(i, 2), r.Recipient, r.TxID, model.Value(i, 6), r.Memo))
	}
	load = func() {
		if loading || closed {
			return
		}
		loading = true
		controls()
		model.rows = nil
		model.PublishRowsReset()
		details.SetText("")
		status.SetText("Loading history for " + name + "…")
		requestedOffset := offset
		ctx, stop := context.WithTimeout(context.Background(), 45*time.Second)
		cancel = stop
		go func() {
			s, page, err := client.SelectedHistory(ctx, address, requestedOffset)
			stop()
			m.window.Synchronize(func() {
				if closed {
					return
				}
				loading = false
				if err != nil {
					total = 0
					status.SetText(err.Error())
					controls()
					return
				}
				model.rows = page.Rows
				total = page.Total
				model.PublishRowsReset()
				text := fmt.Sprintf("%s · %d entries reported", name, total)
				if len(page.Rows) > 0 {
					text += fmt.Sprintf(" · showing %d–%d", offset+1, offset+len(page.Rows))
				} else {
					text += " · no entries on this page"
				}
				if s.MissingHistory {
					text += "\nHistory is incomplete: the node cannot supply all required blocks."
				} else if !s.Synced {
					text += "\nWallet scan is still running. Refresh as it progresses."
				}
				if !page.Recoverable {
					text += "\nRecoverable outgoing history is disabled; some outgoing details may be unavailable."
				}
				status.SetText(text)
				controls()
				if len(page.Rows) > 0 {
					table.SetCurrentIndex(0)
					showRow()
				}
			})
		}()
	}
	err := (Dialog{AssignTo: &dialog, Title: "Wallet history — " + name, Size: Size{Width: 840, Height: 540}, MinSize: Size{Width: 480, Height: 360}, Layout: VBox{}, CancelButton: &closeButton, Children: []Widget{
		TextLabel{Text: "Selected wallet: " + name + "\n" + address, MinSize: Size{Width: 300}, Font: Font{Bold: true}},
		TextLabel{AssignTo: &status, Text: "Loading…", MinSize: Size{Width: 300}},
		TableView{AssignTo: &table, Model: model, MinSize: Size{Height: 140}, OnCurrentIndexChanged: showRow, Columns: []TableViewColumn{{Title: "Type", Width: 90}, {Title: "Time (local)", Width: 140}, {Title: "Amount ZKAS", Width: 120}, {Title: "Address", Width: 200}, {Title: "Transaction ID", Width: 220}, {Title: "Memo", Width: 180}, {Title: "Fee ZKAS", Width: 100}}},
		TextEdit{AssignTo: &details, ReadOnly: true, VScroll: true, HScroll: true, MinSize: Size{Height: 95}},
		TextLabel{Text: "Memo = an optional encrypted message attached to a payment. Empty means no memo was returned. Select an entry above to read its full details. Pages contain up to 500 entries; Refresh returns to the newest page.", MinSize: Size{Width: 300}},
		Composite{Layout: HBox{}, Children: []Widget{
			PushButton{AssignTo: &previous, Text: "Previous page", OnClicked: func() {
				if loading || len(back) == 0 {
					return
				}
				offset = back[len(back)-1]
				back = back[:len(back)-1]
				load()
			}},
			PushButton{AssignTo: &next, Text: "Next page", OnClicked: func() {
				if loading || len(model.rows) == 0 {
					return
				}
				back = append(back, offset)
				offset += len(model.rows)
				load()
			}},
			PushButton{AssignTo: &refresh, Text: "Refresh", OnClicked: func() {
				if loading {
					return
				}
				offset = 0
				back = nil
				load()
			}},
			HSpacer{}, PushButton{AssignTo: &closeButton, Text: "Close", OnClicked: func() { dialog.Cancel() }},
		}},
	}}).Create(m.window)
	if err != nil {
		if dialog != nil {
			dialog.Dispose()
		}
		m.walletError(err)
		return
	}
	defer dialog.Dispose()
	dialog.Closing().Attach(func(_ *bool, _ walk.CloseReason) {
		closed = true
		if cancel != nil {
			cancel()
		}
	})
	preventWheelChanges(dialog.Handle())
	load()
	dialog.Run()
	closed = true
	if cancel != nil {
		cancel()
	}
}
