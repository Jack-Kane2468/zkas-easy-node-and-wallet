package telegrambot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fixture() (*Engine, *int, time.Time) {
	now := time.Unix(1800000000, 0)
	calls := new(int)
	e := &Engine{Config: Config{OwnerOnly: true, OwnerID: 42, Permissions: Permissions{Status: true, Nodes: true}}, Started: now.Add(-time.Second), Handle: func(context.Context, Request) (string, error) { *calls++; return "handled", nil }}
	return e, calls, now
}
func msg(id int64, text string, now time.Time) Message {
	return Message{From: &User{ID: id}, Chat: Chat{ID: id, Type: "private"}, Text: text, Date: now.Unix()}
}
func TestUnauthorizedAndNonPrivateAreSilent(t *testing.T) {
	e, n, now := fixture()
	cases := []Message{msg(99, "/status", now), msg(42, "/status", now.Add(-3*time.Minute)), msg(42, "/status", now.Add(time.Minute))}
	group := msg(42, "/status", now)
	group.Chat.Type = "group"
	cases = append(cases, group)
	mismatch := msg(42, "/status", now)
	mismatch.Chat.ID = 99
	cases = append(cases, mismatch)
	robot := msg(42, "/status", now)
	robot.From.IsBot = true
	cases = append(cases, robot)
	missing := msg(42, "/status", now)
	missing.From = nil
	cases = append(cases, missing)
	for _, m := range cases {
		r, err := e.Process(context.Background(), m, now)
		if r.Text != "" || err != nil {
			t.Fatalf("not silent: %q %v", r.Text, err)
		}
	}
	if *n != 0 {
		t.Fatal("unauthorized handler call")
	}
}
func TestOwnerOnlyAndAllowlist(t *testing.T) {
	e, n, now := fixture()
	e.Config.Allowed = []int64{99}
	e.Process(context.Background(), msg(99, "/status", now), now)
	if *n != 0 {
		t.Fatal("owner-only bypass")
	}
	e.Config.OwnerOnly = false
	e.Process(context.Background(), msg(99, "/status", now), now)
	if *n != 1 {
		t.Fatal("allowed user rejected")
	}
	e.Process(context.Background(), msg(100, "/status", now), now)
	if *n != 1 {
		t.Fatal("unknown user allowed")
	}
}
func TestPermissionsAlsoApplyToDirectCommands(t *testing.T) {
	e, n, now := fixture()
	e.Config.Permissions = Permissions{FVK: true}
	for _, cmd := range []string{"/wallets", "/balance abcdefgh", "/payment abcdefgh", "/node stop zkas", "/mining", "/kaspa", "/receive abcdefgh"} {
		e.Process(context.Background(), msg(42, cmd, now), now)
	}
	if *n != 0 {
		t.Fatal("FVK-only access escaped into vault/service commands")
	}
	e.Process(context.Background(), msg(42, "/fvks", now), now)
	if *n != 1 {
		t.Fatal("FVK command unavailable")
	}
}
func TestConfirmationBoundToSenderExpiresAndConsumed(t *testing.T) {
	e, n, now := fixture()
	e.Config.OwnerOnly = false
	e.Config.Allowed = []int64{99}
	e.Process(context.Background(), msg(42, "/node stop zkas", now), now)
	if *n != 0 {
		t.Fatal("executed before confirmation")
	}
	code := e.pending[42].Code
	e.Process(context.Background(), msg(99, "/confirm "+code, now), now)
	if *n != 0 {
		t.Fatal("cross-user confirmation")
	}
	e.Process(context.Background(), msg(42, "/confirm "+code, now), now)
	if *n != 1 {
		t.Fatal("not executed")
	}
	e.Process(context.Background(), msg(42, "/confirm "+code, now), now)
	if *n != 1 {
		t.Fatal("replayed confirmation")
	}
	e.Process(context.Background(), msg(42, "/node stop zkas", now), now)
	code = e.pending[42].Code
	later := now.Add(3 * time.Minute)
	e.Process(context.Background(), msg(42, "/confirm "+code, later), later)
	if *n != 1 {
		t.Fatal("expired confirmation")
	}
}
func TestRevocationAndCancellation(t *testing.T) {
	e, n, now := fixture()
	e.Process(context.Background(), msg(42, "/node stop zkas", now), now)
	code := e.pending[42].Code
	e.Config.Permissions.Nodes = false
	e.Process(context.Background(), msg(42, "/confirm "+code, now), now)
	if *n != 0 {
		t.Fatal("revoked permission executed")
	}
	e.Config.Permissions.Nodes = true
	e.Process(context.Background(), msg(42, "/node stop zkas", now), now)
	code = e.pending[42].Code
	e.Process(context.Background(), msg(42, "/cancel", now), now)
	e.Process(context.Background(), msg(42, "/confirm "+code, now), now)
	if *n != 0 {
		t.Fatal("cancelled action executed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e.Process(ctx, msg(42, "/status", now), now)
	if *n != 0 {
		t.Fatal("stopped bot handled command")
	}
}
func TestPairRequiresCodeLocalApprovalAndIsOneTime(t *testing.T) {
	e, _, now := fixture()
	e.Config.OwnerID = 0
	e.PairCode = "secret"
	e.PairExpires = now.Add(time.Minute)
	e.Pair = func(int64) error { return errors.New("declined") }
	e.Process(context.Background(), msg(99, "/pair wrong", now), now)
	if e.Config.OwnerID != 0 {
		t.Fatal("bad pairing")
	}
	e.Process(context.Background(), msg(99, "/pair secret", now), now)
	if e.Config.OwnerID != 0 {
		t.Fatal("declined pairing saved")
	}
	e.Pair = func(id int64) error {
		if id != 99 {
			t.Fatal(id)
		}
		return nil
	}
	e.Process(context.Background(), msg(99, "/pair secret", now), now)
	if e.Config.OwnerID != 99 || e.PairCode != "" {
		t.Fatal("pairing failed")
	}
	e.Process(context.Background(), msg(42, "/pair secret", now), now)
	if e.Config.OwnerID != 99 {
		t.Fatal("owner stolen")
	}
}
func TestIDsAreNumericAndUse64Bits(t *testing.T) {
	ids, err := ParseIDs("4294967297, 42\n42")
	if err != nil || len(ids) != 2 || ids[0] != 4294967297 {
		t.Fatal(ids, err)
	}
	for _, v := range []string{"@name", "-100123", "0", "12.5", "1x"} {
		if _, e := ParseIDs(v); e == nil {
			t.Fatal("accepted", v)
		}
	}
}
func TestAPIErrorNeverIncludesTokenOrURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"ok":false,"error_code":401,"description":"secret-token"}`))
	}))
	defer server.Close()
	a := NewAPI("secret-token")
	a.base = server.URL
	_, err := a.Check(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), server.URL) {
		t.Fatal(err)
	}
}
func TestExistingWebhookNotRemoved(t *testing.T) {
	called := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = append(called, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "getMe") {
			w.Write([]byte(`{"ok":true,"result":{"username":"test"}}`))
		} else {
			w.Write([]byte(`{"ok":true,"result":{"url":"https://existing.example"}}`))
		}
	}))
	defer server.Close()
	a := NewAPI("secret")
	a.base = server.URL
	_, err := a.Check(context.Background())
	if err == nil || len(called) != 2 {
		t.Fatal(err, called)
	}
}
func TestRunDiscardsBacklogAndDeduplicatesUpdates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	polls, sends, handled := 0, 0, 0
	now := time.Now()
	e := &Engine{Config: Config{OwnerID: 42, OwnerOnly: true, Permissions: Permissions{Status: true}}, Started: now, Handle: func(context.Context, Request) (string, error) { handled++; return "ok", nil }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "sendMessage") {
			sends++
			w.Write([]byte(`{"ok":true,"result":{}}`))
			cancel()
			return
		}
		polls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if polls == 1 {
			if body["offset"] != float64(-1) {
				t.Error("backlog not skipped")
			}
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []Update{{ID: 50, Message: ptr(msg(42, "/node stop zkas", now))}}})
			return
		}
		if body["offset"] != float64(51) {
			t.Error("offset", body)
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []Update{{ID: 50, Message: ptr(msg(42, "/status", now))}, {ID: 51, Message: ptr(msg(99, "/status", now))}, {ID: 52, Message: ptr(msg(42, "/status", now))}}})
	}))
	defer server.Close()
	a := NewAPI("secret")
	a.base = server.URL
	a.Run(ctx, e, func(string) {})
	if sends != 1 || handled != 1 {
		t.Fatalf("sends=%d handled=%d", sends, handled)
	}
}
func ptr(m Message) *Message { return &m }
