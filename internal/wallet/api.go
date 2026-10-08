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
	"unicode/utf8"
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
	NoteCount      *uint64 `json:"note_count"`
	Behind         uint64  `json:"blocks_behind"`
	Warming        bool    `json:"warming"`
	Error          string  `json:"error"`
	NodeConnected  bool    `json:"node_connected"`
	HasWallet      bool    `json:"has_wallet"`
	Address        string  `json:"address"`
	Synced         bool    `json:"synced"`
	SpendReady     bool    `json:"spend_ready"`
	Loading        bool    `json:"loading"`
	MissingHistory bool    `json:"missing_history"`
	Balance        string  `json:"balance_fc"`
	Spendable      string  `json:"spendable_fc"`
	Maturing       string  `json:"maturing_fc"`
	Scanned        uint64  `json:"scanned_blocks"`
	Chain          uint64  `json:"chain_len"`
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
	notes := "Not reported"
	if s.NoteCount != nil {
		notes = strconv.FormatUint(*s.NoteCount, 10)
	}
	return fmt.Sprintf("%s\r\nNotes: %s\r\nBalance: %s ZKAS   Available to send: %s ZKAS\r\nMaturing: %s ZKAS   Scan: %d / %d", state, notes, s.Balance, s.Spendable, s.Maturing, s.Scanned, s.Chain)
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
	return c.PrepareOptions(ctx, fvk, to, amount, "", false)
}
func (c Client) PrepareOptions(ctx context.Context, fvk, to string, amount uint64, memo string, partial bool) (Prepared, error) {
	var p Prepared
	if !utf8.ValidString(memo) || len(memo) > 512 {
		return p, fmt.Errorf("Memo must be valid text, at most 512 UTF-8 bytes")
	}
	e := c.Do(ctx, "POST", "/api/wallet/prepare", map[string]any{"fvk_hex": fvk, "to": to, "amount_sompi": strconv.FormatUint(amount, 10), "memo": memo, "allow_partial": partial}, &p)
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
			Memo       string `json:"memo"`
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
		if r.Memo != "" {
			out += "Memo: " + r.Memo + "\r\n"
		}
		out += "\r\n"
	}
	for _, r := range h.Pending {
		out += "Pending outgoing transaction: " + r.TxID + "\r\n"
	}
	return out
}

// A consolidation is a self-payment and may cover only one transaction-sized
// portion. Require a net reduction even when it creates recipient + change notes.
func (p Prepared) ValidateConsolidation(requested, maxFee uint64) (int, error) {
	if p.Amount == 0 || p.Amount > requested || p.Remaining != requested-p.Amount {
		return 0, fmt.Errorf("Invalid consolidation amount or remainder")
	}
	copy := p
	copy.Remaining = 0
	if e := copy.Validate(p.Amount, maxFee); e != nil {
		return 0, e
	}
	var auth []struct {
		Index uint64 `json:"index"`
	}
	if e := json.Unmarshal(p.SpendAuth, &auth); e != nil {
		return 0, e
	}
	if len(auth) < 3 || len(auth) > 38 {
		return 0, fmt.Errorf("Consolidation needs at least 3 spendable notes and at most 38 per round; nothing was sent")
	}
	var disclosure []struct {
		OutValue uint64 `json:"out_value"`
	}
	if e := json.Unmarshal(p.Disclosure, &disclosure); e != nil {
		return 0, e
	}
	outputs := 0
	for _, row := range disclosure {
		if row.OutValue > 0 {
			outputs++
		}
	}
	if outputs < 1 || outputs > 2 {
		return 0, fmt.Errorf("Consolidation must produce one or two non-empty notes")
	}
	seen := map[uint64]bool{}
	for _, v := range auth {
		if v.Index >= uint64(len(disclosure)) {
			return 0, fmt.Errorf("Consolidation input index is outside its disclosure")
		}
		if seen[v.Index] {
			return 0, fmt.Errorf("Duplicate consolidation input")
		}
		seen[v.Index] = true
	}
	return len(auth), nil
}
