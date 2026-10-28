package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"zkas-node-manager/internal/telegrambot"
	"zkas-node-manager/internal/wallet"
)

func (m *manager) botAcquire(ctx context.Context) (func(), error) {
	claimed := false
	var reloadErr error
	err := m.botUIRun(ctx, func() {
		if !m.busy {
			reloadErr = m.botReloadVaults()
			if reloadErr != nil {
				return
			}
			m.busy = true
			m.botKRevision++
			claimed = true
			m.kwControls()
		}
	})
	release := func() {
		if !m.closed.Load() {
			m.window.Synchronize(func() { m.busy = false; m.kwControls() })
		}
	}
	if err != nil {
		if claimed {
			release()
		}
		return nil, err
	}
	if reloadErr != nil {
		return nil, reloadErr
	}
	if !claimed {
		return nil, fmt.Errorf("Manager is busy; try again after the current operation")
	}
	return release, nil
}
func (m *manager) botSpendWallet(ctx context.Context, id string) (wallet.Wallet, error) {
	var w wallet.Wallet
	count := 0
	err := m.botUIRun(ctx, func() {
		if m.botZVault == nil {
			return
		}
		for _, v := range m.botZVault.Wallets {
			if len(id) >= 8 && strings.HasPrefix(v.ID, id) && !v.SignedOut && v.Seed != "" {
				w = v
				count++
			}
		}
	})
	if err != nil {
		return w, err
	}
	if count != 1 {
		return w, fmt.Errorf("Unlock this spending wallet for Telegram first")
	}
	return w, nil
}
func (m *manager) botPreparePayment(ctx context.Context, in telegrambot.PaymentInput) (telegrambot.PaymentReview, error) {
	if in.Chain == "kaspa" {
		return m.botPrepareKaspa(ctx, in)
	}
	release, err := m.botAcquire(ctx)
	if err != nil {
		return telegrambot.PaymentReview{}, err
	}
	defer release()
	saved, err := m.botSpendWallet(ctx, in.WalletID)
	if err != nil {
		return telegrambot.PaymentReview{}, err
	}
	// A pending review retains identity and public data, never the spending secret.
	saved.Seed = ""
	saved.Secret = ""
	amount, err := wallet.ParseAmount(in.Amount)
	if err != nil {
		return telegrambot.PaymentReview{}, err
	}
	maxFee, err := wallet.ParseAmount(in.MaxFee)
	if err != nil {
		return telegrambot.PaymentReview{}, err
	}
	if _, err = runSigner(m.root, map[string]any{"op": "address", "address": in.To}); err != nil {
		return telegrambot.PaymentReview{}, fmt.Errorf("Invalid ZKas recipient address")
	}
	port := 0
	m.botUIRun(ctx, func() { port = m.cfg.WalletPort })
	client := wallet.Client{Port: port, Token: saved.Token}
	q, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	s, err := client.Status(q)
	if err != nil {
		return telegrambot.PaymentReview{}, fmt.Errorf("Wallet backend unavailable")
	}
	if !s.HasWallet || s.Error != "" || s.Loading || s.MissingHistory || (!s.Synced && !s.SpendReady) {
		return telegrambot.PaymentReview{}, fmt.Errorf("Open/sync this wallet and wait until it is ready to spend")
	}
	if s.Address != "" && s.Address != saved.Address {
		return telegrambot.PaymentReview{}, fmt.Errorf("Wallet address mismatch")
	}
	prep, err := client.PrepareOptions(q, saved.FVK, in.To, amount, in.Memo, false)
	if err != nil {
		return telegrambot.PaymentReview{}, fmt.Errorf("Wallet backend could not prepare payment; check its log and available balance")
	}
	if err = prep.Validate(amount, maxFee); err != nil {
		return telegrambot.PaymentReview{}, fmt.Errorf("Payment failed amount/fee validation; nothing sent")
	}
	review := telegrambot.PaymentReview{Text: fmt.Sprintf("ZKas wallet: %s\nTo: %s\nAmount: %s ZKAS\nActual fee: %s ZKAS\nMemo: %s", saved.Name, in.To, wallet.FormatAmount(amount), wallet.FormatAmount(prep.Fee), in.Memo)}
	review.Commit = func(ctx context.Context) (string, error) {
		release, err := m.botAcquire(ctx)
		if err != nil {
			return "", err
		}
		defer release()
		current, err := m.botSpendWallet(ctx, saved.ID)
		if err != nil {
			return "", err
		}
		if current.Token != saved.Token || current.FVK != saved.FVK || current.Address != saved.Address {
			return "", fmt.Errorf("Wallet changed; prepare this payment again")
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err = m.botReserveSpend(ctx, "zkas", amount, prep.Fee); err != nil {
			return "", err
		}
		sig, err := runSigner(m.root, map[string]any{"op": "sign_payment", "seed": current.Seed, "to": in.To, "amount": strconv.FormatUint(amount, 10), "maxFee": strconv.FormatUint(prep.Fee, 10), "bundle": prep.Bundle, "disclosureJson": string(prep.Disclosure), "spendAuthJson": string(prep.SpendAuth)})
		current.Seed = ""
		current.Secret = ""
		if err != nil || len(sig.Sigs) == 0 {
			return "", fmt.Errorf("Signing failed; nothing broadcast")
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		var saveErr error
		idx := -1
		err = m.botUIRun(ctx, func() {
			w := m.botZVault.Find(saved.ID)
			idx = len(w.Receipts)
			w.Receipts = append(w.Receipts, wallet.Receipt{Kind: "Telegram payment", Memo: in.Memo, Time: time.Now().Format(time.RFC3339), To: in.To, Amount: wallet.FormatAmount(amount), Fee: wallet.FormatAmount(prep.Fee), State: "Submission outcome unknown — check history before retrying"})
			saveErr = m.botSaveZ()
			if saveErr != nil {
				w.Receipts = w.Receipts[:idx]
			}
		})
		if err != nil || saveErr != nil {
			return "", fmt.Errorf("Receipt could not be saved; payment was not broadcast")
		}
		// Once submission begins, finish recording its outcome even if the bot stops.
		submit, stop := context.WithTimeout(context.Background(), 2*time.Minute)
		defer stop()
		tx, err := client.Submit(submit, prep.Session, sig.Sigs)
		if err != nil {
			return "", fmt.Errorf("Submission outcome unknown. Check history before sending again. This request will not be retried")
		}
		err = m.botUIRun(submit, func() {
			w := m.botZVault.Find(saved.ID)
			w.Receipts[idx].TxID = tx.TxID
			w.Receipts[idx].State = "Broadcast — awaiting chain confirmation"
			saveErr = m.botSaveZ()

		})
		result := "Payment broadcast; awaiting confirmation.\nTransaction ID: " + tx.TxID
		if err != nil || saveErr != nil {
			result += "\nFinal receipt could not be saved; keep this ID and do not resend."
		}
		return result, nil
	}
	return review, nil
}
func (m *manager) botPrepareKaspa(ctx context.Context, in telegrambot.PaymentInput) (telegrambot.PaymentReview, error) {
	release, err := m.botAcquire(ctx)
	if err != nil {
		return telegrambot.PaymentReview{}, err
	}
	defer release()
	var p *kaspaWalletProcess
	var name string
	m.botUIRun(ctx, func() {
		if m.botKVault != nil && m.botKState.Open && m.botKState.Filename == in.WalletID && m.botKState.Selected == in.AccountID {
			if w := m.botKVault.Find(in.WalletID); w != nil {
				p = m.botKProcess
				name = w.Name
			}
		}
	})
	if p == nil {
		return telegrambot.PaymentReview{}, fmt.Errorf("Open this Kaspa wallet/account using the bot menu first")
	}
	var s kaspaWalletState
	if err = p.call(map[string]any{"op": "status"}, &s); err != nil || !s.Ready || !s.Synced || !s.Connected || s.Filename != in.WalletID || s.Selected != in.AccountID {
		return telegrambot.PaymentReview{}, fmt.Errorf("Wait for this Kaspa wallet/account to connect and sync")
	}
	var estimate struct{ Token, Text, FeeSompi string }
	if err = p.call(map[string]any{"op": "estimate", "address": in.To, "amount": in.Amount}, &estimate); err != nil {
		return telegrambot.PaymentReview{}, fmt.Errorf("Could not estimate payment. Check the recipient, spendable balance and node connection")
	}
	fee, feeErr := strconv.ParseUint(estimate.FeeSompi, 10, 64)
	if feeErr != nil {
		return telegrambot.PaymentReview{}, fmt.Errorf("Kaspa fee was not reported; restart the manager with the complete current runtime")
	}
	amount, err := wallet.ParseAmount(in.Amount)
	if err != nil {
		return telegrambot.PaymentReview{}, err
	}
	review := telegrambot.PaymentReview{Text: "Kaspa wallet: " + name + "\nAccount: " + in.AccountID + "\n" + estimate.Text}
	review.Commit = func(ctx context.Context) (string, error) {
		release, err := m.botAcquire(ctx)
		if err != nil {
			return "", err
		}
		defer release()
		var password, passphrase string
		valid := false
		m.botUIRun(ctx, func() {
			if m.botKVault != nil && m.botKProcess == p && m.botKState.Open && m.botKState.Filename == in.WalletID && m.botKState.Selected == in.AccountID {
				if w := m.botKVault.Find(in.WalletID); w != nil {
					password = w.Password
					passphrase = w.Passphrase
					valid = true
				}
			}
		})
		if !valid {
			return "", fmt.Errorf("Wallet locked or selected account changed. Prepare the payment again")
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err = m.botReserveSpend(ctx, "kaspa", amount, fee); err != nil {
			return "", err
		}
		var saveErr error
		idx := -1
		err = m.botUIRun(ctx, func() {
			w := m.botKVault.Find(in.WalletID)
			idx = len(w.Receipts)
			w.Receipts = append(w.Receipts, time.Now().Format(time.RFC3339)+"\n"+review.Text+"\nSubmission outcome unknown — check history before retrying")
			saveErr = m.botSaveK()
			if saveErr != nil {
				w.Receipts = w.Receipts[:idx]
			}
		})
		if err != nil || saveErr != nil {
			return "", fmt.Errorf("Receipt could not be saved; payment was not submitted")
		}
		var receipt struct{ Text string }
		err = p.call(map[string]any{"op": "send", "token": estimate.Token, "password": password, "passphrase": passphrase}, &receipt)
		password = ""
		passphrase = ""
		if err != nil {
			return "", fmt.Errorf("Kaspa send did not return a successful receipt. Submission may have begun; check history before retrying")
		}
		q, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		err = m.botUIRun(q, func() {
			w := m.botKVault.Find(in.WalletID)
			w.Receipts[idx] = time.Now().Format(time.RFC3339) + "\n" + review.Text + "\n" + receipt.Text
			saveErr = m.botSaveK()
		})
		if err == nil && saveErr == nil {
			err = m.botUIRun(q, func() { saveErr = m.botKSnapshot() })
		}
		result := receipt.Text
		if err != nil || saveErr != nil {
			result += "\nFinal receipt could not be saved; keep these transaction IDs. Do not resend."
		}
		return result, nil
	}
	return review, nil
}

func (m *manager) botReserveSpend(ctx context.Context, chain string, amount, fee uint64) error {
	var limits telegrambot.SpendingLimits
	err := m.botUIRun(ctx, func() { limits = m.botConfig.Limits })
	if err != nil {
		return err
	}
	if err = telegrambot.ReserveSpend(m.root, limits, chain, amount, fee); err != nil {
		return fmt.Errorf("Payment not sent: spending limit check/reservation failed: %w", err)
	}
	return nil
}
