package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/kaspavault"
)

type kaspaWalletState struct {
	Open, Connected, Synced, Ready     bool
	Filename, Selected, Balance, Error string
	Accounts                           []struct {
		ID, Name, Address, Kind string
		Index                   uint32
	}
}
type kaspaWalletReply struct {
	OK     bool
	Error  string
	Result json.RawMessage
}
type kaspaWalletProcess struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	in      io.WriteCloser
	replies chan kaspaWalletReply
	done    chan struct{}
}

func startKaspaWalletProcess(root string) (*kaspaWalletProcess, error) {
	dir, e := walletRuntime(root)
	if e != nil {
		return nil, e
	}
	kc, e := chains.Read(root)
	if e != nil {
		return nil, e
	}
	if kc.NodeMode == "basic" {
		return nil, errors.New("Choose Wallet / application backend in Settings → Kaspa to use the built-in wallet")
	}
	cmd := exec.Command(filepath.Join(dir, "node.exe"), "--no-warnings", filepath.Join(dir, "kaspa-wallet.cjs"), filepath.Join(root, "kaspa-wallets"), strconv.Itoa(kc.Borsh))
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	for _, v := range os.Environ() {
		u := strings.ToUpper(v)
		if !strings.HasPrefix(u, "NODE_") && !strings.HasPrefix(u, "ELECTRON_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	in, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		return nil, e
	}
	p := &kaspaWalletProcess{cmd: cmd, in: in, replies: make(chan kaspaWalletReply, 1), done: make(chan struct{})}
	if e = cmd.Start(); e != nil {
		return nil, e
	}
	go func() {
		defer close(p.replies)
		defer close(p.done)
		scan := bufio.NewScanner(out)
		scan.Buffer(make([]byte, 4096), 4<<20)
		for scan.Scan() {
			line := scan.Text()
			if !strings.HasPrefix(line, "KASPA_REPLY ") {
				continue
			}
			var r kaspaWalletReply
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "KASPA_REPLY ")), &r) == nil {
				p.replies <- r
			}
		}
		cmd.Wait()
	}()
	return p, nil
}
func (p *kaspaWalletProcess) stop() {
	if p == nil {
		return
	}
	p.in.Close()
	p.cmd.Process.Kill()
	select {
	case <-p.done:
	case <-time.After(3 * time.Second):
	}
}
func (p *kaspaWalletProcess) call(q any, out any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	b, e := json.Marshal(q)
	if e != nil {
		return e
	}
	defer clear(b)
	if _, e = p.in.Write(append(b, '\n')); e != nil {
		return errors.New("Wallet process stopped. Lock and reopen your wallet")
	}
	timer := time.NewTimer(90 * time.Second)
	defer timer.Stop()
	select {
	case r, ok := <-p.replies:
		if !ok {
			return errors.New("Wallet process stopped. Lock and reopen your wallet")
		}
		if !r.OK {
			return fmt.Errorf("%s", r.Error)
		}
		if out != nil {
			return json.Unmarshal(r.Result, out)
		}
		return nil
	case <-timer.C:
		p.cmd.Process.Kill()
		return errors.New("Wallet operation timed out. If sending, check history and your balance before trying again; a transaction may have been submitted")
	}
}

type kaspaWalletUI struct {
	kwVault                         *kaspavault.Vault
	kwVaultPassword                 string
	kwProcess                       *kaspaWalletProcess
	kwList, kwAccounts              *walk.ComboBox
	kwStatus                        *walk.TextLabel
	kwReceive                       *walk.LineEdit
	kwHistory                       *walk.TextEdit
	kwButtons                       map[string]**walk.PushButton
	kwAddressTable                  *walk.TableView
	kwAddressModel                  *kaspaAddressModel
	kwAddressStatus                 *walk.TextLabel
	kwAddressDetails                *walk.TextEdit
	kwReceiveLabel                  *walk.Label
	kwAddressOffset, kwAddressTotal int
	kwFiles                         []string
	kwState                         kaspaWalletState
	kwUpdating, kwPolling           bool
}

