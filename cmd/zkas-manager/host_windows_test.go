package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"zkas-node-manager/internal/node"
)

// This test must run on Windows. It does not install startup entries or shortcuts.
func TestHostLifecycle(t *testing.T) {
	managerPath := os.Getenv("ZKAS_MANAGER_TEST_EXE")
	fakePath := os.Getenv("ZKAS_FAKE_NODE_EXE")
	if managerPath == "" || fakePath == "" {
		t.Skip("Build the manager and testnode fixture; set ZKAS_MANAGER_TEST_EXE and ZKAS_FAKE_NODE_EXE")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	root := rootDir()
	os.MkdirAll(root, 0700)
	copyFile := func(src, dest string) {
		b, e := os.ReadFile(src)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(dest, b, 0700); e != nil {
			t.Fatal(e)
		}
	}
	copyFile(managerPath, filepath.Join(root, "ZKasNodeManager.exe"))
	fakeDest := filepath.Join(root, "fixture.exe")
	copyFile(fakePath, fakeDest)
	c := node.Defaults(root)
	c.EnableWallet = false
	c.Executable = fakeDest
	c.SHA256, _ = node.FileHash(fakeDest)
	c.Version = "fixture"
	if e := node.Save(root, c); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { stopNode(root) })
	if e := startNode(root, c); e != nil {
		t.Fatal(e)
	}
	r, e := hostCommand(root, "status")
	if e != nil || !r.Running || r.Version != "fixture" {
		t.Fatalf("bad status: %+v %v", r, e)
	}
	if e = startNode(root, c); e == nil {
		t.Fatal("duplicate node allowed")
	}
	if e = stopNode(root); e != nil {
		t.Fatal(e)
	}
	if hostActive(root) {
		t.Fatal("host stayed active")
	}
	log, e := os.ReadFile(filepath.Join(root, "logs", "console.log"))
	if e != nil || !strings.Contains(string(log), "fixture graceful shutdown") {
		t.Fatalf("no graceful exit: %s %v", log, e)
	}
	if e = startNode(root, c); e != nil {
		t.Fatal("restart failed:", e)
	}
	if e = stopNode(root); e != nil {
		t.Fatal(e)
	}
}

func TestWalletStopsBeforeNode(t *testing.T) {
	managerPath, fakePath := os.Getenv("ZKAS_MANAGER_TEST_EXE"), os.Getenv("ZKAS_FAKE_NODE_EXE")
	if managerPath == "" || fakePath == "" {
		t.Skip("set Windows test fixture paths")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	root := rootDir()
	os.MkdirAll(root, 0700)
	trace := filepath.Join(root, "shutdown-order.txt")
	t.Setenv("ZKAS_TEST_TRACE", trace)
	for source, dest := range map[string]string{managerPath: filepath.Join(root, "ZKasNodeManager.exe"), fakePath: filepath.Join(root, "fixture.exe")} {
		b, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(dest, b, 0700); e != nil {
			t.Fatal(e)
		}
	}
	c := node.Defaults(root)
	c.Executable = filepath.Join(root, "fixture.exe")
	c.SHA256, _ = node.FileHash(c.Executable)
	c.WalletExecutable = c.Executable
	c.WalletSHA256 = c.SHA256
	c.Version = "fixture"
	if e := node.Save(root, c); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { stopNode(root) })
	if e := startNode(root, c); e != nil {
		t.Fatal(e)
	}
	if !serviceActive(root, "node") || !serviceActive(root, "wallet") {
		t.Fatal("missing service host")
	}
	if e := stopNode(root); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(trace)
	if e != nil || string(b) != "wallet\nnode\n" {
		t.Fatalf("wrong shutdown order: %q %v", b, e)
	}
}

func TestMiningHostWorksBesideExistingServices(t *testing.T) {
	managerPath, fakePath := os.Getenv("ZKAS_MANAGER_TEST_EXE"), os.Getenv("ZKAS_FAKE_NODE_EXE")
	if managerPath == "" || fakePath == "" {
		t.Skip("set Windows test fixture paths")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	root := rootDir()
	os.MkdirAll(root, 0700)
	trace := filepath.Join(root, "shutdown-order.txt")
	t.Setenv("ZKAS_TEST_TRACE", trace)
	for source, dest := range map[string]string{managerPath: filepath.Join(root, "ZKasNodeManager.exe"), fakePath: filepath.Join(root, "fixture.exe")} {
		b, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(dest, b, 0700); e != nil {
			t.Fatal(e)
		}
	}
	c := node.Defaults(root)
	c.Executable = filepath.Join(root, "fixture.exe")
	c.SHA256, _ = node.FileHash(c.Executable)
	c.WalletExecutable = c.Executable
	c.WalletSHA256 = c.SHA256
	c.Version = "fixture"
	if e := node.Save(root, c); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { stopNode(root) })
	if e := startNode(root, c); e != nil {
		t.Fatal(e)
	}
	mc := node.DefaultMining()
	mc.Executable = c.Executable
	mc.SHA256 = c.SHA256
	mc.HostExecutable = filepath.Join(root, "ZKasNodeManager.exe")
	mc.Version = c.Version
	if e := node.SaveMining(root, mc); e != nil {
		t.Fatal(e)
	}
	if e := startOne(root, c, "mining"); e != nil {
		t.Fatal(e)
	}
	if !serviceActive(root, "node") || !serviceActive(root, "wallet") || !serviceActive(root, "mining") {
		t.Fatal("missing service host")
	}
	if e := stopNode(root); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(trace)
	if e != nil || string(b) != "mining\nwallet\nnode\n" {
		t.Fatalf("wrong shutdown order: %q %v", b, e)
	}
}
