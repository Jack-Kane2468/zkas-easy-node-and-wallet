package wallet

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestSelectedHistoryUsesSelectedTokenAndOffset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Wallet-Token")
		if token != "wallet-a" && token != "wallet-b" {
			t.Errorf("wrong profile token %q", token)
			http.Error(w, "bad token", 403)
			return
		}
		if r.Method != "GET" {
			t.Errorf("history must be read only: %s", r.Method)
		}
		if r.URL.Path == "/api/status" {
			json.NewEncoder(w).Encode(Status{HasWallet: true, Address: token, Synced: true})
			return
		}
		if r.URL.Path != "/api/wallet/history" || r.URL.Query().Get("offset") != "500" || r.URL.Query().Get("limit") != "500" {
			t.Errorf("wrong page: %s", r.URL)
		}
		json.NewEncoder(w).Encode(HistoryPage{Total: 501, Recoverable: true, Rows: []HistoryRow{{Kind: "receive", TxID: token + "-tx", Amount: "123456789", Memo: "Invoice 123"}}})
	}))
	defer server.Close()
	_, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	for _, token := range []string{"wallet-a", "wallet-b"} {
		_, page, e := (Client{Port: port, Token: token}).SelectedHistory(context.Background(), token, 500)
		if e != nil || len(page.Rows) != 1 {
			t.Fatalf("history: %+v %v", page, e)
		}
		r := page.Rows[0]
		if r.TxID != token+"-tx" || r.Memo != "Invoice 123" || r.Amount != "123456789" {
			t.Fatalf("wrong wallet/details: %+v", r)
		}
	}
}

func TestSelectedHistoryRejectsWrongOrUnavailableWallet(t *testing.T) {
	for _, status := range []Status{{HasWallet: true, Address: "other"}, {Loading: true}, {HasWallet: false}, {HasWallet: true, Address: ""}} {
		t.Run(strconv.FormatBool(status.HasWallet)+status.Address+strconv.FormatBool(status.Loading), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/status" {
					t.Error("history requested before identity validation")
				}
				json.NewEncoder(w).Encode(status)
			}))
			defer server.Close()
			_, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
			port, _ := strconv.Atoi(portText)
			_, page, e := (Client{Port: port, Token: "selected"}).SelectedHistory(context.Background(), "expected", 0)
			if e == nil || len(page.Rows) > 0 {
				t.Fatalf("unverified wallet accepted: %v", e)
			}
		})
	}
}
