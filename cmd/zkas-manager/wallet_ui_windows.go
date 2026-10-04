package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows"
	"zkas-node-manager/internal/wallet"
)

type walletUI struct {
	vault          *wallet.Vault
	vaultPassword  string
	selectedWallet string
	walletPicker   *walk.ComboBox
	walletState    *walk.Label
	walletAddress  *walk.LineEdit
	walletBalance  *walk.TextLabel
	walletHistory  *walk.TextEdit
}

func (m *manager) walletTab() TabPage {
	return TabPage{Title: "Wallet", Layout: VBox{MarginsZero: true}, Children: []Widget{TabWidget{Pages: []TabPage{m.personalWalletPage(), m.walletToolsTab()}}}}
}

func (m *manager) personalWalletPage() TabPage {
	button := func(text string, fn func()) PushButton { return PushButton{Text: text, OnClicked: fn} }
	return TabPage{Title: "My wallets", Layout: VBox{MarginsZero: true}, Children: []Widget{ScrollView{Layout: VBox{Spacing: 8}, Children: []Widget{
		Label{Text: "Your wallets", Font: Font{PointSize: 13, Bold: true}},
		TextLabel{Text: "Keep several wallets here and choose the one you want to use. Your recovery phrase stays on this PC. The wallet API handles syncing and broadcasting.", MinSize: Size{Width: 240}},
		Label{AssignTo: &m.walletState, Text: "Wallet vault is locked."},
		Composite{Layout: HBox{}, Children: []Widget{button("Unlock / set up vault", m.unlockVault), button("Lock all wallets", m.lockVault)}},
		ComboBox{AssignTo: &m.walletPicker, Model: []string{"Unlock the vault first"}, CurrentIndex: 0, OnCurrentIndexChanged: m.chooseWallet},
		Composite{Layout: HBox{}, Children: []Widget{button("Create wallet", func() { m.addWallet(false, false) }), button("Import key / phrase", func() { m.addWallet(true, false) }), button("Import FVK (view only)", func() { m.addWallet(true, true) })}},
		Composite{Layout: HBox{}, Children: []Widget{button("Sign out of this wallet", m.signOutWallet), button("Sign back in", m.signInWallet), button("Remove wallet…", m.removeWallet)}},
		Label{Text: "Receive", Font: Font{Bold: true}},
		LineEdit{AssignTo: &m.walletAddress, ReadOnly: true},
		Composite{Layout: HBox{}, Children: []Widget{button("Copy receive address", func() {
			if w := m.activeWallet(); w != nil {
				m.copyText("Receive address", m.walletAddress.Text())
			}
		}), button("New receiving address", m.newReceiveAddress)}},
		button("Show all receiving addresses", m.showReceiveAddresses),
		TextLabel{Text: "Extra receiving addresses (your canary addresses) all receive into this wallet. They are not separate wallets or theft alarms.", MinSize: Size{Width: 240}},
		TextLabel{AssignTo: &m.walletBalance, Text: "Unlock and select a wallet to view its balance.", MinSize: Size{Width: 240}},
		Composite{Layout: HBox{}, Children: []Widget{button("Refresh / sync", m.syncWallet), button("Show history", m.showWalletHistory), button("Send ZKAS…", m.sendWallet)}},
		TextEdit{AssignTo: &m.walletHistory, ReadOnly: true, VScroll: true, MinSize: Size{Height: 160}, Text: "Transactions and saved send receipts appear here."},
		Label{Text: "Backup and viewing keys", Font: Font{Bold: true}},
		Composite{Layout: HBox{}, Children: []Widget{button("Show recovery backup…", m.showRecovery), button("Export FVK…", func() { m.exportViewKey(false) }), button("Export OVK…", func() { m.exportViewKey(true) })}},
		TextLabel{Text: "FVK: view incoming funds and recoverable outgoing history, without spending. OVK: outgoing viewing key; it cannot show your full balance or authorize payments. Use Wallet tools for independent viewing-key scans.", MinSize: Size{Width: 240}},
	}}}}
}

type inputField struct {
	Label, Value string
	Secret       bool
}

