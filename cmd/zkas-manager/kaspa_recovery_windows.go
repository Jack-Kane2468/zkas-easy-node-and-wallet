package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"zkas-node-manager/internal/kaspavault"
)

func (m *manager) kwExportRecovery() {
	if m.busy || m.kwVault == nil || !m.kwState.Open {
		return
	}
	entry := m.kwVault.Find(m.kwState.Filename)
	if entry == nil {
		return
	}
	for _, a := range m.kwState.Accounts {
		if a.ID == m.kwState.Selected && strings.Contains(a.Kind, "keypair") {
			m.walletError(fmt.Errorf("This wallet was imported from a private key and has no seed phrase. Select its receiving address and use Export selected receiving key."))
			return
		}
	}
	v, ok := m.inputDialog("Export Kaspa recovery phrase", "Confirm your Kaspa vault password to reveal this wallet’s recovery phrase.", []inputField{{Label: "Kaspa vault password", Secret: true}})
	if !ok {
		return
	}
	defer clear(v)
	if _, e := kaspavault.Load(m.root, v[0]); e != nil {
		m.walletError(e)
		return
	}
	recovery := entry.Recovery
	if recovery == "" {
		fields, ok := m.inputDialog("Save recovery phrase", "This wallet’s phrase was not saved in the vault by the older manager. Enter your original backup once. It will be verified against this wallet before being saved encrypted for future exports.", []inputField{{Label: "Original recovery phrase", Secret: true}})
		if !ok {
			return
		}
		recovery = strings.Join(strings.Fields(fields[0]), " ")
		clear(fields)
	}
	m.kwAction(map[string]any{"op": "verify_recovery", "account": m.kwState.Selected, "secret": recovery, "passphrase": entry.Passphrase}, false, func(raw json.RawMessage) {
		var result struct{ Verified bool }
		if json.Unmarshal(raw, &result) != nil || !result.Verified {
			m.walletError(fmt.Errorf("Recovery phrase verification failed"))
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
		text := recovery
		note := "Anyone with this phrase can spend this wallet’s funds. Keep it private. Copying places it on the system clipboard."
		if entry.Passphrase != "" {
			text += "\r\n\r\nBIP39 passphrase (also required to restore):\r\n" + entry.Passphrase
		}
		for _, a := range m.kwState.Accounts {
			if a.ID == m.kwState.Selected {
				text += fmt.Sprintf("\r\n\r\nSelected account number: %d", a.AccountIndex)
			}
		}
		m.showText("Kaspa recovery phrase", note, text, false)
	})
}
