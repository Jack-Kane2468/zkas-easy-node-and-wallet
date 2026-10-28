package telegrambot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func callback(id int64, data string) Callback {
	return Callback{ID: "cb", From: &User{ID: id}, Message: &Message{Chat: Chat{ID: id, Type: "private"}}, Data: data}
}
func TestPlainConfirmAndCancel(t *testing.T) {
	e, n, now := fixture()
	prompt, err := e.Process(context.Background(), msg(42, "/node stop zkas", now), now)
	if err != nil || len(prompt.Choices) != 1 || prompt.Choices[0][0].Text != "Confirm" {
		t.Fatal(prompt, err)
	}
	code := e.pending[42].Code
	if strings.Contains(prompt.Text, code) {
		t.Fatal("visible confirmation code")
	}
	e.Process(context.Background(), msg(42, "Confirm", now), now)
	if *n != 1 {
		t.Fatal("plain confirm did not work")
	}
	e.Process(context.Background(), msg(42, "confirm", now), now)
	if *n != 1 {
		t.Fatal("replayed plain confirm")
	}
	e.Process(context.Background(), msg(42, "/node stop zkas", now), now)
	e.Process(context.Background(), msg(42, "cancel", now), now)
	e.Process(context.Background(), msg(42, "/confirm", now), now)
	if *n != 1 {
		t.Fatal("cancelled action executed")
	}
}
func TestMenusOnlyShowAllowedSections(t *testing.T) {
	e, _, now := fixture()
	e.Config.Permissions = Permissions{FVK: true}
	reply, _ := e.Process(context.Background(), msg(42, "/menu", now), now)
	var commands []string
	for _, row := range reply.Choices {
		for _, b := range row {
			commands = append(commands, b.Command)
		}
	}
	if strings.Join(commands, ",") != "fvks,help" {
		t.Fatal(commands)
	}
	reply, _ = e.Process(context.Background(), msg(42, "nodes", now), now)
	if strings.Contains(reply.Text, "Node controls.") {
		t.Fatal("disabled submenu opened")
	}
}
func TestButtonsKeepSenderAndPendingRequestBinding(t *testing.T) {
	e, n, now := fixture()
	e.Config.OwnerOnly = false
	e.Config.Allowed = []int64{99}
	prompt, _ := e.Process(context.Background(), msg(42, "node stop zkas", now), now)
	keys, err := e.Keyboard(42, prompt.Choices, now)
	if err != nil {
		t.Fatal(err)
	}
	oldConfirm, oldCancel := keys[0][0].Data, keys[0][1].Data
	if len(oldConfirm) > 64 {
		t.Fatal("callback too long")
	}
	e.ProcessCallback(context.Background(), callback(99, oldConfirm), now)
	if *n != 0 {
		t.Fatal("cross-user button accepted")
	}
	group := callback(42, oldConfirm)
	group.Message.Chat.Type = "supergroup"
	r, _ := e.ProcessCallback(context.Background(), group, now)
	if r.Text != "" {
		t.Fatal("group got reply")
	}
	prompt, _ = e.Process(context.Background(), msg(42, "node stop kaspa", now), now)
	e.ProcessCallback(context.Background(), callback(42, oldCancel), now)
	if _, ok := e.pending[42]; !ok {
		t.Fatal("old cancel removed new request")
	}
	e.ProcessCallback(context.Background(), callback(42, oldConfirm), now)
	if *n != 0 {
		t.Fatal("old button approved new request")
	}
	keys, _ = e.Keyboard(42, prompt.Choices, now)
	fresh := keys[0][0].Data
	e.ProcessCallback(context.Background(), callback(42, fresh), now)
	if *n != 1 {
		t.Fatal("fresh confirmation failed")
	}
	e.ProcessCallback(context.Background(), callback(42, fresh), now)
	if *n != 1 {
		t.Fatal("confirmation button replayed")
	}
}
func TestCallbackNavigationExpiryRestartAndRevocation(t *testing.T) {
	e, n, now := fixture()
	keys, _ := e.Keyboard(42, [][]Choice{{{"Status", "status"}}}, now)
	data := keys[0][0].Data
	r, err := e.ProcessCallback(context.Background(), callback(42, data), now)
	if err != nil || r.Text != "handled" || *n != 1 {
		t.Fatal(r, err, *n)
	}
	e.Config.Permissions.Status = false
	e.ProcessCallback(context.Background(), callback(42, data), now)
	if *n != 1 {
		t.Fatal("revoked callback executed")
	}
	e.Config.Permissions.Status = true
	e.ProcessCallback(context.Background(), callback(42, data), now.Add(31*time.Minute))
	if *n != 1 {
		t.Fatal("expired menu executed")
	}
	e.buttons = nil
	e.ProcessCallback(context.Background(), callback(42, data), now)
	if *n != 1 {
		t.Fatal("pre-restart callback executed")
	}
	e.ProcessCallback(context.Background(), callback(42, "node stop zkas"), now)
	if *n != 1 {
		t.Fatal("unissued callback accepted")
	}
}
func TestWalletMenusPreserveFullStableIDAndPermissions(t *testing.T) {
	e, _, now := fixture()
	e.Config.Permissions = Permissions{Wallets: true}
	e.Menu = func(context.Context, Request) [][]Choice {
		return [][]Choice{{{"My wallet", "wallet stable-wallet-id"}}}
	}
	reply, _ := e.Process(context.Background(), msg(42, "/wallets", now), now)
	if reply.Choices[0][0].Text != "My wallet" || reply.Choices[0][0].Command != "wallet stable-wallet-id" {
		t.Fatal(reply)
	}
	reply, _ = e.Process(context.Background(), msg(42, "wallet stable-wallet-id", now), now)
	for _, row := range reply.Choices {
		for _, b := range row {
			if strings.HasPrefix(b.Command, "payment") {
				t.Fatal("payment button without permission")
			}
			if strings.HasPrefix(b.Command, "balance") && b.Command != "balance stable-wallet-id" {
				t.Fatal("wallet identity changed")
			}
		}
	}
}
func TestCopyableMessageAndInlineMarkup(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()
	a := NewAPI("secret")
	a.base = server.URL
	err := a.Send(context.Background(), 42, Reply{Text: "copy this address"}, [][]Button{{{Text: "Menu", Data: "hidden-id"}}})
	if err != nil {
		t.Fatal(err)
	}
	if body["protect_content"] == true {
		t.Fatal("copying blocked")
	}
	if _, ok := body["parse_mode"]; ok {
		t.Fatal("untrusted data interpreted as markup")
	}
	rows := body["reply_markup"].(map[string]any)["inline_keyboard"].([]any)
	button := rows[0].([]any)[0].(map[string]any)
	if button["callback_data"] != "hidden-id" || button["text"] != "Menu" {
		t.Fatal(body)
	}
}
func TestCallbackPollingAcknowledgesBeforeHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e, n, _ := fixture()
	now := time.Now()
	e.Started = now
	keys, _ := e.Keyboard(42, [][]Choice{{{"Status", "status"}}}, now)
	calls := []string{}
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		calls = append(calls, method)
		switch method {
		case "getUpdates":
			polls++
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			types := body["allowed_updates"].([]any)
			if len(types) != 2 || types[1] != "callback_query" {
				t.Error("callbacks not requested")
			}
			if polls == 1 {
				w.Write([]byte(`{"ok":true,"result":[]}`))
				return
			}
			cb := callback(42, keys[0][0].Data)
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []Update{{ID: 1, Callback: &cb}}})
		case "answerCallbackQuery":
			if *n != 0 {
				t.Error("callback not acknowledged before handler")
			}
			w.Write([]byte(`{"ok":true,"result":true}`))
		case "sendMessage":
			w.Write([]byte(`{"ok":true,"result":{}}`))
			cancel()
		}
	}))
	defer server.Close()
	a := NewAPI("secret")
	a.base = server.URL
	a.Run(ctx, e, func(string) {})
	if *n != 1 || strings.Join(calls, ",") != "getUpdates,getUpdates,answerCallbackQuery,sendMessage" {
		t.Fatal(*n, calls)
	}
}