func (m *manager) inputDialog(title, note string, fields []inputField) ([]string, bool) {
	var d *walk.Dialog
	var accept, cancel *walk.PushButton
	var values []string
	edits := make([]*walk.LineEdit, len(fields))
	children := []Widget{TextLabel{Text: note, MinSize: Size{Width: 300}}}
	for i, f := range fields {
		children = append(children, Label{Text: f.Label}, LineEdit{AssignTo: &edits[i], Text: f.Value, PasswordMode: f.Secret})
	}
	children = append(children, Composite{Layout: HBox{}, Children: []Widget{HSpacer{}, PushButton{AssignTo: &accept, Text: "Continue", OnClicked: func() {
		values = make([]string, len(edits))
		for i, edit := range edits {
			values[i] = edit.Text()
		}
		d.Accept()
	}}, PushButton{AssignTo: &cancel, Text: "Cancel", OnClicked: func() { d.Cancel() }}}})
	e := (Dialog{AssignTo: &d, Title: title, MinSize: Size{Width: 420}, Layout: VBox{}, DefaultButton: &accept, CancelButton: &cancel, Children: children}).Create(m.window)
	result := walk.DlgCmdCancel
	if e == nil {
		preventWheelChanges(d.Handle())
		result = d.Run()
	}
	if d != nil {
		defer d.Dispose()
	}
	if e != nil || result != walk.DlgCmdOK {
		return nil, false
	}
	return values, true
}
func (m *manager) showText(title, note, text string, requireBackup bool) bool {
	var d *walk.Dialog
	var ok, cancel *walk.PushButton
	var saved *walk.CheckBox
	children := []Widget{TextLabel{Text: note, MinSize: Size{Width: 300}}, TextEdit{ReadOnly: true, VScroll: true, Text: text, MinSize: Size{Height: 150}}, PushButton{Text: "Copy this text", OnClicked: func() { m.copyText(title, text) }}}
	if requireBackup {
		children = append(children, CheckBox{AssignTo: &saved, Text: "I saved my recovery phrase somewhere safe."})
	}
	children = append(children, Composite{Layout: HBox{}, Children: []Widget{HSpacer{}, PushButton{AssignTo: &ok, Text: "Done", OnClicked: func() {
		if requireBackup && !saved.Checked() {
			walk.MsgBox(d, "Save your backup", "Save the phrase and check the box first.", walk.MsgBoxIconInformation)
			return
		}
		d.Accept()
	}}, PushButton{AssignTo: &cancel, Text: "Cancel", OnClicked: func() { d.Cancel() }}}})
	e := (Dialog{AssignTo: &d, Title: title, Size: Size{Width: 600, Height: 360}, Layout: VBox{}, DefaultButton: &ok, CancelButton: &cancel, Children: children}).Create(m.window)
	result := walk.DlgCmdCancel
	if e == nil {
		preventWheelChanges(d.Handle())
		result = d.Run()
	}
	if d != nil {
		defer d.Dispose()
	}
	return e == nil && result == walk.DlgCmdOK
}
func (m *manager) walletError(e error) {
	walk.MsgBox(m.window, "Wallet", e.Error(), walk.MsgBoxIconWarning)
}
func (m *manager) activeWallet() *wallet.Wallet {
	if m.busy || m.vault == nil {
		return nil
	}
	w := m.vault.Find(m.selectedWallet)
	if w == nil {
		return nil
	}
	if w.SignedOut {
		m.walletError(fmt.Errorf("This wallet is signed out. Choose Sign back in."))
		return nil
	}
	return w
}
func (m *manager) requireVault() bool {
	if m.busy {
		return false
	}
	if m.vault == nil {
		m.walletError(fmt.Errorf("Unlock or set up your wallet vault first"))
		return false
	}
	return true
}
func protectWalletDir(root string) error {
	dir := filepath.Join(root, "personal-wallets")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	u, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return e
	}
	return runPS("$ErrorActionPreference='Stop'; & icacls.exe " + psQuote(dir) + " /inheritance:r /grant:r " + psQuote("*"+u.User.Sid.String()+":(OI)(CI)F") + " '*S-1-5-18:(OI)(CI)F' | Out-Null;if($LASTEXITCODE -ne 0){throw 'Cannot protect wallet folder'}")
}
func (m *manager) unlockVault() {
	if m.busy {
		return
	}
	if m.vault != nil {
		m.walletError(fmt.Errorf("The vault is already unlocked"))
		return
	}
	existing := wallet.Exists(m.root)
	fields := []inputField{{"Vault password", "", true}}
	note := "Enter your vault password. This unlocks saved wallets on this PC."
	if !existing {
		note = "Choose a password of at least 12 characters. It encrypts all wallets saved here. Keep each wallet's recovery phrase too: a password alone cannot restore your funds."
		fields = append(fields, inputField{"Repeat password", "", true})
	}
	values, ok := m.inputDialog("Wallet vault", note, fields)
	if !ok {
		return
	}
	password := values[0]
	if !existing && (utf8.RuneCountInString(password) < 12 || password != values[1]) {
		m.walletError(fmt.Errorf("Use at least 12 characters and enter the same password twice"))
		return
	}
	m.action(func() error {
		if e := protectWalletDir(m.root); e != nil {
			return e
		}
		var v *wallet.Vault
		var e error
		if existing {
			v, e = wallet.Load(m.root, password)
		} else {
			v = &wallet.Vault{}
			e = wallet.Save(m.root, password, v)
		}
		if e != nil {
			return e
		}
		m.onUI(func() { m.vault = v; m.vaultPassword = password; m.rebuildWalletList() })
		return nil
	})
}
func (m *manager) lockVault() {
	if m.busy {
		return
	}
	m.vault = nil
	m.vaultPassword = ""
	m.selectedWallet = ""
	m.rebuildWalletList()
}
func (m *manager) rebuildWalletList() {
	if m.walletPicker == nil {
		return
	}
	names := []string{}
	index := -1
	if m.vault != nil {
		for i, w := range m.vault.Wallets {
			label := w.Name
			if w.Seed == "" {
				label += " (view only)"
			}
			if w.SignedOut {
				label += " (signed out)"
			}
			names = append(names, label)
			if w.ID == m.selectedWallet {
				index = i
			}
		}
	}
	if len(names) == 0 {
		m.selectedWallet = ""
		names = []string{"No wallet selected"}
		index = 0
	} else if index < 0 {
		index = 0
		m.selectedWallet = m.vault.Wallets[0].ID
	}
	m.walletPicker.SetModel(names)
	m.walletPicker.SetCurrentIndex(index)
	m.renderSelectedWallet()
}
func (m *manager) chooseWallet() {
	if m.vault == nil || m.busy {
		return
	}
	i := m.walletPicker.CurrentIndex()
	if i >= 0 && i < len(m.vault.Wallets) {
		m.selectedWallet = m.vault.Wallets[i].ID
	}
	m.renderSelectedWallet()
}
func (m *manager) renderSelectedWallet() {
	if m.walletState == nil {
		return
	}
	m.walletAddress.SetText("")
	m.walletBalance.SetText("Select a wallet and click Refresh / sync.")
	m.walletHistory.SetText("")
	if m.vault == nil {
		m.walletState.SetText("Vault locked — unlock it to use your wallets.")
		return
	}
	w := m.vault.Find(m.selectedWallet)
	if w == nil {
		m.walletState.SetText("Vault unlocked. Create or import a wallet.")
		return
	}
	if w.SignedOut {
		m.walletState.SetText(w.Name + " is signed out.")
		return
	}
	mode := "Ready to receive and sign payments"
	if w.Seed == "" {
		mode = "View only — import the spending key or phrase to send"
	}
	m.walletState.SetText(w.Name + " · " + mode)
	m.walletAddress.SetText(w.Address)
	m.walletHistory.SetText(receiptText(w))
}
func (m *manager) refreshWalletControls() {
	if m.walletPicker != nil {
		m.walletPicker.SetEnabled(!m.busy && m.vault != nil)
	}
}
func (m *manager) addWallet(importing, viewOnly bool) {
	if !m.requireVault() {
		return
	}
	fields := []inputField{{"Wallet name", "", false}}
	note := "Create a new wallet backed by a 12-word recovery phrase."
	if importing {
		note = "Paste your recovery phrase or 64-character spending key.\n\nAccount number: leave this at 0 for a normal import. One recovery phrase can hold several separate accounts, numbered 0, 1, 2, and so on. Only change this if you previously created another account under that SAME phrase and know its number. Use the original number to recover the same addresses and balance. This is not the number of wallets you have, and it does not select a receiving address.\n\nImporting a spending key? Always leave this at 0."
		fields = append(fields, inputField{"Key or recovery phrase", "", true}, inputField{"Account number (usually 0)", "0", false})
	}
	if viewOnly {
		note = "An FVK lets you view a wallet without its spending key. The local wallet API must be running to validate it."
		fields = []inputField{{"Wallet name", "", false}, {"Full viewing key (192 hex characters)", "", true}}
	}
	values, ok := m.inputDialog("Add wallet", note, fields)
	if !ok {
		return
	}
	name := strings.TrimSpace(values[0])
	if name == "" || len(name) > 80 {
		m.walletError(fmt.Errorf("Enter a wallet name up to 80 characters"))
		return
	}
	account := uint64(0)
	var e error
	if importing && !viewOnly {
		account, e = strconv.ParseUint(values[2], 10, 31)
		if e != nil {
			m.walletError(fmt.Errorf("Account number must be a whole number from 0 to 2147483647. Use 0 unless restoring a known additional account."))
			return
		}
	}
	port := m.cfg.WalletPort
	m.action(func() error {
		var result signerResult
		var e error
		if viewOnly {
			result.FVK, e = wallet.CleanHex(values[1], 96)
		} else {
			request := map[string]any{"op": "create"}
			if importing {
				request = map[string]any{"op": "import", "secret": values[1], "account": account}
			}
			result, e = runSigner(m.root, request)
		}
		if e != nil {
			return e
		}
		var existingID string
		m.onUI(func() {
			for _, w := range m.vault.Wallets {
				if w.FVK == result.FVK {
					existingID = w.ID
				}
			}
		})
		if existingID != "" {
			old := *m.vault.Find(existingID)
			if viewOnly || old.Seed != "" {
				return fmt.Errorf("This wallet is already saved. Select it from the wallet list")
			}
			upgraded := m.vault.Find(existingID)
			upgraded.Secret = result.Secret
			upgraded.Seed = result.Seed
			upgraded.Account = uint32(account)
			upgraded.SignedOut = false
			if e = wallet.Save(m.root, m.vaultPassword, m.vault); e != nil {
				*upgraded = old
				return e
			}
			m.onUI(func() { m.selectedWallet = existingID; m.rebuildWalletList() })
			return nil
		}
		id, e := wallet.NewID()
		if e != nil {
			return e
		}
		token, e := wallet.NewID()
		if e != nil {
			return e
		}
		if viewOnly {
			ctx, cancel := wallet.Timeout(60 * time.Second)
			defer cancel()
			c := wallet.Client{Port: port, Token: token}
			if e = c.Watch(ctx, result.FVK); e != nil {
				return e
			}
			var addr struct {
				Address string `json:"address"`
			}
			if e = c.Do(ctx, "GET", "/api/wallet/address?index=0", nil, &addr); e != nil {
				return e
			}
			result.Address = addr.Address
		}
		if !importing {
			var backed bool
			m.onUI(func() {
				backed = m.showText("Save your recovery phrase", "Anyone with these words can spend this wallet's funds. Save them offline. The vault password is a separate password.", result.Secret, true)
			})
			if !backed {
				return nil
			}
		}
		w := wallet.Wallet{ID: id, Name: name, Secret: result.Secret, Seed: result.Seed, Account: uint32(account), FVK: result.FVK, Address: result.Address, Token: token, NextIndex: 1, Addresses: []wallet.Address{{Index: 0, Address: result.Address}}}
		m.onUI(func() { m.vault.Wallets = append(m.vault.Wallets, w) })
		if e = wallet.Save(m.root, m.vaultPassword, m.vault); e != nil {
			m.onUI(func() { m.vault.Wallets = m.vault.Wallets[:len(m.vault.Wallets)-1] })
			return e
		}
		m.onUI(func() { m.selectedWallet = id; m.rebuildWalletList() })
		return nil
	})
}
func (m *manager) syncWallet() {
	w := m.activeWallet()
	if w == nil {
		return
	}
	id, token, fvk, expectedAddress := w.ID, w.Token, w.FVK, w.Address
	port := m.cfg.WalletPort
	m.action(func() error {
		ctx, cancel := wallet.Timeout(90 * time.Second)
		defer cancel()
		c := wallet.Client{Port: port, Token: token}
		s, e := c.Status(ctx)
		if e != nil {
			return e
		}
		if !s.HasWallet {
			if e = c.Watch(ctx, fvk); e != nil {
				return e
			}
			s, e = c.Status(ctx)
			if e != nil {
				return e
			}
		}
		if s.Address != "" && s.Address != expectedAddress {
			return fmt.Errorf("Wallet API returned a different address; refresh cancelled")
		}
		var history json.RawMessage
		historyErr := c.Do(ctx, "GET", "/api/wallet/history?limit=30", nil, &history)
		m.onUI(func() {
			if m.selectedWallet == id {
				m.walletBalance.SetText(s.Display())
				if historyErr == nil {
					m.walletHistory.SetText(wallet.HistoryDisplay(history) + "\r\n" + receiptText(m.vault.Find(id)))
				}
			}
		})
		return nil
	})
}
func (m *manager) newReceiveAddress() {
	w := m.activeWallet()
	if w == nil {
		return
	}
	if w.NextIndex == ^uint32(0) {
		m.walletError(fmt.Errorf("Address index limit reached"))
		return
	}
	id, token, fvk, index := w.ID, w.Token, w.FVK, w.NextIndex
	port := m.cfg.WalletPort
	m.action(func() error {
		ctx, cancel := wallet.Timeout(60 * time.Second)
		defer cancel()
		c := wallet.Client{Port: port, Token: token}
		s, e := c.Status(ctx)
		if e != nil {
			return e
		}
		if !s.HasWallet {
			if e = c.Watch(ctx, fvk); e != nil {
				return e
			}
		}
		var out struct {
			Address string `json:"address"`
			Index   uint32 `json:"index"`
		}
		if e = c.Do(ctx, "GET", fmt.Sprintf("/api/wallet/address?index=%d", index), nil, &out); e != nil {
			return e
		}
		if out.Address == "" || out.Index != index {
			return fmt.Errorf("Unexpected receiving address response")
		}
		if _, e = runSigner(m.root, map[string]any{"op": "address", "address": out.Address}); e != nil {
			return e
		}
		w := m.vault.Find(id)
		w.Addresses = append(w.Addresses, wallet.Address{Index: index, Address: out.Address})
		w.NextIndex++
		if e = wallet.Save(m.root, m.vaultPassword, m.vault); e != nil {
			w.Addresses = w.Addresses[:len(w.Addresses)-1]
			w.NextIndex--
			return e
		}
		m.onUI(func() {
			m.walletAddress.SetText(out.Address)
			m.showText("New receiving address", fmt.Sprintf("Address #%d receives into %s, alongside its other addresses.", index, w.Name), out.Address, false)
		})
		return nil
	})
}
func (m *manager) showReceiveAddresses() {
	w := m.activeWallet()
	if w == nil {
		return
	}
	text := ""
	for _, a := range w.Addresses {
		text += fmt.Sprintf("Address #%d\r\n%s\r\n\r\n", a.Index, a.Address)
	}
	m.showText("Receiving addresses", "Every address listed receives into the same selected wallet.", text, false)
}
func (m *manager) reauthenticate() bool {
	values, ok := m.inputDialog("Confirm vault password", "Enter your vault password to reveal private wallet information.", []inputField{{"Password", "", true}})
	if !ok {
		return false
	}
	if _, e := wallet.Load(m.root, values[0]); e != nil {
		m.walletError(e)
		return false
	}
	return true
}
func (m *manager) showRecovery() {
	w := m.activeWallet()
	if w == nil {
		return
	}
	if w.Secret == "" {
		m.walletError(fmt.Errorf("This view-only wallet contains no spending key or recovery phrase"))
		return
	}
	if !m.reauthenticate() {
		return
	}
	m.showText("Recovery backup", "Keep this secret. Import this phrase/key using the account index shown to restore this wallet.", fmt.Sprintf("Wallet: %s\r\nAccount index: %d\r\n\r\n%s", w.Name, w.Account, w.Secret), false)
}
func (m *manager) exportViewKey(ovk bool) {
	w := m.activeWallet()
	if w == nil || !m.reauthenticate() {
		return
	}
	key := w.FVK
	title, note := "Full viewing key (FVK)", "Sharing this reveals viewing capability, including incoming funds and recoverable outgoing history. It cannot authorize spending."
	if ovk {
		var e error
		key, e = wallet.OVK(w.FVK)
		if e != nil {
			m.walletError(e)
			return
		}
		title = "Outgoing viewing key (OVK)"
		note = "External Orchard OVK. It can reveal outgoing payment details where recoverable history was used. It cannot reveal the entire wallet balance or spend."
	}
	m.showText(title, note, key, false)
}
func (m *manager) checkViewKey(ovk bool) {
	if !m.requireVault() {
		return
	}
	kind, size := "FVK", 96
	if ovk {
		kind, size = "OVK", 32
	}
	v, ok := m.inputDialog("Check "+kind, "Paste a viewing key. The check reports format and whether it matches a saved wallet.", []inputField{{kind, "", true}})
	if !ok {
		return
	}
	key, e := wallet.CleanHex(v[0], size)
	if e != nil {
		m.walletError(e)
		return
	}
	matches := []string{}
	for _, w := range m.vault.Wallets {
		candidate := w.FVK
		if ovk {
			candidate, _ = wallet.OVK(candidate)
		}
		if key == candidate {
			matches = append(matches, w.Name)
		}
	}
	message := "Correct length and hex format. No matching wallet in this vault."
	if len(matches) > 0 {
		message = "Matches saved wallet(s): " + strings.Join(matches, ", ")
	}
	if ovk {
		message += "\n\nAn arbitrary 32-byte OVK has no checksum or identity proof. This check does not scan outgoing transactions. An OVK alone cannot show a full balance."
	} else {
		message += "\n\nTo validate an unfamiliar FVK with ZKas and view its balance/history, use Import FVK (view only). Format alone does not prove a valid curve key."
	}
	walk.MsgBox(m.window, "Viewing key check", message, walk.MsgBoxIconInformation)
}
func (m *manager) signOutWallet() {
	w := m.activeWallet()
	if w == nil {
		return
	}
	w.SignedOut = true
	if e := wallet.Save(m.root, m.vaultPassword, m.vault); e != nil {
		w.SignedOut = false
		m.walletError(e)
		return
	}
	m.rebuildWalletList()
}
func (m *manager) signInWallet() {
	if !m.requireVault() {
		return
	}
	w := m.vault.Find(m.selectedWallet)
	if w == nil || !w.SignedOut {
		return
	}
	if !m.reauthenticate() {
		return
	}
	w.SignedOut = false
	if e := wallet.Save(m.root, m.vaultPassword, m.vault); e != nil {
		w.SignedOut = true
		m.walletError(e)
		return
	}
	m.rebuildWalletList()
}
func (m *manager) removeWallet() {
	if !m.requireVault() {
		return
	}
	w := m.vault.Find(m.selectedWallet)
	if w == nil {
		return
	}
	values, ok := m.inputDialog("Remove wallet from this PC", "This removes this wallet's saved recovery secret from this vault. Funds remain on-chain. Make sure you have a recovery backup. Existing viewing/scan data in walletd is retained. Type the wallet name to confirm.", []inputField{{"Type: " + w.Name, "", false}})
	if !ok || values[0] != w.Name {
		return
	}
	old := append([]wallet.Wallet(nil), m.vault.Wallets...)
	id := w.ID
	next := []wallet.Wallet{}
	for _, x := range old {
		if x.ID != id {
			next = append(next, x)
		}
	}
	m.vault.Wallets = next
	if e := wallet.Save(m.root, m.vaultPassword, m.vault); e != nil {
		m.vault.Wallets = old
		m.walletError(e)
		return
	}
	m.selectedWallet = ""
	m.rebuildWalletList()
}

