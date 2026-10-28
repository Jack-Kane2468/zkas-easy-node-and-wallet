package telegrambot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ledgerMu sync.Mutex

// State is separately encrypted so saving Bot settings cannot reset spend totals.
func saveState(root, name string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	defer clear(b)
	b, e = crypt(b, true)
	if e != nil {
		return e
	}
	dir := filepath.Join(root, "telegram-bot")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(dir, "state-*.tmp")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}
func loadState(root, name string, v any) error {
	b, e := os.ReadFile(filepath.Join(root, "telegram-bot", name))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	p, e := crypt(b, false)
	if e != nil {
		return e
	}
	defer clear(p)
	return json.Unmarshal(p, v)
}
func ReserveSpend(root string, l SpendingLimits, chain string, amount, fee uint64) error {
	if !l.Enabled {
		return nil
	}
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	var v SpendLedger
	if e := loadState(root, "spending.bin", &v); e != nil {
		return e
	}
	next, e := v.Reserve(l, chain, amount, fee, time.Now())
	if e != nil {
		return e
	}
	return saveState(root, "spending.bin", next)
}
func LoadMonitor(root string) (MonitorState, error) {
	var s MonitorState
	e := loadState(root, "monitor.bin", &s)
	return s, e
}
func SaveMonitor(root string, s MonitorState) error { return saveState(root, "monitor.bin", s) }