func (m *manager) kaspaWalletTab() TabPage {
	m.kwButtons = map[string]**walk.PushButton{}
	button := func(key, title string, fn func()) PushButton {
		p := new(*walk.PushButton)
		m.kwButtons[key] = p
		return PushButton{AssignTo: p, Text: title, OnClicked: fn, StretchFactor: 1}
	}
	pair := func(children ...Widget) Composite {
		return Composite{Layout: Grid{Columns: 2, Spacing: 6}, Children: children}
	}
	m.kwAddressModel = &kaspaAddressModel{}
	return TabPage{Title: "Wallet", Layout: VBox{MarginsZero: true}, Children: []Widget{ScrollView{Layout: VBox{Spacing: 8}, Children: []Widget{
		Label{Text: "Kaspa wallet", Font: Font{PointSize: 12, Bold: true}},
		TextLabel{Text: "One Kaspa vault holds your wallets. Unlock it, choose a wallet, then connect to your local node.", MinSize: Size{Width: 240}},
		GroupBox{Title: "Wallets", Layout: VBox{}, Children: []Widget{
			pair(button("create", "Create wallet…", func() { m.kwCreate(false) }), button("import", "Import phrase / key…", func() { m.kwCreate(true) })),
			ComboBox{AssignTo: &m.kwList, Model: []string{"Unlock the vault"}, CurrentIndex: 0},
			pair(button("open", "Unlock / open wallet", m.kwOpen), button("lock", "Lock vault", m.kwLock)),
			pair(button("refresh", "Refresh wallet list", m.kwRefreshList), button("remove", "Remove wallet…", m.kwDelete)),
			Label{Text: "Account"}, ComboBox{AssignTo: &m.kwAccounts, OnCurrentIndexChanged: func() {
				if !m.kwUpdating && m.kwAccounts.CurrentIndex() >= 0 {
					i := m.kwAccounts.CurrentIndex()
					if i < len(m.kwState.Accounts) {
						m.kwAction(map[string]any{"op": "select", "account": m.kwState.Accounts[i].ID}, true, nil)
					}
				}
			}},
		}},
		TextLabel{Text: "Account balance includes UTXOs across its receiving and change addresses. Sends use the account’s automatic coin selection.", MinSize: Size{Width: 240}},
		TextLabel{AssignTo: &m.kwStatus, MinSize: Size{Width: 240}, Text: "Vault locked.", Font: Font{Bold: true}},
		pair(button("connect", "Connect / refresh balance", func() { m.kwAction(map[string]any{"op": "connect"}, true, nil) }), button("send", "Send KAS…", m.kwSend)),
		GroupBox{Title: "Receive", Layout: VBox{}, Children: []Widget{
			Label{AssignTo: &m.kwReceiveLabel, Text: "Receiving address"}, LineEdit{AssignTo: &m.kwReceive, ReadOnly: true},
			pair(button("copy", "Copy receiving address", func() {
				if m.kwReceive.Text() != "" {
					m.copyText("Kaspa receiving address", m.kwReceive.Text())
				}
			}), button("new", "Generate next address", func() { m.kwAction(map[string]any{"op": "address"}, true, nil) })),
		}},
		GroupBox{Title: "Addresses and UTXOs", Layout: VBox{}, Children: []Widget{
			TextLabel{Text: "Each row is an indexed address, with its own unspent outputs. Change addresses hold change returned by payments. Unspent totals may include immature rewards; they are not a spendable-balance estimate.", MinSize: Size{Width: 240}},
			button("addresses", "Refresh address balances", func() { m.kwLoadAddresses(0) }),
			TextLabel{AssignTo: &m.kwAddressStatus, Text: "Open a wallet to view its indexed addresses.", MinSize: Size{Width: 240}},
			TableView{AssignTo: &m.kwAddressTable, Model: m.kwAddressModel, MinSize: Size{Width: 240, Height: 140}, OnCurrentIndexChanged: m.kwSelectAddress, Columns: []TableViewColumn{{Title: "Branch", Width: 65}, {Title: "Index", Width: 55}, {Title: "Address", Width: 350}, {Title: "Unspent KAS", Width: 120}, {Title: "UTXOs", Width: 60}}},
			pair(button("previous", "Previous addresses", func() { m.kwLoadAddresses(m.kwAddressOffset - 50) }), button("next", "Next addresses", func() { m.kwLoadAddresses(m.kwAddressOffset + 50) })),
			button("copyselected", "Copy selected address", func() {
				i := m.kwAddressTable.CurrentIndex()
				if i >= 0 && i < len(m.kwAddressModel.rows) {
					m.copyText("Kaspa address", m.kwAddressModel.rows[i].Address)
				}
			}),
			button("exportkey", "Export selected receiving key…", m.kwExportReceivingKey),
			TextEdit{AssignTo: &m.kwAddressDetails, ReadOnly: true, VScroll: true, MinSize: Size{Width: 240, Height: 95}, Text: "Select an address to see its UTXOs."},
		}},
		button("history", "Show transaction history", func() {
			m.kwAction(map[string]any{"op": "history"}, false, func(r json.RawMessage) {
				var v struct{ Text string }
				json.Unmarshal(r, &v)
				if m.kwVault != nil {
					if entry := m.kwVault.Find(m.kwState.Filename); entry != nil && len(entry.Receipts) > 0 {
						v.Text += "\r\n\r\nLocal send receipts:\r\n" + strings.Join(entry.Receipts, "\r\n\r\n")
					}
				}
				m.kwHistory.SetText(v.Text)
			})
		}),
		TextEdit{AssignTo: &m.kwHistory, ReadOnly: true, VScroll: true, MinSize: Size{Width: 240, Height: 120}, Text: "Recent transactions and local send receipts appear here."},
		TextLabel{Text: "A pruned node provides current UTXOs. It cannot reconstruct every previously spent transaction when importing a phrase.", MinSize: Size{Width: 240}},
		GroupBox{Title: "Vault backup", Layout: VBox{}, Children: []Widget{
			pair(button("backup", "Back up vault…", m.kwExport), button("restore", "Restore vault…", m.kwRestoreVault)),
			button("password", "Change vault password…", m.kwChangePassword),
			TextLabel{Text: "Keep your recovery phrases and vault password safe.", MinSize: Size{Width: 240}},
		}},
	}}}}
}
func (m *manager) kwEnsure() error {
	if m.kwProcess != nil {
		return nil
	}
	p, e := startKaspaWalletProcess(m.root)
	if e == nil {
		m.kwProcess = p
	}
	return e
}
func (m *manager) kwControls() {
	if m.kwList == nil {
		return
	}
	for key, p := range m.kwButtons {
		if *p == nil {
			continue
		}
		enabled := !m.busy
		switch key {
		case "lock", "backup":
			enabled = enabled && m.kwVault != nil
		case "copy", "new", "connect", "history", "remove", "addresses":
			enabled = enabled && m.kwState.Open
		case "send":
			enabled = enabled && m.kwState.Open && m.kwState.Ready
		case "previous":
			enabled = enabled && m.kwState.Open && m.kwAddressOffset > 0
		case "next":
			enabled = enabled && m.kwState.Open && m.kwAddressOffset+len(m.kwAddressModel.rows) < m.kwAddressTotal
		case "exportkey":
			i := m.kwAddressTable.CurrentIndex()
			enabled = enabled && m.kwState.Open && i >= 0 && i < len(m.kwAddressModel.rows) && m.kwAddressModel.rows[i].Branch == "Receive"
		case "copyselected":
			enabled = enabled && m.kwState.Open && m.kwAddressTable.CurrentIndex() >= 0
		}
		if key == "new" {
			for _, a := range m.kwState.Accounts {
				if a.ID == m.kwState.Selected && strings.Contains(a.Kind, "keypair") {
					enabled = false
				}
			}
		}
		(*p).SetEnabled(enabled)
	}
	m.kwList.SetEnabled(!m.busy)
	m.kwAccounts.SetEnabled(!m.busy && m.kwState.Open)
}
func (m *manager) kwApply(s kaspaWalletState) {
	if m.kwState.Filename != s.Filename || m.kwState.Selected != s.Selected {
		m.kwClearAddresses()
	}
	m.kwState = s
	m.kwUpdating = true
	defer func() { m.kwUpdating = false }()
	names := []string{}
	idx := -1
	addr := ""
	for i, a := range s.Accounts {
		names = append(names, a.Name)
		if a.ID == s.Selected {
			idx = i
			addr = a.Address
			m.kwReceiveLabel.SetText(fmt.Sprintf("Receiving address · index %d", a.Index))
		}
	}
	oldNames, _ := m.kwAccounts.Model().([]string)
	if strings.Join(oldNames, "\n") != strings.Join(names, "\n") {
		m.kwAccounts.SetModel(names)
	}
	if m.kwAccounts.CurrentIndex() != idx {
		m.kwAccounts.SetCurrentIndex(idx)
	}
	if m.kwReceive.Text() != addr {
		m.kwReceive.SetText(addr)
	}
	status := "Locked. Unlock a wallet to continue."
	if !s.Open {
		m.kwReceiveLabel.SetText("Receiving address")
	}
	if s.Open {
		status = "Unlocked • Local node disconnected. Click Connect / refresh balance."
		if s.Connected {
			status = "Connected • Waiting for node / wallet synchronization.\n" + s.Balance
			if s.Synced {
				status = "Connected to local Kaspa node • " + s.Balance
			}
		}
		if s.Error != "" {
			status += "\n" + s.Error
		}
	}
	if s.Open {
		for _, a := range s.Accounts {
			if a.ID == s.Selected {
				status = "Active account: " + a.Name + "\n" + status
				break
			}
		}
	}
	m.kwStatus.SetText(status)
	m.kwControls()
}
func (m *manager) kwAction(q map[string]any, state bool, done func(json.RawMessage)) {
	if m.busy {
		return
	}
	if e := m.kwEnsure(); e != nil {
		m.walletError(e)
		return
	}
	p := m.kwProcess
	m.chainAction("Working with Kaspa wallet", "Kaspa wallet operation completed.", func() error {
		defer func() { delete(q, "password"); delete(q, "secret"); delete(q, "passphrase") }()
		var r json.RawMessage
		if e := p.call(q, &r); e != nil {
			return e
		}
		if state {
			var s kaspaWalletState
			if e := json.Unmarshal(r, &s); e != nil {
				return e
			}
			if e := m.kwSnapshotState(s); e != nil {
				m.onUI(func() { m.kwApply(s) })
				return e
			}
		}
		var page *kaspaAddressPage
		if state {
			var snapshot kaspaAddressPage
			offset := 0
			if q["op"] == "address" {
				var current kaspaWalletState
				if json.Unmarshal(r, &current) == nil {
					for _, a := range current.Accounts {
						if a.ID == current.Selected {
							offset = int(a.Index) / 50 * 50
						}
					}
				}
			}
			if err := p.call(map[string]any{"op": "addresses", "offset": offset}, &snapshot); err == nil {
				page = &snapshot
			}
		}
		m.onUI(func() {
			if state {
				var s kaspaWalletState
				if json.Unmarshal(r, &s) == nil {
					m.kwApply(s)
					m.kwVaultList()
					if page != nil {
						m.kwApplyAddressPage(*page)
					} else {
						m.kwClearAddresses()
					}
				}
			}
			if done != nil {
				done(r)
			}
		})
		return nil
	})
}
func (m *manager) kwRefreshList() {
	if m.busy {
		return
	}
	if m.kwVault != nil {
		m.kwVaultList()
	}
}
func (m *manager) kwOpen() {
	if m.busy || !m.kwUnlockVault() {
		return
	}
	if len(m.kwFiles) == 0 {
		m.kwVaultList()
	}
	i := m.kwList.CurrentIndex()
	if i < 0 || i >= len(m.kwFiles) {
		return
	}
	entry := m.kwVault.Find(m.kwFiles[i])
	if entry == nil {
		return
	}
	m.kwHistory.SetText("")
	m.kwAction(map[string]any{"op": "open", "filename": entry.ID, "password": entry.Password}, true, nil)
}
func (m *manager) kwLock() {
	if m.busy {
		return
	}
	if m.kwProcess != nil {
		m.kwProcess.stop()
		m.kwProcess = nil
	}
	m.kwVault = nil
	m.kwVaultPassword = ""
	m.kwApply(kaspaWalletState{})
	m.kwHistory.SetText("")
}
func (m *manager) kwCreate(importing bool) {
	if m.busy {
		return
	}
	if !m.kwUnlockVault() {
		return
	}
	note := "Create a Kaspa wallet inside your unlocked Kaspa vault. No additional wallet password is needed."
	fields := []inputField{{Label: "Wallet name"}}
	if importing {
		note = "Import a Kaspa BIP39 phrase (12 or 24 words) or 64-character private key. Account number is normally 0; other numbers derive different addresses and balances. A private key uses 0. The optional BIP39 passphrase is an original recovery secret, not your vault password."
		fields = append(fields, inputField{Label: "Recovery phrase or private key", Secret: true}, inputField{Label: "Account number", Value: "0"}, inputField{Label: "Original BIP39 passphrase (usually blank)", Secret: true})
	}
	v, ok := m.inputDialog("Kaspa wallet", note, fields)
	if !ok {
		return
	}
	if strings.TrimSpace(v[0]) == "" {
		m.walletError(errors.New("Enter a wallet name"))
		return
	}
	id := make([]byte, 16)
	credential := make([]byte, 32)
	if _, e := rand.Read(id); e != nil {
		m.walletError(e)
		return
	}
	if _, e := rand.Read(credential); e != nil {
		m.walletError(e)
		return
	}
	name := "easy-" + hex.EncodeToString(id)
	entry := kaspavault.Wallet{ID: name, Name: strings.TrimSpace(v[0]), Password: hex.EncodeToString(credential)}
	clear(credential)
	q := map[string]any{"op": "create", "filename": name, "title": entry.Name, "password": entry.Password, "index": 0}
	if importing {
		index, e := strconv.ParseUint(v[2], 10, 31)
		if e != nil {
			m.walletError(errors.New("Account number must be a whole number; normally 0"))
			return
		}
		q["secret"] = v[1]
		q["index"] = index
		q["passphrase"] = v[3]
		entry.Passphrase = v[3]
		entry.Recovery = v[1]
	}
	// Persist the internal credential before the SDK can create a wallet file.
	m.kwVault.Wallets = append(m.kwVault.Wallets, entry)
	if e := kaspavault.Save(m.root, m.kwVaultPassword, m.kwVault); e != nil {
		m.kwVault.Wallets = m.kwVault.Wallets[:len(m.kwVault.Wallets)-1]
		m.walletError(e)
		return
	}
	if importing {
		m.kwAction(q, true, nil)
		return
	}
	if e := m.kwEnsure(); e != nil {
		m.walletError(e)
		return
	}
	p := m.kwProcess
	m.chainAction("Creating Kaspa wallet", "Wallet created. Connect to your local node to check its balance.", func() error {
		defer delete(q, "password")
		defer delete(q, "secret")
		var generated struct{ Phrase string }
		if e := p.call(map[string]any{"op": "generate"}, &generated); e != nil {
			return e
		}
		saved := false
		m.onUI(func() {
			saved = m.showText("Save your Kaspa recovery phrase", "Write these words down in order. Anyone with this phrase can spend your funds. The next step stores a password-encrypted wallet on this PC.", generated.Phrase, true)
		})
		if !saved {
			return errors.New("Wallet creation canceled; no new wallet was saved")
		}
		q["secret"] = generated.Phrase
		m.kwVault.Find(name).Recovery = generated.Phrase
		if e := kaspavault.Save(m.root, m.kwVaultPassword, m.kwVault); e != nil {
			return e
		}
		var s kaspaWalletState
		if e := p.call(q, &s); e != nil {
			return e
		}
		if e := m.kwSnapshotState(s); e != nil {
			m.onUI(func() { m.kwApply(s) })
			return e
		}
		var page kaspaAddressPage
		pageErr := p.call(map[string]any{"op": "addresses", "offset": 0}, &page)
		m.onUI(func() {
			m.kwApply(s)
			m.kwVaultList()
			if pageErr == nil {
				m.kwApplyAddressPage(page)
			}
		})
		return nil
	})
}
func (m *manager) kwSend() {
	if m.busy {
		return
	}
	if !m.kwState.Open || !m.kwState.Connected || !m.kwState.Synced || !m.kwState.Ready {
		m.walletError(errors.New("Unlock a wallet and connect to your synced local Kaspa node first"))
		return
	}
	v, ok := m.inputDialog("Send KAS", "Enter the destination and amount. You will review the estimated network fee before any transaction is sent.", []inputField{{Label: "Recipient kaspa: address"}, {Label: "Amount in KAS"}})
	if !ok {
		return
	}
	p := m.kwProcess
	m.chainAction("Preparing Kaspa payment", "Payment submitted. The transaction IDs appear in the wallet panel.", func() error {
		var review struct{ Token, Text string }
		if e := p.call(map[string]any{"op": "estimate", "address": strings.TrimSpace(v[0]), "amount": strings.TrimSpace(v[1])}, &review); e != nil {
			return e
		}
		entry := m.kwVault.Find(m.kwState.Filename)
		if entry == nil {
			return errors.New("Open this wallet through the Kaspa vault first")
		}
		confirmed := false
		m.onUI(func() {
			if walk.MsgBox(m.window, "Confirm KAS payment", review.Text+"\n\nSend this payment?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes {
				confirmed = true
			}
		})
		if !confirmed {
			return errors.New("Payment canceled; nothing submitted")
		}
		entry.Receipts = append(entry.Receipts, time.Now().Format(time.RFC3339)+"\n"+review.Text+"\nSubmission outcome unknown — check history before retrying.")
		if e := kaspavault.Save(m.root, m.kwVaultPassword, m.kwVault); e != nil {
			entry.Receipts = entry.Receipts[:len(entry.Receipts)-1]
			return fmt.Errorf("Payment was NOT submitted: could not save its receipt: %w", e)
		}
		var receipt struct{ Text string }
		e := p.call(map[string]any{"op": "send", "token": review.Token, "password": entry.Password, "passphrase": entry.Passphrase}, &receipt)
		if e != nil {
			return fmt.Errorf("%w. If submission began, check your history and balance before retrying", e)
		}
		entry.Receipts[len(entry.Receipts)-1] = time.Now().Format(time.RFC3339) + "\n" + review.Text + "\n" + receipt.Text
		m.onUI(func() { m.kwHistory.SetText(receipt.Text) })
		if e := m.kwSnapshot(); e != nil {
			return fmt.Errorf("Payment submitted. Keep these IDs; do not resend: %s. Backup update failed: %w", receipt.Text, e)
		}
		return nil
	})
}
func (m *manager) kwExport() {
	if !m.kwUnlockVault() {
		return
	}
	d := walk.FileDialog{Title: "Back up all Kaspa vault wallets", Filter: "Encrypted Kaspa vault (*.json)|*.json", FilePath: "kaspa-vault-backup.json", Flags: 2}
	ok, e := d.ShowSave(m.window)
	if e != nil || !ok {
		return
	}
	m.chainAction("Backing up Kaspa vault", "Encrypted Kaspa vault backup saved.", func() error {
		if e := m.kwSnapshot(); e != nil {
			return e
		}
		b, e := os.ReadFile(kaspavault.Path(m.root))
		if e != nil {
			return e
		}
		return os.WriteFile(d.FilePath, b, 0600)
	})
}
func (m *manager) kwPoll() {
	if m.kwProcess == nil || !m.kwState.Open || m.kwPolling || m.busy {
		return
	}
	m.kwPolling = true
	p := m.kwProcess
	go func() {
		var s kaspaWalletState
		e := p.call(map[string]any{"op": "status"}, &s)
		if m.closed.Load() {
			return
		}
		m.onUI(func() {
			m.kwPolling = false
			if m.kwProcess != p || m.busy {
				return
			}
			if e != nil {
				m.kwStatus.SetText(e.Error())
			} else {
				m.kwApply(s)
			}
		})
	}()
}