// Network monitoring reads only the selected wallet's token; spending secrets
// are never copied to status requests or public sharing configuration.
func (m *manager) walletPollSnapshot() (string, string, int) {
	if m.vault == nil || m.busy {
		return "", "", 0
	}
	w := m.vault.Find(m.selectedWallet)
	if w == nil || w.SignedOut {
		return "", "", 0
	}
	return w.ID, w.Token, m.cfg.WalletPort
}
func (m *manager) pollPersonalWallet(id, token string, port int) {
	if id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s, e := (wallet.Client{Port: port, Token: token}).Status(ctx)
	m.onUI(func() {
		if m.vault == nil || m.selectedWallet != id || m.busy {
			return
		}
		w := m.vault.Find(id)
		if w == nil || w.SignedOut {
			return
		}
		if e != nil {
			m.walletBalance.SetText("Local wallet API unavailable. Start services in Overview, then Refresh / sync.")
		} else {
			if s.Address != "" && s.Address != w.Address {
				m.walletBalance.SetText("Wallet API identity mismatch. Stop and check the local wallet service.")
			} else {
				m.walletBalance.SetText(s.Display())
			}
		}
	})
}

func receiptText(w *wallet.Wallet) string {
	if w == nil {
		return ""
	}
	text := "Saved send receipts (check history for confirmation):\r\n"
	for i := len(w.Receipts) - 1; i >= 0; i-- {
		r := w.Receipts[i]
		text += fmt.Sprintf("%s · %s ZKAS · %s\r\nTo: %s\r\nTx: %s\r\n", r.Time, r.Amount, r.State, r.To, r.TxID)
	}
	return text
}
