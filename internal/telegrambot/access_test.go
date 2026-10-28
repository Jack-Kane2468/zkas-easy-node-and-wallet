package telegrambot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestChainPermissionsCoverMenusCommandsAndPayments(t *testing.T) {
	for _, chain := range []string{"zkas", "kaspa"} {
		e, called, now := fixture()
		e.Config.Permissions = Permissions{Wallets: chain == "zkas", KaspaWallets: chain == "kaspa", Spend: true, RemoteLock: true}
		allowed, denied := "wallets", "kaspa_wallets"
		if chain == "kaspa" {
			allowed, denied = denied, allowed
		}
		r, _ := e.Process(context.Background(), msg(42, "/menu", now), now)
		var commands []string
		for _, row := range r.Choices {
			for _, b := range row {
				commands = append(commands, b.Command)
			}
		}
		found := false
		for _, cmd := range commands {
			if cmd == denied {
				t.Fatal("disabled chain in menu", commands)
			}
			if cmd == allowed {
				found = true
			}
		}
		if !found {
			t.Fatal("enabled chain missing")
		}
		e.Process(context.Background(), msg(42, "/"+denied, now), now)
		if *called != 0 {
			t.Fatal("disabled command reached handler")
		}
		other := "kaspa wallet account"
		if chain == "kaspa" {
			other = "zkas wallet"
		}
		e.Process(context.Background(), msg(42, "/send "+other, now), now)
		if e.payments[42] != nil {
			t.Fatal("disabled-chain payment created")
		}
		e.Process(context.Background(), msg(42, "/unlock "+strings.Fields(other)[0], now), now)
		if e.unlocks[42].Chain != "" {
			t.Fatal("disabled-chain unlock created")
		}
	}
}
func TestRemoteUnlockPrivateTimedAndSecretNotEchoed(t *testing.T) {
	e, _, now := fixture()
	e.Config.Permissions = Permissions{KaspaWallets: true, RemoteLock: true}
	var request Request
	calls, deletes := 0, 0
	e.Handle = func(_ context.Context, r Request) (string, error) {
		request = r
		calls++
		return "Unlocked for Telegram only", nil
	}
	e.DeleteSecret = func(id, mid int64) error {
		if id != 42 {
			t.Fatal("wrong sender")
		}
		deletes++
		return nil
	}
	e.Process(context.Background(), msg(42, "/unlock kaspa", now), now)
	secret := " password with spaces "
	reply, err := e.Process(context.Background(), msg(42, secret, now), now)
	if err != nil || calls != 1 || deletes != 1 || request.Args[1] != secret || strings.Contains(reply.Text, secret) {
		t.Fatal("unlock contract failed", calls, deletes)
	}
	e.Process(context.Background(), msg(42, "/unlock kaspa", now), now)
	if e.unlocks[42].Chain != "" {
		t.Fatal("cooldown bypassed")
	}
	later := now.Add(31 * time.Second)
	e.Process(context.Background(), msg(42, "/unlock kaspa", later), later)
	e.Process(context.Background(), msg(42, "/cancel", later), later)
	e.Process(context.Background(), msg(42, secret, later), later)
	if calls != 1 {
		t.Fatal("cancelled unlock executed")
	}
	e.Process(context.Background(), msg(42, "/unlock kaspa", later), later)
	expired := later.Add(3 * time.Minute)
	e.Process(context.Background(), msg(42, secret, expired), expired)
	if calls != 1 {
		t.Fatal("expired unlock executed")
	}
	e.Process(context.Background(), msg(42, "/unlock kaspa", expired), expired)
	e.Config.Permissions.RemoteLock = false
	e.Process(context.Background(), msg(42, secret, expired), expired)
	if calls != 1 {
		t.Fatal("revoked unlock executed")
	}
}
func TestCopyButtonsUseTelegramClipboardPayload(t *testing.T) {
	address := "kaspa:" + strings.Repeat("q", 60)
	tx := strings.Repeat("a", 64)
	b := copyButtons(address + "\n" + address + "\nTX: " + tx)
	if len(b) != 2 || b[0][0].CopyText.Text != address || b[1][0].CopyText.Text != tx {
		t.Fatal(b)
	}
	data, _ := json.Marshal(b)
	if strings.Contains(string(data), "callback_data") || !strings.Contains(string(data), "copy_text") {
		t.Fatal(string(data))
	}
	if len(copyButtons("zkas:"+strings.Repeat("q", 300))) != 0 {
		t.Fatal("oversized clipboard payload")
	}
}
func TestStopOnCloseDefault(t *testing.T) {
	var c Config
	if c.KeepRunning {
		t.Fatal("new configs must stop on close")
	}
	json.Unmarshal([]byte(`{"Token":"old"}`), &c)
	if c.KeepRunning {
		t.Fatal("legacy configs must stop on close")
	}
}

