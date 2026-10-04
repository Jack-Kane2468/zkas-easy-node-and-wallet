package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	Port  int
	Token string
}

func (c Client) Do(ctx context.Context, method, path string, body any, out any) error {
	var b []byte
	var e error
	if body != nil {
		b, e = json.Marshal(body)
		if e != nil {
			return e
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, "http://127.0.0.1:"+strconv.Itoa(c.Port)+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("X-Wallet-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	res, e := client.Do(req)
	if e != nil {
		return fmt.Errorf("Local wallet API unavailable: %w", e)
	}
	defer res.Body.Close()
	data, e := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if e != nil {
		return e
	}
	if res.StatusCode != 200 {
		var message struct {
			Error string `json:"error"`
		}
		json.Unmarshal(data, &message)
		if len(message.Error) > 600 {
			message.Error = message.Error[:600]
		}
		return fmt.Errorf("Wallet API HTTP %d: %s", res.StatusCode, message.Error)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}
func (c Client) Watch(ctx context.Context, fvk string) error {
	return c.Do(ctx, "POST", "/api/wallet/watch", map[string]any{"fvk_hex": fvk, "birthday": 0, "recoverable_history": true}, nil)
}

type Status struct {
	Behind         uint64 `json:"blocks_behind"`
	Warming        bool   `json:"warming"`
	Error          string `json:"error"`
	NodeConnected  bool   `json:"node_connected"`
	HasWallet      bool   `json:"has_wallet"`
	Address        string `json:"address"`
	Synced         bool   `json:"synced"`
	SpendReady     bool   `json:"spend_ready"`
	Loading        bool   `json:"loading"`
	MissingHistory bool   `json:"missing_history"`
	Balance        string `json:"balance_fc"`
	Spendable      string `json:"spendable_fc"`
	Maturing       string `json:"maturing_fc"`
	Scanned        uint64 `json:"scanned_blocks"`
	Chain          uint64 `json:"chain_len"`
}

func (c Client) Status(ctx context.Context) (Status, error) {
	var s Status
	e := c.Do(ctx, "GET", "/api/status", nil, &s)
	return s, e
}
func (s Status) Display() string {
	if s.Loading {
		return "Opening wallet. Balance is not available yet."
	}
	if !s.HasWallet {
		return "Not registered with the local wallet API yet. Click Refresh / sync."
	}
	state := "Scanning history…"
	if s.Synced || s.SpendReady {
		state = "Ready"
	}
	if s.MissingHistory {
		state = "History incomplete — displayed balance may be too low"
	}
	return fmt.Sprintf("%s\r\nBalance: %s ZKAS   Available to send: %s ZKAS\r\nMaturing: %s ZKAS   Scan: %d / %d", state, s.Balance, s.Spendable, s.Maturing, s.Scanned, s.Chain)
}

type Prepared struct {
	Session    string          `json:"session"`
	Bundle     string          `json:"bundle_hex"`
	Disclosure json.RawMessage `json:"disclosure"`
	SpendAuth  json.RawMessage `json:"spend_auth"`
	Amount     uint64          `json:"amount_sompi"`
	Fee        uint64          `json:"fee_sompi"`
	Remaining  uint64          `json:"remaining_sompi"`
}

func (c Client) Prepare(ctx context.Context, fvk, to string, amount uint64) (Prepared, error) {
	var p Prepared
	e := c.Do(ctx, "POST", "/api/wallet/prepare", map[string]any{"fvk_hex": fvk, "to": to, "amount_sompi": strconv.FormatUint(amount, 10), "allow_partial": false}, &p)
	return p, e
}
func (p Prepared) Validate(amount, maxFee uint64) error {
	if p.Session == "" || p.Bundle == "" || len(p.Disclosure) == 0 || len(p.SpendAuth) == 0 {
		return fmt.Errorf("Daemon did not provide a verifiable prepared payment; update it before sending")
	}
	if p.Amount != amount || p.Remaining != 0 {
		return fmt.Errorf("Daemon changed the requested amount or proposed a partial payment")
	}
	if p.Fee == 0 || p.Fee > maxFee {
		return fmt.Errorf("Prepared fee %s ZKAS exceeds the allowed limit or is invalid", FormatAmount(p.Fee))
	}
	return nil
}

type SubmitResult struct {
	TxID   string `json:"txid"`
	Amount uint64 `json:"amount_sompi"`
	Fee    uint64 `json:"fee_sompi"`
}

func (c Client) Submit(ctx context.Context, session string, sigs json.RawMessage) (SubmitResult, error) {
	var s SubmitResult
	e := c.Do(ctx, "POST", "/api/wallet/submit", map[string]any{"session": session, "sigs": sigs}, &s)
	if e == nil && s.TxID == "" {
		e = fmt.Errorf("No transaction ID returned; broadcast outcome is unknown")
	}
	return s, e
}
func Timeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func HistoryDisplay(data []byte) string {
	var h struct {
		Rows []struct {
			Kind       string `json:"kind"`
			TxID       string `json:"txid"`
			Timestamp  int64  `json:"timestamp"`
			Amount     uint64 `json:"amountSompi"`
			Recipient  string `json:"recipient"`
			AmountKind string `json:"amountKind"`
		} `json:"rows"`
		Pending []struct {
			TxID string `json:"txid"`
		} `json:"pendingOutgoing"`
	}
	if e := json.Unmarshal(data, &h); e != nil {
		return "History response could not be read."
	}
	out := "Recent on-chain history:\r\n"
	if len(h.Rows) == 0 {
		out += "No transactions found yet. A history scan may still be running.\r\n"
	}
	for _, r := range h.Rows {
		kind := r.Kind
		switch kind {
		case "received":
			kind = "Received"
		case "sent":
			kind = "Sent"
		case "coinbase":
			kind = "Mining reward"
		}
		if r.AmountKind == "netOutflow" {
			kind += " (net outflow)"
		}
		out += fmt.Sprintf("%s · %s · %s ZKAS\r\nTx: %s\r\n", time.UnixMilli(r.Timestamp).Local().Format("2006-01-02 15:04"), kind, FormatAmount(r.Amount), r.TxID)
		if r.Recipient != "" {
			out += "To: " + r.Recipient + "\r\n"
		}
		out += "\r\n"
	}
	for _, r := range h.Pending {
		out += "Pending outgoing transaction: " + r.TxID + "\r\n"
	}
	return out
}
