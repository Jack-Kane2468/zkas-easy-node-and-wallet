package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"zkas-node-manager/internal/kaspavault"
	"zkas-node-manager/internal/telegrambot"
	"zkas-node-manager/internal/wallet"
)

// These credentials and runtime belong exclusively to Telegram. UI callbacks
// serialize changes; botAcquire excludes desktop mutations during wallet work.
type botSession struct {
	kaspaShared                *kaspaWalletProcess
	botZVault                  *wallet.Vault
	botKVault                  *kaspavault.Vault
	botZPassword, botKPassword string
	botKProcess                *kaspaWalletProcess
	botKState                  kaspaWalletState
	botKRevision               uint64
}

func (m *manager) botLockSessions() {
	m.botZVault = nil
	m.botZPassword = ""
	m.botKVault = nil
	m.botKPassword = ""
	m.botKRevision++
	if m.botKProcess != nil {
		m.botKProcess.stop()
		m.botKProcess = nil
	}
	m.botKState = kaspaWalletState{}
}
func (m *manager) botVaultAction(ctx context.Context, c telegrambot.Config, r telegrambot.Request) (string, error) {
	if !c.Permissions.RemoteLock || len(r.Args) < 1 || !c.Permissions.Chain(r.Args[0]) {
		return "Wallet access is disabled.", nil
	}
	release, err := m.botAcquire(ctx)
	if err != nil {
		return err.Error(), nil
	}
	defer release()
	chain := r.Args[0]
	if r.Command == "lock" {
		err = m.botUIRun(ctx, func() {
			if chain == "zkas" {
				m.botZVault = nil
				m.botZPassword = ""
			} else {
				m.botKVault = nil
				m.botKPassword = ""
				m.botKRevision++
				if m.botKProcess != nil {
					m.botKProcess.stop()
					m.botKProcess = nil
				}
				m.botKState = kaspaWalletState{}
			}
		})
		return chain + " locked in Telegram. Desktop lock state unchanged.", err
	}
	if len(r.Args) != 2 {
		return "Choose Unlock from the menu first.", nil
	}
	password := r.Args[1]
	defer func() { r.Args[1] = ""; password = "" }()
	if chain == "zkas" {
		v, e := wallet.Load(m.root, password)
		if e != nil {
			return "Could not unlock the ZKas vault. Check its password and that the vault exists.", nil
		}
		err = m.botUIRun(ctx, func() { m.botZVault = v; m.botZPassword = password })
	} else {
		v, e := kaspavault.Load(m.root, password)
		if e != nil {
			return "Could not unlock the Kaspa vault. Check its password and that the vault exists.", nil
		}
		err = m.botUIRun(ctx, func() { m.botKVault = v; m.botKPassword = password })
	}
	return chain + " unlocked for Telegram only. Choose Wallets to continue.", err
}
func (m *manager) botKAvailable(id string) bool {
	if m.botKVault == nil || !kaspaVaultID.MatchString(id) || m.botKVault.Find(id) == nil {
		return false
	}
	st, e := os.Stat(filepath.Join(m.root, "kaspa-wallets", id+".wallet"))
	return e == nil && st.Mode().IsRegular()
}
func (m *manager) botKEnsure() error {
	// Independent logical sessions share one serialized SDK storage owner.
	if m.botKProcess != nil && m.botKProcess.alive() {
		return nil
	}
	if m.botKProcess != nil {
		m.botKProcess.stop()
	}
	p, e := m.newKaspaSession("telegram")
	m.botKProcess = p
	return e
}
func (m *manager) botKApply(s kaspaWalletState) { m.botKState = s }
func (m *manager) botReloadVaults() error {
	// Reload before each operation so desktop edits/password changes revoke stale
	// Telegram credentials and a bot save cannot overwrite newer wallet metadata.
	if m.botZVault != nil {
		v, e := wallet.Load(m.root, m.botZPassword)
		if e != nil {
			m.botZVault = nil
			m.botZPassword = ""
			return fmt.Errorf("ZKas vault changed; unlock it again in Telegram")
		}
		m.botZVault = v
	}
	if m.botKVault != nil {
		v, e := kaspavault.Load(m.root, m.botKPassword)
		if e != nil {
			m.botKVault = nil
			m.botKPassword = ""
			if m.botKProcess != nil {
				m.botKProcess.stop()
				m.botKProcess = nil
			}
			return fmt.Errorf("Kaspa vault changed; unlock it again in Telegram")
		}
		m.botKVault = v
	}
	return nil
}
func (m *manager) botSaveZ() error {
	if e := wallet.Save(m.root, m.botZPassword, m.botZVault); e != nil {
		return e
	}
	if m.vault != nil {
		v, e := wallet.Load(m.root, m.vaultPassword)
		if e == nil {
			m.vault = v
		}
	}
	return nil
}
func (m *manager) botSaveK() error {
	if e := kaspavault.Save(m.root, m.botKPassword, m.botKVault); e != nil {
		return e
	}
	if m.kwVault != nil {
		v, e := kaspavault.Load(m.root, m.kwVaultPassword)
		if e == nil {
			m.kwVault = v
		}
	}
	return nil
}
func (m *manager) botKSnapshotState(_ kaspaWalletState) error { return m.botKSnapshot() }
func (m *manager) botKSnapshot() error {
	if m.botKVault == nil {
		return nil
	}
	for i := range m.botKVault.Wallets {
		w := &m.botKVault.Wallets[i]
		if !kaspaVaultID.MatchString(w.ID) {
			return fmt.Errorf("Invalid wallet ID")
		}
		b, e := os.ReadFile(filepath.Join(m.root, "kaspa-wallets", w.ID+".wallet"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if len(b) > 4<<20 {
			return fmt.Errorf("Wallet backup too large")
		}
		w.Backup = b
	}
	return m.botSaveK()
}

func (m *manager) botLocalUnlock(chain string) {
	if m.busy {
		return
	}
	v, ok := m.inputDialog("Unlock "+chain+" for Telegram", "This unlocks the bot session only. The desktop vault stays in its current lock state.", []inputField{{Label: "Vault password", Secret: true}})
	if !ok {
		return
	}
	defer func() { v[0] = "" }()
	if chain == "zkas" {
		w, e := wallet.Load(m.root, v[0])
		if e != nil {
			m.walletError(e)
			return
		}
		m.botZVault = w
		m.botZPassword = v[0]
	} else {
		w, e := kaspavault.Load(m.root, v[0])
		if e != nil {
			m.walletError(e)
			return
		}
		m.botKVault = w
		m.botKPassword = v[0]
	}
	m.botState.SetText(chain + " unlocked for the Telegram session.")
}

// Called on the UI thread. Only this worker writes Kaspa SDK wallet storage;
// logical sessions retain separate credentials, selections and lock state.
func (m *manager) newKaspaSession(name string) (*kaspaWalletProcess, error) {
	if m.kaspaShared == nil || !m.kaspaShared.alive() {
		p, e := startKaspaWalletProcess(m.root)
		if e != nil {
			return nil, e
		}
		m.kaspaShared = p
	}
	m.kaspaShared.users.Add(1)
	return &kaspaWalletProcess{backend: m.kaspaShared, session: name}, nil
}
func (m *manager) botWalletAccess(ctx context.Context, chain string) (bool, string, error) {
	unlocked := false
	desktop := false
	err := m.botUIRun(ctx, func() {
		if chain == "kaspa" {
			unlocked = m.botKVault != nil
			desktop = m.kwVault != nil
		} else {
			unlocked = m.botZVault != nil
			desktop = m.vault != nil
		}
	})
	state := "locked"
	if desktop {
		state = "unlocked"
	}
	return unlocked, "Desktop manager: " + chain + " vault " + state + ".", err
}
