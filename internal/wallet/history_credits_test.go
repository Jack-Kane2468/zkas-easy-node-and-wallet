package wallet

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReceivedCreditMemoAndOpeningError(t *testing.T) {
	data := []byte(`{"rows":[{"kind":"received","memo":null,"credits":[{"recipient":"test-address","memo":"test memo","amountSompiExact":"100"}]}]}`)
	var p HistoryPage
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if p.Rows[0].DisplayMemo() != "test memo" || !strings.Contains(HistoryDisplay(data), "test memo") {
		t.Fatal("received memo missing")
	}
	if !strings.Contains((Status{Loading: true, Error: "cannot verify checkpoint"}).Display(), "cannot verify checkpoint") {
		t.Fatal("hidden opening error")
	}
}
