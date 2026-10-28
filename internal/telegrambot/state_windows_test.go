package telegrambot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDurableSpendReservation(t *testing.T) {
	root := t.TempDir()
	l := SpendingLimits{Enabled: true, ZKas: ChainLimit{Daily: "1"}}
	if e := ReserveSpend(root, l, "zkas", 80000000, 0); e != nil {
		t.Fatal(e)
	}
	if e := Save(root, Config{OwnerID: 42}); e != nil {
		t.Fatal(e)
	}
	if e := ReserveSpend(root, l, "zkas", 30000000, 0); e == nil {
		t.Fatal("settings save reset reservation")
	}
	if e := os.WriteFile(filepath.Join(root, "telegram-bot", "spending.bin"), []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := ReserveSpend(root, l, "zkas", 1, 0); e == nil {
		t.Fatal("corrupt ledger failed open")
	}
}
