package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Retain only a bounded stderr tail in memory. Never forward raw SDK diagnostics
// (which could include user input) into Telegram or write them to disk.
type walletProcessDiagnostic struct {
	mu   sync.Mutex
	tail []byte
	exit string
}

func (d *walletProcessDiagnostic) Write(b []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(b)
	if len(b) > 8192 {
		b = b[len(b)-8192:]
	}
	d.tail = append(d.tail, b...)
	if len(d.tail) > 8192 {
		d.tail = d.tail[len(d.tail)-8192:]
	}
	return n, nil
}
func (d *walletProcessDiagnostic) finish(code int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.exit = fmt.Sprintf("Exit code %d.", code)
}
func (d *walletProcessDiagnostic) reason() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := strings.ToLower(string(d.tail))
	reason := ""
	for _, v := range []struct{ match, text string }{
		{"cannot find module", "A required wallet runtime module could not be loaded. Extract the complete manager ZIP."},
		{"heap out of memory", "The wallet process ran out of memory."},
		{"memory allocation", "The wallet process could not allocate memory."},
		{"websocket is not defined", "The wallet runtime could not initialize WebSocket support."},
		{"eacces", "Windows denied access to a wallet/runtime file."},
		{"eperm", "Windows blocked a wallet/runtime file operation."},
		{"enoent", "A required wallet/runtime file was not found."},
		{"runtimeerror", "The Kaspa SDK reported a WebAssembly runtime failure."},
		{"unhandled", "The wallet runtime exited after an unhandled asynchronous error."},
		{"syntaxerror", "The wallet runtime reported a JavaScript syntax error."},
	} {
		if strings.Contains(s, v.match) {
			reason = v.text
			break
		}
	}
	if reason == "" {
		reason = "The wallet runtime exited without a recognized cause."
	}
	return strings.TrimSpace(reason+" "+d.exit) + safeWalletTrace(string(d.tail))
}
func (p *kaspaWalletProcess) alive() bool {
	if p == nil || p.stopped.Load() {
		return false
	}
	if p.backend != nil {
		return p.backend.alive()
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
func (p *kaspaWalletProcess) failure() error {
	if p.backend != nil {
		return p.backend.failure()
	}
	return fmt.Errorf("Kaspa wallet process stopped. %s Open the selected wallet to restart it. The Kaspa node is a separate process.", p.diagnostic.reason())
}

// Extract only code locations and fixed categories. Raw panic values, local paths,
// wallet IDs, keys and exception messages are deliberately not copied.
var walletTraceLocation = regexp.MustCompile(`wasm-function\[[0-9]+\]:0x[0-9a-fA-F]+|[A-Za-z0-9_.-]+\.(?:cjs|mjs|js|rs):[0-9]+(?::[0-9]+)?`)
var walletPhase = regexp.MustCompile(`WALLET_PHASE (open|start|connect|activate)`)

func safeWalletTrace(s string) string {
	out := ""
	phases := walletPhase.FindAllStringSubmatch(s, -1)
	if len(phases) > 0 {
		out += "\nLast wallet stage: " + phases[len(phases)-1][1]
	}
	for _, v := range []string{"unreachable", "memory access out of bounds", "table index is out of bounds", "BorrowMutError", "already borrowed", "assertion failed"} {
		if strings.Contains(s, v) {
			out += "\nRuntime detail: " + v
		}
	}
	seen := map[string]bool{}
	for _, v := range walletTraceLocation.FindAllString(s, -1) {
		if !seen[v] && len(seen) < 12 {
			out += "\n" + v
			seen[v] = true
		}
	}
	return out
}

func (d *walletProcessDiagnostic) trace() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return safeWalletTrace(string(d.tail))
}
