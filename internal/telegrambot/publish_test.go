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

func TestMenuEditsAndFallback(t *testing.T) {
	for _, mode := range []string{"edit", "missing", "unchanged", "other-error"} {
		t.Run(mode, func(t *testing.T) {
			calls := []string{}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
				calls = append(calls, method)
				var b map[string]any
				json.NewDecoder(r.Body).Decode(&b)
				if method == "editMessageText" {
					if b["message_id"] != float64(123) {
						t.Error(b)
					}
					if len(b["reply_markup"].(map[string]any)["inline_keyboard"].([]any)) != 0 {
						t.Error("stale buttons retained")
					}
					switch mode {
					case "missing":
						w.Write([]byte(`{"ok":false,"error_code":400,"description":"message to edit not found"}`))
						return
					case "unchanged":
						w.Write([]byte(`{"ok":false,"error_code":400,"description":"message is not modified"}`))
						return
					case "other-error":
						w.Write([]byte(`{"ok":false,"error_code":500,"description":"secret detail"}`))
						return
					}
				}
				w.Write([]byte(`{"ok":true,"result":{"message_id":456}}`))
			}))
			defer s.Close()
			a := NewAPI("secret")
			a.base = s.URL
			mid, err := a.Publish(context.Background(), 42, 123, Reply{Text: "menu"}, nil)
			if mode == "missing" {
				if mid != 456 || len(calls) != 2 || calls[1] != "sendMessage" {
					t.Fatal(mid, calls)
				}
			} else {
				if len(calls) != 1 {
					t.Fatal(calls)
				}
			}
			if mode == "other-error" {
				if err == nil || strings.Contains(err.Error(), "secret") {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTypedCommandsSendFreshRepliesButtonsEdit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e, _, _ := fixture()
	now := time.Now()
	e.Started = now.Add(-time.Second)
	k, _ := e.Keyboard(42, [][]Choice{{{"Refresh", "status"}}}, now)
	polls, sends := 0, 0
	methods := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		switch method {
		case "getUpdates":
			polls++
			if polls == 1 {
				w.Write([]byte(`{"ok":true,"result":[]}`))
				return
			}
			m1 := msg(42, "/status", now)
			m2 := msg(42, "/status", now)
			cb := callback(42, k[0][0].Data)
			cb.Message.ID = 101
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []Update{{ID: 1, Message: &m1}, {ID: 2, Message: &m2}, {ID: 3, Callback: &cb}}})
		case "sendMessage":
			methods = append(methods, method)
			sends++
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": 99 + sends}})
		case "editMessageText":
			methods = append(methods, method)
			var b map[string]any
			json.NewDecoder(r.Body).Decode(&b)
			if b["message_id"] != float64(101) {
				t.Error(b)
			}
			w.Write([]byte(`{"ok":true,"result":true}`))
			cancel()
		default:
			w.Write([]byte(`{"ok":true,"result":true}`))
		}
	}))
	defer server.Close()
	a := NewAPI("secret")
	a.base = server.URL
	a.Run(ctx, e, func(string) {})
	if strings.Join(methods, ",") != "sendMessage,sendMessage,editMessageText" {
		t.Fatal(methods)
	}
}
