package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"zkas-node-manager/internal/rotatinglog"
)

var botLogMu sync.Mutex

// Callers pass fixed operational messages or sanitized SDK trace locations only; never Telegram content or raw API errors.
func (m *manager) botLog(level, message string) {
	botLogMu.Lock()
	defer botLogMu.Unlock()
	folder := filepath.Join(m.root, "logs")
	if os.MkdirAll(folder, 0700) != nil {
		return
	}
	w, err := rotatinglog.New(filepath.Join(folder, "bot.log"), 2<<20)
	if err != nil {
		return
	}
	defer w.Close()
	fmt.Fprintf(w, "%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000-07:00"), level, message)
}
