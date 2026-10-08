package wallet

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestMemoAndPartialRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/wallet/prepare" || r.Header.Get("X-Wallet-Token") != "test-token" {
			t.Error("wrong endpoint/auth")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["memo"] != "hello 🌍" || body["amount_sompi"] != "18446744073709551615" || body["allow_partial"] != true {
			t.Error(body)
		}
		if _, ok := body["seed"]; ok {
			t.Error("seed sent")
		}
		w.Write([]byte(`{"amount_sompi":18446744073709551615}`))
	}))
	defer server.Close()
	_, ps, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	port, _ := strconv.Atoi(ps)
	c := Client{Port: port, Token: "test-token"}
	p, e := c.PrepareOptions(context.Background(), "fvk", "to", ^uint64(0), "hello 🌍", true)
	if e != nil || p.Amount != ^uint64(0) {
		t.Fatal(e)
	}
	if _, e = c.PrepareOptions(context.Background(), "fvk", "to", 1, strings.Repeat("🌍", 129), false); e == nil {
		t.Fatal("oversized memo accepted")
	}
	if calls != 1 {
		t.Fatal("invalid memo reached daemon")
	}
}
func TestConsolidationGuards(t *testing.T) {
	p := Prepared{Session: "session", Bundle: "bundle", Disclosure: json.RawMessage(`[{"out_value":90},{"out_value":0},{"out_value":0}]`), SpendAuth: json.RawMessage(`[{"index":0},{"index":1},{"index":2}]`), Amount: 90, Fee: 2, Remaining: 10}
	if n, e := p.ValidateConsolidation(100, 3); e != nil || n != 3 {
		t.Fatal(n, e)
	}
	bad := []Prepared{}
	q0 := p
	q0.Disclosure = json.RawMessage(`[{"out_value":30},{"out_value":30},{"out_value":30}]`)
	bad = append(bad, q0)
	q := p
	q.Remaining = 11
	bad = append(bad, q)
	q = p
	q.Fee = 4
	bad = append(bad, q)
	q = p
	q.Amount = 101
	bad = append(bad, q)
	q = p
	q.SpendAuth = json.RawMessage(`[{"index":0},{"index":1}]`)
	bad = append(bad, q)
	q = p
	q.SpendAuth = json.RawMessage(`[{"index":0},{"index":1},{"index":1}]`)
	bad = append(bad, q)
	q = p
	q.Bundle = ""
	bad = append(bad, q)
	for _, v := range bad {
		if _, e := v.ValidateConsolidation(100, 3); e == nil {
			t.Fatal("unsafe merge accepted", v)
		}
	}
	if p.Validate(100, 3) == nil {
		t.Fatal("ordinary send accepted partial consolidation")
	}
}
