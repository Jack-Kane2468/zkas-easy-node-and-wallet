package telegrambot

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
	"zkas-node-manager/internal/wallet"
)

type PaymentInput struct {
	UserID                                               int64
	Chain, WalletID, AccountID, To, Amount, MaxFee, Memo string
}
type PaymentReview struct {
	Text   string
	Commit func(context.Context) (string, error)
}
type paymentFlow struct {
	Input     PaymentInput
	Step      int
	Expires   time.Time
	Code      string
	Review    PaymentReview
	Delivered int64
}

func (e *Engine) MarkDelivered(id int64, now time.Time) {
	if p := e.payments[id]; p != nil && p.Code != "" && p.Delivered == 0 {
		p.Delivered = now.Unix()
	}
}
func paymentPrompt(s string) Reply {
	return Reply{Text: s, Choices: [][]Choice{{{"Cancel", "cancel"}}}}
}
func (e *Engine) processPayment(ctx context.Context, m Message, cmd string, f []string, now time.Time) (Reply, bool, error) {
	id := m.From.ID
	for uid, p := range e.payments {
		if !now.Before(p.Expires) {
			delete(e.payments, uid)
		}
	}
	if cmd == "menu" || cmd == "start" || cmd == "cancel" {
		delete(e.payments, id)
		return Reply{}, false, nil
	}
	if cmd == "send" {
		if !e.permitted("send") {
			return paymentPrompt("Telegram sending is disabled. Enable Send from Telegram in the manager's Bot tab."), true, nil
		}
		if len(f) < 3 || (f[1] != "zkas" && f[1] != "kaspa") || (f[1] == "zkas" && len(f) != 3) || (f[1] == "kaspa" && len(f) != 4) {
			return paymentPrompt("Choose Send from a wallet menu, or /send zkas WALLET_ID or /send kaspa WALLET_ID ACCOUNT_ID."), true, nil
		}
		if e.payments == nil {
			e.payments = map[int64]*paymentFlow{}
		}
		if len(e.payments) >= 4 && e.payments[id] == nil {
			return paymentPrompt("Other payment reviews are active. Try again shortly."), true, nil
		}
		if !e.Config.Permissions.Chain(f[1]) {
			return paymentPrompt("Wallet access for this chain is disabled."), true, nil
		}
		in := PaymentInput{UserID: id, Chain: f[1], WalletID: f[2]}
		if len(f) == 4 {
			in.AccountID = f[3]
		}
		delete(e.pending, id)
		e.payments[id] = &paymentFlow{Input: in, Expires: now.Add(10 * time.Minute)}
		return paymentPrompt("Enter the recipient's " + in.Chain + ": address."), true, nil
	}
	p := e.payments[id]
	if p == nil {
		if cmd == "pay_confirm" {
			return Reply{}, true, nil
		}
		return Reply{}, false, nil
	}
	if !e.permitted("send") || !e.Config.Permissions.Chain(p.Input.Chain) {
		delete(e.payments, id)
		return paymentPrompt("Sending permission was revoked."), true, nil
	}
	if p.Code != "" {
		if cmd == "pay_confirm" || cmd == "confirm" {
			valid := cmd == "pay_confirm" && len(f) == 2 && f[1] == p.Code
			if cmd == "confirm" && len(f) == 1 {
				valid = p.Delivered > 0 && m.Date > p.Delivered
			}
			if !valid {
				return paymentPrompt("Review this payment first, then tap its Confirm payment button or send confirm again."), true, nil
			}
			delete(e.payments, id) // Consume before signing, even when submission fails.
			if e.Working != nil {
				e.Working(id, "Submitting the confirmed payment… Do not resend while waiting.")
			}
			result, err := p.Review.Commit(ctx)
			if err != nil {
				result = err.Error()
			}
			return Reply{Text: result, Choices: e.mainMenu()}, true, nil
		}
		if strings.HasPrefix(m.Text, "/") {
			delete(e.payments, id)
			return Reply{}, false, nil
		}
		return Reply{Text: p.Review.Text, Choices: [][]Choice{{{"Confirm payment", "pay_confirm " + p.Code}, {"Cancel", "cancel"}}}}, true, nil
	}
	if strings.HasPrefix(m.Text, "/") && cmd != "no_memo" {
		delete(e.payments, id)
		return Reply{}, false, nil
	}
	v := strings.TrimSpace(m.Text)
	switch p.Step {
	case 0:
		if !strings.HasPrefix(v, p.Input.Chain+":") || len(v) > 256 || strings.ContainsAny(v, " \t\r\n") {
			return paymentPrompt("Enter a mainnet " + p.Input.Chain + ": address."), true, nil
		}
		p.Input.To = v
		p.Step = 1
		return paymentPrompt("Enter the amount in " + strings.ToUpper(p.Input.Chain) + "."), true, nil
	case 1:
		n, err := wallet.ParseAmount(v)
		if err != nil || n == 0 {
			return paymentPrompt("Enter a positive amount with up to 8 decimal places."), true, nil
		}
		p.Input.Amount = v
		if p.Input.Chain == "zkas" {
			p.Step = 2
			return paymentPrompt("Enter the maximum fee in ZKAS (for example 0.3). You will review the actual fee before sending."), true, nil
		}
	case 2:
		if _, err := wallet.ParseAmount(v); err != nil {
			return paymentPrompt("Enter a valid maximum fee in ZKAS."), true, nil
		}
		p.Input.MaxFee = v
		p.Step = 3
		return Reply{Text: "Enter an optional memo (up to 512 UTF-8 bytes), or tap No memo.", Choices: [][]Choice{{{"No memo", "no_memo"}, {"Cancel", "cancel"}}}}, true, nil
	case 3:
		if cmd != "no_memo" {
			if len(m.Text) > 512 || !utf8.ValidString(m.Text) {
				return paymentPrompt("Memo must fit in 512 UTF-8 bytes."), true, nil
			}
			p.Input.Memo = m.Text
		}
	}
	delete(e.payments, id)
	if e.Prepare == nil {
		return paymentPrompt("Payment controller unavailable."), true, nil
	}
	if e.Working != nil {
		e.Working(id, "Preparing your payment review… Nothing has been sent. ZKas proof preparation can take several minutes.")
	}
	review, err := e.Prepare(ctx, p.Input)
	if err != nil {
		return paymentPrompt("Payment was not submitted: " + err.Error()), true, nil
	}
	if review.Commit == nil {
		return paymentPrompt("Payment review unavailable; nothing sent."), true, nil
	}
	code, err := Secret()
	if err != nil {
		return Reply{}, true, err
	}
	p.Code = code
	p.Review = review
	p.Expires = time.Now().Add(2 * time.Minute)
	e.payments[id] = p
	return Reply{Text: fmt.Sprintf("%s\n\nConfirm within 2 minutes to sign and broadcast. Broadcast is not yet chain confirmation.", review.Text), Choices: [][]Choice{{{"Confirm payment", "pay_confirm " + code}, {"Cancel", "cancel"}}}}, true, nil
}
