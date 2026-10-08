package main

import (
	"fmt"
	"github.com/lxn/walk"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"zkas-node-manager/internal/wallet"
)

func (m *manager) sendWallet()        { m.walletPayment(false) }
func (m *manager) consolidateWallet() { m.walletPayment(true) }
func (m *manager) walletPayment(consolidate bool) {
	if m.busy {
		return
	}
	w := m.activeWallet()
	if w == nil {
		return
	}
	if w.Seed == "" {
		m.walletError(fmt.Errorf("This is a view-only wallet. Import its spending key or phrase to send"))
		return
	}
	var to, memo string
	var amount, maxFee uint64
	var e error
	if consolidate {
		values, ok := m.inputDialog("Consolidate ZKAS notes", "Combine at least 3 spendable notes into fewer notes by sending them back to this wallet. One round processes up to 38 notes. A network fee applies, and the resulting funds must mature before they can be spent again (about 10 minutes). You will review the exact amount and fee before signing.", []inputField{{"Maximum fee for this round (ZKAS)", "0.3", false}})
		if !ok {
			return
		}
		maxFee, e = wallet.ParseAmount(values[0])
		to = w.Address
	} else {
		values, ok := m.inputDialog("Send ZKAS from "+w.Name, "Enter the recipient and amount. You will review the actual fee before broadcasting. The optional memo is encrypted with the recipient’s note; the recipient and holders of applicable viewing keys can read it. Maximum 512 UTF-8 bytes.", []inputField{{"Recipient zkas: address", "", false}, {"Amount (ZKAS)", "", false}, {"Maximum fee (ZKAS)", "0.3", false}, {"Memo (optional)", "", false}})
		if !ok {
			return
		}
		to = strings.TrimSpace(values[0])
		memo = values[3]
		amount, e = wallet.ParseAmount(values[1])
		if e == nil {
			maxFee, e = wallet.ParseAmount(values[2])
		}
	}
	if e != nil {
		m.walletError(e)
		return
	}
	if !utf8.ValidString(memo) || len(memo) > 512 {
		m.walletError(fmt.Errorf("Memo must fit within 512 UTF-8 bytes"))
		return
	}
	saved := *w
	port := m.cfg.WalletPort
	m.action(func() error {
		if _, e := runSigner(m.root, map[string]any{"op": "address", "address": to}); e != nil {
			return e
		}
		c := wallet.Client{Port: port, Token: saved.Token}
		ctx, cancel := wallet.Timeout(10 * time.Minute)
		defer cancel()
		status, e := c.Status(ctx)
		if e != nil {
			return e
		}
		if !status.HasWallet {
			return fmt.Errorf("Click Refresh / sync first")
		}
		if status.Address != "" && status.Address != saved.Address {
			return fmt.Errorf("Wallet API returned a different wallet address; payment cancelled")
		}
		if status.Loading || status.MissingHistory || (!status.Synced && !status.SpendReady) {
			return fmt.Errorf("Wait for this wallet's history scan to finish before sending")
		}
		m.setProgress("Preparing payment. The first proof can take several minutes; nothing has been broadcast.")
		if consolidate {
			available, err := wallet.ParseAmount(status.Spendable)
			if err != nil || available <= maxFee {
				return fmt.Errorf("Not enough spendable balance above the fee reserve; nothing was sent")
			}
			amount = available - maxFee
			m.setProgress("Preparing one consolidation round. Nothing has been broadcast.")
		}
		prep, e := c.PrepareOptions(ctx, saved.FVK, to, amount, memo, consolidate)
		if e != nil {
			return e
		}
		detail := ""
		if consolidate {
			count, err := prep.ValidateConsolidation(amount, maxFee)
			if err != nil {
				return err
			}
			amount = prep.Amount
			detail = fmt.Sprintf("\nConsolidation: %d spendable notes into at most 2 notes.\nThis is a payment back to your own wallet. Funds need to mature again.\nOnly this round will run; any remaining notes stay in your wallet.\n", count)
		} else {
			if e = prep.Validate(amount, maxFee); e != nil {
				return e
			}
		}
		if memo != "" {
			detail += "\nEncrypted memo: " + memo + "\n"
		}
		var approved bool
		m.onUI(func() {
			approved = walk.MsgBox(m.window, "Review payment", fmt.Sprintf("From: %s\n\nTo:\n%s\n\nAmount: %s ZKAS\nActual fee: %s ZKAS\nMaximum allowed fee: %s ZKAS\n\nSign and broadcast this payment?", saved.Name, to, wallet.FormatAmount(amount), wallet.FormatAmount(prep.Fee), wallet.FormatAmount(maxFee))+detail, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes
		})
		if !approved {
			return nil
		}
		// The official signer checks recipient, amount, change and actual bundle fee.
		// Never fall back to blind signing a server-supplied sighash.
		signatures, e := runSigner(m.root, map[string]any{"op": "sign_payment", "seed": saved.Seed, "to": to, "amount": strconv.FormatUint(amount, 10), "maxFee": strconv.FormatUint(prep.Fee, 10), "bundle": prep.Bundle, "disclosureJson": string(prep.Disclosure), "spendAuthJson": string(prep.SpendAuth)})
		if e != nil {
			return e
		}
		if len(signatures.Sigs) == 0 {
			return fmt.Errorf("Signer returned no signatures")
		}
		record := wallet.Receipt{Memo: memo, Kind: "Payment", Time: time.Now().Format(time.RFC3339), To: to, Amount: wallet.FormatAmount(amount), Fee: wallet.FormatAmount(prep.Fee), State: "Submission outcome unknown — check chain history before retrying"}
		if consolidate {
			record.Kind = "Note consolidation"
		}
		selected := m.vault.Find(saved.ID)
		selected.Receipts = append(selected.Receipts, record)
		// Record the attempt durably BEFORE broadcasting so an interrupted response
		// cannot disappear when the user reopens the wallet. Never auto-retry submit.
		if e = wallet.Save(m.root, m.vaultPassword, m.vault); e != nil {
			selected.Receipts = selected.Receipts[:len(selected.Receipts)-1]
			return fmt.Errorf("Payment was NOT broadcast because its receipt could not be saved: %w", e)
		}
		m.setProgress("Broadcasting payment…")
		result, e := c.Submit(ctx, prep.Session, signatures.Sigs)
		if e != nil {
			m.onUI(m.renderSelectedWallet)
			return fmt.Errorf("Submission outcome is unknown. Do NOT resend automatically. Check wallet history first. Details: %w", e)
		}
		receipt := &selected.Receipts[len(selected.Receipts)-1]
		receipt.TxID = result.TxID
		receipt.State = "Broadcast — awaiting chain confirmation"
		if e = wallet.Save(m.root, m.vaultPassword, m.vault); e != nil {
			return fmt.Errorf("Payment broadcast with transaction ID %s, but saving the final receipt failed. Keep this ID and check history; do not resend. %w", result.TxID, e)
		}
		m.onUI(func() {
			m.renderSelectedWallet()
			m.showText("Payment broadcast", "Broadcast does not yet mean confirmed. Keep this transaction ID and check history.", result.TxID, false)
		})
		return nil
	})
}
