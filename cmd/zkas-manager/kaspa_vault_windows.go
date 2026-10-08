package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/lxn/walk"
	"os"
	"path/filepath"
	"regexp"
	"zkas-node-manager/internal/kaspavault"
)

var kaspaVaultID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

func (m *manager) kwUnlockVault() bool {
	if m.kwVault != nil {
		return true
	}
	exists := kaspavault.Exists(m.root)
	title, note := "Create Kaspa vault", "Choose one password for all your Kaspa wallets, separate from ZKas. Use at least 12 characters."
	fields := []inputField{{Label: "Kaspa vault password", Secret: true}}
	if exists {
		title = "Unlock Kaspa vault"
		note = "Unlock your Kaspa wallets with your Kaspa vault password."
	} else {
		fields = append(fields, inputField{Label: "Confirm password", Secret: true})
	}
	v, ok := m.inputDialog(title, note, fields)
	if !ok {
		return false
	}
	defer clear(v)
	var vault *kaspavault.Vault
	var e error
	if exists {
		vault, e = kaspavault.Load(m.root, v[0])
	} else {
		if v[0] != v[1] {
			m.walletError(errors.New("Passwords do not match"))
			return false
		}
		vault = &kaspavault.Vault{}
		e = kaspavault.Save(m.root, v[0], vault)
	}
	if e != nil {
		m.walletError(e)
		return false
	}
	m.kwVault = vault
	m.kwVaultPassword = v[0]
	m.kwStatus.SetText("Kaspa vault unlocked. Select a wallet and open it.")
	m.kwControls()
	return true
}
func (m *manager) kwSnapshotState(_ kaspaWalletState) error { return m.kwSnapshot() }
func (m *manager) kwSnapshot() error {
	if m.kwVault == nil {
		return nil
	}
	for i := range m.kwVault.Wallets {
		w := &m.kwVault.Wallets[i]
		if !kaspaVaultID.MatchString(w.ID) {
			return errors.New("Invalid wallet ID in Kaspa vault")
		}
		b, e := os.ReadFile(filepath.Join(m.root, "kaspa-wallets", w.ID+".wallet"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if len(b) > 4<<20 {
			return errors.New("Kaspa wallet exceeds backup size limit")
		}
		w.Backup = b
	}
	return kaspavault.Save(m.root, m.kwVaultPassword, m.kwVault)
}
func (m *manager) kwVaultList() {
	if m.kwVault == nil {
		return
	}
	names := []string{}
	m.kwFiles = nil
	idx := -1
	for _, w := range m.kwVault.Wallets {
		if _, e := os.Stat(filepath.Join(m.root, "kaspa-wallets", w.ID+".wallet")); e != nil {
			continue
		}
		if w.ID == m.kwState.Filename {
			idx = len(names)
		}
		names = append(names, w.Name)
		m.kwFiles = append(m.kwFiles, w.ID)
	}
	m.kwList.SetModel(names)
	if idx < 0 && len(names) > 0 {
		idx = 0
	}
	m.kwList.SetCurrentIndex(idx)
}
func (m *manager) kwRestoreVault() {
	if m.busy || !m.kwUnlockVault() {
		return
	}
	d := walk.FileDialog{Title: "Restore encrypted Kaspa vault backup", Filter: "Encrypted Kaspa vault (*.json)|*.json"}
	ok, e := d.ShowOpen(m.window)
	if e != nil || !ok {
		return
	}
	v, ok := m.inputDialog("Open vault backup", "Enter the backup’s vault password. Restored wallets are added to your current vault; existing wallets are preserved.", []inputField{{Label: "Backup vault password", Secret: true}})
	if !ok {
		return
	}
	m.chainAction("Restoring Kaspa vault", "Kaspa wallets restored into your current vault.", func() error {
		defer clear(v)
		st, e := os.Stat(d.FilePath)
		if e != nil {
			return e
		}
		if st.Size() > 64<<20 {
			return errors.New("Vault backup exceeds 64 MB")
		}
		imported, e := kaspavault.LoadFile(d.FilePath, v[0])
		if e != nil {
			return e
		}
		if len(imported.Wallets) == 0 {
			return errors.New("Backup contains no wallets")
		}
		added := []kaspavault.Wallet{}
		paths := []string{}
		committed := false
		defer func() {
			if !committed {
				for _, p := range paths {
					os.Remove(p)
				}
			}
		}()
		if e = os.MkdirAll(filepath.Join(m.root, "kaspa-wallets"), 0700); e != nil {
			return e
		}
		for _, w := range imported.Wallets {
			if len(w.Backup) == 0 {
				continue
			}
			if len(w.Backup) > 4<<20 || w.Password == "" {
				return errors.New("Invalid wallet in vault backup")
			}
			id := make([]byte, 16)
			if _, e = rand.Read(id); e != nil {
				return e
			}
			w.ID = "restored-" + hex.EncodeToString(id)
			dst := filepath.Join(m.root, "kaspa-wallets", w.ID+".wallet")
			f, e := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e != nil {
				return e
			}
			paths = append(paths, dst)
			_, e = f.Write(w.Backup)
			ce := f.Close()
			if e != nil {
				return e
			}
			if ce != nil {
				return ce
			}
			added = append(added, w)
		}
		if len(added) == 0 {
			return errors.New("Backup contains no saved wallet files")
		}
		n := len(m.kwVault.Wallets)
		m.kwVault.Wallets = append(m.kwVault.Wallets, added...)
		if e = kaspavault.Save(m.root, m.kwVaultPassword, m.kwVault); e != nil {
			m.kwVault.Wallets = m.kwVault.Wallets[:n]
			return e
		}
		committed = true
		m.onUI(m.kwVaultList)
		return nil
	})
}
func (m *manager) kwDelete() {
	if m.busy || m.kwVault == nil || !m.kwState.Open {
		return
	}
	w := m.kwVault.Find(m.kwState.Filename)
	if w == nil {
		return
	}
	if walk.MsgBox(m.window, "Remove Kaspa wallet", fmt.Sprintf("Remove %s from this manager?\n\nThis locks the wallet and removes it from the active list. An encrypted recovery copy remains in kaspa-vault/removed. Blockchain funds are not deleted. Keep a vault backup before proceeding.", w.Name), walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
		return
	}
	m.chainAction("Removing Kaspa wallet", "Wallet removed from the active list. Encrypted recovery copy retained.", func() error {
		if e := m.kwSnapshot(); e != nil {
			return e
		}
		dir := filepath.Join(m.root, "kaspa-vault", "removed")
		if e := os.MkdirAll(dir, 0700); e != nil {
			return e
		}
		// Save a complete encrypted vault snapshot before removing its credential.
		b, e := os.ReadFile(kaspavault.Path(m.root))
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(dir, w.ID+"-vault.json"), b, 0600); e != nil {
			return e
		}
		m.kwProcess.stop()
		m.kwProcess = nil
		m.onUI(func() { m.kwApply(kaspaWalletState{}); m.kwHistory.SetText("") })
		src := filepath.Join(m.root, "kaspa-wallets", w.ID+".wallet")
		dst := filepath.Join(dir, w.ID+".wallet")
		if e = os.Rename(src, dst); e != nil {
			return e
		}
		old := m.kwVault.Wallets
		next := []kaspavault.Wallet{}
		for _, entry := range old {
			if entry.ID != w.ID {
				next = append(next, entry)
			}
		}
		m.kwVault.Wallets = next
		if e = kaspavault.Save(m.root, m.kwVaultPassword, m.kwVault); e != nil {
			m.kwVault.Wallets = old
			os.Rename(dst, src)
			return e
		}
		m.onUI(func() { m.kwApply(kaspaWalletState{}); m.kwVaultList(); m.kwHistory.SetText("") })
		return nil
	})
}

func (m *manager) kwChangePassword() {
	if m.busy || !m.kwUnlockVault() {
		return
	}
	v, ok := m.inputDialog("Change Kaspa vault password", "Use at least 12 characters. This changes the password for the current vault. Previously exported backups still use their original password; save a new backup after changing it.", []inputField{{Label: "New vault password", Secret: true}, {Label: "Confirm password", Secret: true}})
	if !ok {
		return
	}
	if v[0] != v[1] {
		clear(v)
		m.walletError(errors.New("Passwords do not match"))
		return
	}
	m.chainAction("Updating Kaspa vault password", "Kaspa vault password changed.", func() error {
		defer clear(v)
		if e := kaspavault.Save(m.root, v[0], m.kwVault); e != nil {
			return e
		}
		m.kwVaultPassword = v[0]
		return nil
	})
}
