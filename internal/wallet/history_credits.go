package wallet

import "strings"

type HistoryCredit struct {
	Recipient string `json:"recipient"`
	Amount    string `json:"amountSompiExact"`
	Memo      string `json:"memo"`
}

func (r HistoryRow) DisplayMemo() string {
	parts := []string{}
	if r.Memo != "" {
		parts = append(parts, r.Memo)
	}
	seen := map[string]bool{}
	for _, c := range r.Credits {
		if c.Memo == "" || c.Memo == r.Memo {
			continue
		}
		line := c.Memo
		if len(r.Credits) > 1 {
			line = c.Recipient + ": " + c.Memo
		}
		if !seen[line] {
			parts = append(parts, line)
			seen[line] = true
		}
	}
	return strings.Join(parts, "\r\n")
}
func (r HistoryRow) CreditDetails() string {
	var out strings.Builder
	for _, c := range r.Credits {
		out.WriteString("\r\nReceived output\r\nAddress: " + c.Recipient + "\r\nAmount (sompi): " + c.Amount + "\r\n")
		if c.Memo != "" {
			out.WriteString("Memo: " + c.Memo + "\r\n")
		}
	}
	return out.String()
}
