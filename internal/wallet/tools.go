package wallet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type HistoryRow struct {
	Kind      string `json:"kind"`
	TxID      string `json:"txid"`
	Timestamp uint64 `json:"timestamp"`
	DAA       uint64 `json:"daaScore"`
	Amount    string `json:"amountSompiExact"`
	Fee       string `json:"feeSompiExact"`
	FeeKnown  bool   `json:"feeKnown"`
	Recipient string `json:"recipient"`
	Memo      string `json:"memo"`
}
type HistoryPage struct {
	Total       int          `json:"total"`
	Rows        []HistoryRow `json:"rows"`
	Recoverable bool         `json:"recoverableHistory"`
}

func (c Client) HistoryPage(ctx context.Context, offset int) (HistoryPage, error) {
	var p HistoryPage
	e := c.Do(ctx, "GET", fmt.Sprintf("/api/wallet/history?limit=500&offset=%d", offset), nil, &p)
	return p, e
}
func ViewingToken(fvk string) string {
	s := sha256.Sum256([]byte("ZKas standalone view tool v1:" + fvk))
	return "viewtool-" + hex.EncodeToString(s[:24])
}
