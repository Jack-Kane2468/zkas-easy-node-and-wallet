package telegrambot

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func paymentFixture(t *testing.T) (*Engine, *int, *PaymentInput, time.Time) {
	t.Helper()
	now := time.Now()
	n := new(int)
	in := new(PaymentInput)
	e := &Engine{Config: Config{OwnerID: 42, OwnerOnly: true, Permissions: Permissions{Wallets: true, Spend: true}}, Started: now.Add(-time.Second)}
	e.Prepare = func(_ context.Context, p PaymentInput) (PaymentReview, error) {
		*in = p
		return PaymentReview{Text: "Reviewed recipient, amount and fee", Commit: func(context.Context) (string, error) { *n++; return "TX receipt", nil }}, nil
	}
	return e, n, in, now
}
func enterPayment(t *testing.T, e *Engine, now time.Time) Reply {
	t.Helper()
	var r Reply
	for _, s := range []string{"/send zkas abcdefgh-wallet", "zkas:recipient", "12.5", "0.3", "hello memo"} {
		var err error
		r, err = e.Process(context.Background(), msg(42, s, now), now)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(r.Text, "Reviewed") {
		t.Fatal(r)
	}
	return r
}
func TestPaymentDeliveryConfirmationAndReplay(t *testing.T) {
	e, n, in, now := paymentFixture(t)
	r := enterPayment(t, e, now)
	if *n != 0 || in.Amount != "12.5" || in.To != "zkas:recipient" || in.Memo != "hello memo" || in.WalletID != "abcdefgh-wallet" {
		t.Fatal(n, in)
	}
	e.Process(context.Background(), msg(42, "confirm", now), now)
	if *n != 0 {
		t.Fatal("undelivered review accepted")
	}
	e.MarkDelivered(42, now)
	e.Process(context.Background(), msg(42, "confirm", now), now)
	if *n != 0 {
		t.Fatal("queued confirm accepted")
	}
	k, err := e.Keyboard(42, r.Choices, now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.ProcessCallback(context.Background(), callback(42, k[0][0].Data), now)
	if err != nil || result.Text != "TX receipt" || *n != 1 {
		t.Fatal(result, err, *n)
	}
	result, _ = e.ProcessCallback(context.Background(), callback(42, k[0][0].Data), now)
	if *n != 1 || result.Text != "" {
		t.Fatal("duplicate overwrote receipt or paid", result, *n)
	}
}
func TestPaymentRevocationCancellationExpiryAndIdentity(t *testing.T) {
	for _, mode := range []string{"revoke", "cancel", "menu", "expire", "other-user", "old-button"} {
		t.Run(mode, func(t *testing.T) {
			e, n, _, now := paymentFixture(t)
			r := enterPayment(t, e, now)
			k, _ := e.Keyboard(42, r.Choices, now)
			switch mode {
			case "revoke":
				e.Config.Permissions.Spend = false
			case "cancel":
				e.Process(context.Background(), msg(42, "/cancel", now), now)
			case "menu":
				e.Process(context.Background(), msg(42, "/menu", now), now)
			case "expire":
				e.payments[42].Expires = now.Add(-time.Second)
			case "old-button":
				enterPayment(t, e, now)
			case "other-user":
				e.Config.OwnerOnly = false
				e.Config.Allowed = []int64{99}
			}
			id := int64(42)
			if mode == "other-user" {
				id = 99
			}
			e.ProcessCallback(context.Background(), callback(id, k[0][0].Data), now)
			if *n != 0 {
				t.Fatal("unauthorized confirmation")
			}
		})
	}
}
func TestPaymentSimpleConfirmAndNoRetry(t *testing.T) {
	e, n, _, now := paymentFixture(t)
	enterPayment(t, e, now)
	e.MarkDelivered(42, now)
	e.payments[42].Review.Commit = func(context.Context) (string, error) { *n++; return "", errors.New("outcome unknown") }
	e.Process(context.Background(), msg(42, "/confirm old-service-code", now.Add(time.Second)), now.Add(time.Second))
	if *n != 0 {
		t.Fatal("service code accepted")
	}
	r, _ := e.Process(context.Background(), msg(42, "confirm", now.Add(time.Second)), now.Add(time.Second))
	if !strings.Contains(r.Text, "outcome unknown") || *n != 1 {
		t.Fatal(r, *n)
	}
	e.Process(context.Background(), msg(42, "confirm", now.Add(2*time.Second)), now.Add(2*time.Second))
	if *n != 1 {
		t.Fatal("auto retry")
	}
}
func TestKaspaPaymentAndOldPermission(t *testing.T) {
	e, n, in, now := paymentFixture(t)
	e.Config.Permissions.KaspaWallets = true
	for _, s := range []string{"/send kaspa wallet-id account-id", "kaspa:recipient", "1"} {
		e.Process(context.Background(), msg(42, s, now), now)
	}
	if in.WalletID != "wallet-id" || in.AccountID != "account-id" || in.Amount != "1" || *n != 0 {
		t.Fatal(in)
	}
	e.Config.Permissions.Spend = false
	e.Config.Permissions.Payments = true
	if e.permitted("send") {
		t.Fatal("old local permission grants spending")
	}
}
func TestNavigationDuringPaymentDoesNotBecomeInput(t *testing.T) {
	e, _, _, now := paymentFixture(t)
	e.Process(context.Background(), msg(42, "/send zkas abcdefgh", now), now)
	k, _ := e.Keyboard(42, [][]Choice{{{"Menu", "menu"}}}, now)
	e.ProcessCallback(context.Background(), callback(42, k[0][0].Data), now)
	if e.payments[42] != nil {
		t.Fatal("navigation retained payment")
	}
}
