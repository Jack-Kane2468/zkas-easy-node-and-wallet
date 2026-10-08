package main

import (
	"encoding/json"
	"fmt"
	"zkas-node-manager/internal/kaspavault"
)

func (m *manager) kwExportReceivingKey() {
	if m.busy || m.kwVault == nil || !m.kwState.Open {
		return
	}
	i := m.kwAddressTable.CurrentIndex()
	if i < 0 || i >= len(m.kwAddressModel.rows) {
		return
	}
	row := m.kwAddressModel.rows[i]
	if row.Branch != "Receive" {
		return
	}
	entry := m.kwVault.Find(m.kwState.Filename)
	if entry == nil {
		return
	}
	values, ok := m.inputDialog("Export receiving-address private key", "Anyone with this key can spend funds sent to this address. Confirm your Kaspa vault password to reveal it.", []inputField{{Label: "Kaspa vault password", Secret: true}})
	if !ok {
		return
	}
	defer clear(values)
	if _, e := kaspavault.Load(m.root, values[0]); e != nil {
		m.walletError(e)
		return
	}
	recovery := entry.Recovery
	if recovery == "" {
		v, ok := m.inputDialog("Receiving-address key", "Enter this wallet’s recovery phrase or original private key. It will be checked against the selected address and saved encrypted in your vault for future exports.", []inputField{{Label: "Recovery phrase or private key", Secret: true}})
		if !ok {
			return
		}
		recovery = v[0]
		clear(v)
	}
	// Bind the response to the captured account and row, even if a stale UI row is supplied.
	account := m.kwState.Selected
	m.kwAction(map[string]any{"op": "export_key", "account": account, "index": row.Index, "address": row.Address, "secret": recovery, "passphrase": entry.Passphrase}, false, func(raw json.RawMessage) {
		var result struct {
			PrivateKey, Address string
			Index               int
		}
		if e := json.Unmarshal(raw, &result); e != nil {
			m.walletError(e)
			return
		}
		if result.Address != row.Address || result.Index != row.Index || len(result.PrivateKey) != 64 {
			m.walletError(fmt.Errorf("Private-key export did not match the selected address"))
			return
		}
		if entry.Recovery == "" {
			entry.Recovery = recovery
			if e := kaspavault.Save(m.root, m.kwVaultPassword, m.kwVault); e != nil {
				entry.Recovery = ""
				m.walletError(e)
				return
			}
		}
		m.showText("Kaspa receiving-address private key", "This is the key for the address below, not a backup of the entire wallet. Keep it private. Copying it places it on the system clipboard.", fmt.Sprintf("Receiving index: %d\r\nAddress: %s\r\n\r\nPrivate key (hex):\r\n%s", row.Index, row.Address, result.PrivateKey), false)
		result.PrivateKey = ""
	})
}
