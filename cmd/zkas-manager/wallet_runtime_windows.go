package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"zkas-node-manager/internal/node"
)

type signerResult struct {
	Secret  string          `json:"secret"`
	Seed    string          `json:"seed"`
	Address string          `json:"address"`
	FVK     string          `json:"fvk"`
	Valid   bool            `json:"valid"`
	Sigs    json.RawMessage `json:"sigs"`
}

func walletRuntime(root string) (string, error) {
	exe, e := os.Executable()
	if e != nil {
		return "", e
	}
	dir := filepath.Join(filepath.Dir(exe), "wallet-runtime")
	if _, e = os.Stat(filepath.Join(dir, "node.exe")); e != nil {
		dir = filepath.Join(root, "wallet-runtime")
	}
	for name, want := range walletRuntimeHashes {
		got, e := node.FileHash(filepath.Join(dir, name))
		if e != nil || got != want {
			return "", fmt.Errorf("Wallet runtime is missing or changed. Extract the complete package with its wallet-runtime folder, then open that EXE")
		}
	}
	return dir, nil
}
func stageWalletRuntime(root string) error {
	dir, e := walletRuntime(root)
	if e != nil {
		return e
	}
	dest := filepath.Join(root, "wallet-runtime")
	if strings.EqualFold(dir, dest) {
		return nil
	}
	if e = os.MkdirAll(dest, 0700); e != nil {
		return e
	}
	for name, want := range walletRuntimeHashes {
		out := filepath.Join(dest, name)
		if h, _ := node.FileHash(out); h == want {
			continue
		}
		b, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			return e
		}
		if e = os.WriteFile(out+".new", b, 0600); e != nil {
			return e
		}
		if e = os.Rename(out+".new", out); e != nil {
			return e
		}
	}
	return nil
}
func runSigner(root string, request any) (signerResult, error) {
	var result signerResult
	dir, e := walletRuntime(root)
	if e != nil {
		return result, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(dir, "node.exe"), "--no-warnings", filepath.Join(dir, "signer.mjs"))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Dir = dir
	// Prevent inherited NODE_OPTIONS from injecting code into a key-handling process.
	for _, v := range os.Environ() {
		upper := strings.ToUpper(v)
		if !strings.HasPrefix(upper, "NODE_") && !strings.HasPrefix(upper, "ELECTRON_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	b, e := json.Marshal(request)
	if e != nil {
		return result, e
	}
	defer clear(b)
	cmd.Stdin = bytes.NewReader(b)
	output, e := cmd.Output()
	defer clear(output)
	var reply struct {
		OK     bool         `json:"ok"`
		Error  string       `json:"error"`
		Result signerResult `json:"result"`
	}
	if json.Unmarshal(output, &reply) != nil {
		return result, fmt.Errorf("Local signer could not run; check the extracted runtime files")
	}
	if !reply.OK {
		return result, fmt.Errorf("Signer: %s", reply.Error)
	}
	if e != nil {
		return result, fmt.Errorf("Local signer failed")
	}
	return reply.Result, nil
}
