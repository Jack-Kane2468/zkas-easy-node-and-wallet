package main

import (
	"encoding/json"
	"fmt"
	"github.com/lxn/walk"
	"strconv"
	"strings"
	"zkas-node-manager/internal/wallet"
)

type kaspaAddressRow struct {
	Branch, Address, Amount string
	Index, Count            int
	Known                   bool
	Utxos                   []struct {
		TxID, Amount, DAA string
		Index             int
		Coinbase          bool
	}
}
type kaspaAddressPage struct {
	Rows          []kaspaAddressRow
	Offset, Total int
	Online        bool
	CheckedAt     string
}
type kaspaAddressModel struct {
	walk.TableModelBase
	rows []kaspaAddressRow
}

func (m *kaspaAddressModel) RowCount() int { return len(m.rows) }
func (m *kaspaAddressModel) Value(row, col int) interface{} {
	r := m.rows[row]
	switch col {
	case 0:
		return r.Branch
	case 1:
		return r.Index
	case 2:
		return r.Address
	case 3:
		if !r.Known {
			return "Not checked"
		}
		n, e := strconv.ParseUint(r.Amount, 10, 64)
		if e != nil {
			return "Unavailable"
		}
		return wallet.FormatAmount(n)
	case 4:
		if !r.Known {
			return "—"
		}
		return r.Count
	}
	return ""
}
func (m *manager) kwClearAddresses() {
	if m.kwAddressModel == nil {
		return
	}
	m.kwAddressModel.rows = nil
	m.kwAddressModel.PublishRowsReset()
	m.kwAddressOffset = 0
	m.kwAddressTotal = 0
	m.kwAddressStatus.SetText("Refresh addresses to load indexes and current UTXOs.")
	m.kwAddressDetails.SetText("")
}
func (m *manager) kwLoadAddresses(offset int) {
	if m.busy || !m.kwState.Open {
		return
	}
	if offset < 0 {
		offset = 0
	}
	m.kwAction(map[string]any{"op": "addresses", "offset": offset}, false, func(raw json.RawMessage) {
		var page kaspaAddressPage
		if e := json.Unmarshal(raw, &page); e != nil {
			m.walletError(e)
			return
		}
		m.kwApplyAddressPage(page)
	})
}
func (m *manager) kwApplyAddressPage(page kaspaAddressPage) {
	m.kwAddressOffset = page.Offset
	m.kwAddressTotal = page.Total
	m.kwAddressModel.rows = page.Rows
	m.kwAddressModel.PublishRowsReset()
	text := fmt.Sprintf("Addresses %d–%d of %d · receiving and change addresses", page.Offset+1, page.Offset+len(page.Rows), page.Total)
	if page.Online {
		text += "\r\nUTXO snapshot: " + page.CheckedAt
	} else {
		text += "\r\nUTXOs not checked. Connect to a synced local node, then refresh."
	}
	m.kwAddressStatus.SetText(text)
	m.kwAddressDetails.SetText("")
	m.kwControls()
}
func (m *manager) kwSelectAddress() {
	if m.kwAddressTable == nil || m.kwAddressDetails == nil {
		return
	}
	i := m.kwAddressTable.CurrentIndex()
	if i < 0 || i >= len(m.kwAddressModel.rows) {
		m.kwAddressDetails.SetText("")
		return
	}
	r := m.kwAddressModel.rows[i]
	var b strings.Builder
	fmt.Fprintf(&b, "%s address · index %d\r\n%s\r\n", r.Branch, r.Index, r.Address)
	if !r.Known {
		b.WriteString("UTXOs not checked. Connect to a synced local node and refresh.")
	} else {
		fmt.Fprintf(&b, "Unspent: %v KAS · %d UTXOs\r\n", m.kwAddressModel.Value(i, 3), r.Count)
		if r.Count == 0 {
			b.WriteString("No current UTXOs at this address. This does not mean the address was never used.\r\n")
		}
		for _, u := range r.Utxos {
			n, e := strconv.ParseUint(u.Amount, 10, 64)
			amount := "Unavailable"
			if e == nil {
				amount = wallet.FormatAmount(n)
			}
			fmt.Fprintf(&b, "\r\n%s:%d\r\n%s KAS · block DAA %s · mining reward: %t\r\n", u.TxID, u.Index, amount, u.DAA, u.Coinbase)
		}
		if r.Count > len(r.Utxos) {
			fmt.Fprintf(&b, "\r\nShowing the first %d outputs; the total includes all %d.\r\n", len(r.Utxos), r.Count)
		}
	}
	m.kwAddressDetails.SetText(b.String())
	m.kwControls()
}
