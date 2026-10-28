package main

import (
	"strings"
	"testing"
)

func TestStoppedWalletCallDoesNotWrite(t *testing.T) {
	p := &kaspaWalletProcess{done: make(chan struct{})}
	p.stopped.Store(true)
	if p.alive() {
		t.Fatal("stopped process considered reusable")
	}
	if e := p.call(map[string]any{"op": "status"}, nil); e == nil || !strings.Contains(e.Error(), "Open the selected wallet") {
		t.Fatal(e)
	}
	p.stopped.Store(false)
	close(p.done)
	if p.alive() {
		t.Fatal("exited process considered reusable")
	}
}
func TestWalletDiagnosticsBoundedAndRedacted(t *testing.T) {
	d := &walletProcessDiagnostic{}
	s := strings.Repeat("private-test-input ", 1000) + "\nCannot find module 'private-path'"
	d.Write([]byte(s))
	d.finish(1)
	if len(d.tail) > 8192 {
		t.Fatal("unbounded diagnostics")
	}
	r := d.reason()
	if !strings.Contains(r, "module") || !strings.Contains(r, "Exit code 1") || strings.Contains(r, "private-") {
		t.Fatal(r)
	}
}

func TestSafeWalletTraceKeepsLocationsNotInput(t *testing.T) {
	raw := "WALLET_PHASE open\nWALLET_PHASE activate\nRuntimeError: unreachable\n at secret (C:\\Users\\private-user\\wallet-runtime\\kaspa.cjs:123:4)\n at wasm://wasm/abcdef:wasm-function[42]:0x1234\nseed=private-key-value"
	text := safeWalletTrace(raw)
	for _, want := range []string{"activate", "unreachable", "kaspa.cjs:123:4", "wasm-function[42]:0x1234"} {
		if !strings.Contains(text, want) {
			t.Fatal(text)
		}
	}
	for _, bad := range []string{"private-user", "private-key-value", "C:\\Users", "seed="} {
		if strings.Contains(text, bad) {
			t.Fatal(text)
		}
	}
}