func TestWalletEntryPromptsThenResumes(t *testing.T) {
	e, _, now := fixture()
	e.Config.Permissions = Permissions{Wallets: true, KaspaWallets: true, RemoteLock: true}
	unlocked := false
	e.WalletAccess = func(_ context.Context, chain string) (bool, string, error) {
		return unlocked, "Desktop manager: " + chain + " vault unlocked.", nil
	}
	var calls []string
	e.Handle = func(_ context.Context, r Request) (string, error) {
		calls = append(calls, r.Command)
		if r.Command == "unlock" {
			unlocked = r.Args[1] == "correct password"
		}
		return "Wallet screen", nil
	}
	e.DeleteSecret = func(int64, int64) error { return nil }
	home, _ := e.Process(context.Background(), msg(42, "/menu", now), now)
	for _, row := range home.Choices {
		for _, button := range row {
			if strings.HasPrefix(button.Command, "unlock ") || strings.HasPrefix(button.Command, "lock ") {
				t.Fatal("bulky lock controls in main menu")
			}
		}
	}
	reply, err := e.Process(context.Background(), msg(42, "/kaspa_open wallet-id", now), now)
	if err != nil || !strings.Contains(reply.Text, "password") || !strings.Contains(reply.Text, "vault unlocked") || len(calls) != 0 {
		t.Fatal(reply, err, calls)
	}
	reply, err = e.Process(context.Background(), msg(42, "correct password", now), now)
	if err != nil || strings.Join(calls, ",") != "unlock,kaspa_open" || reply.Text != "Wallet screen" {
		t.Fatal(reply, err, calls)
	}
	// Already-unlocked Telegram sessions skip the prompt even when desktop is unlocked.
	calls = nil
	e.Process(context.Background(), msg(42, "/kaspa_wallets", now), now)
	if strings.Join(calls, ",") != "kaspa_wallets" {
		t.Fatal(calls)
	}
	for _, row := range e.mainMenu() {
		for _, b := range row {
			if strings.HasPrefix(b.Command, "lock ") {
				t.Fatal("lock in home")
			}
		}
	}
}
func TestWrongPasswordDoesNotResumeAndRemotePermissionIsRequired(t *testing.T) {
	e, _, now := fixture()
	e.Config.Permissions = Permissions{Wallets: true, RemoteLock: true}
	e.WalletAccess = func(context.Context, string) (bool, string, error) {
		return false, "Desktop manager: zkas vault locked.", nil
	}
	var commands []string
	e.Handle = func(_ context.Context, r Request) (string, error) {
		commands = append(commands, r.Command)
		return "Wrong password", nil
	}
	e.Process(context.Background(), msg(42, "/wallets", now), now)
	e.Process(context.Background(), msg(42, "wrong password", now), now)
	if strings.Join(commands, ",") != "unlock" {
		t.Fatal(commands)
	}
	e.Config.Permissions.RemoteLock = false
	reply, _ := e.Process(context.Background(), msg(42, "/wallets", now.Add(time.Minute)), now.Add(time.Minute))
	if e.unlocks[42].Chain != "" || !strings.Contains(reply.Text, "locally") {
		t.Fatal(reply)
	}
}
