package main

import (
	"os"
	"path/filepath"
	"testing"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/node"
)

func TestAllServiceIdentitiesAreSeparate(t *testing.T) {
	seen := map[string]bool{}
	for _, kind := range []string{"node", "wallet", "mining", "sharing", "kaspa", "dual"} {
		id := serviceIdentity(`C:\test\manager`, kind)
		if seen[id] {
			t.Fatal(kind)
		}
		seen[id] = true
	}
}
func TestKaspaDataAndPortConflicts(t *testing.T) {
	root := t.TempDir()
	c := node.Defaults(root)
	if e := checkKaspaIsolation(root, c); e != nil {
		t.Fatal(e)
	}
	c.DataDir = chains.DataDir(root)
	if checkKaspaIsolation(root, c) == nil {
		t.Fatal("reused Kaspa data")
	}
	c = node.Defaults(root)
	c.GRPC = 16110
	if checkKaspaIsolation(root, c) == nil {
		t.Fatal("conflicting RPC")
	}
}

// Only runs on a native Windows test runner with the harmless console fixture.
func TestKaspaHostIndependentOfZKas(t *testing.T) {
	managerPath, fakePath := os.Getenv("ZKAS_MANAGER_TEST_EXE"), os.Getenv("ZKAS_FAKE_NODE_EXE")
	if managerPath == "" || fakePath == "" {
		t.Skip("set Windows fixture paths")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	root := rootDir()
	os.MkdirAll(root, 0700)
	host, fake := filepath.Join(root, "ZKasNodeManager.exe"), filepath.Join(root, "fixture.exe")
	for src, dst := range map[string]string{managerPath: host, fakePath: fake} {
		b, e := os.ReadFile(src)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(dst, b, 0700); e != nil {
			t.Fatal(e)
		}
	}
	hash, e := node.FileHash(fake)
	if e != nil {
		t.Fatal(e)
	}
	c := node.Defaults(root)
	c.EnableWallet = false
	c.Executable = fake
	c.SHA256 = hash
	c.Version = "fixture"
	if e = node.Save(root, c); e != nil {
		t.Fatal(e)
	}
	s := chains.Config{Host: host, Mode: "kaspa", Kaspa: chains.Binary{Path: fake, SHA: hash, Version: "fixture"}}
	if e = chains.Save(root, s); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { stopKaspa(root); stopNode(root) })
	if e = startKaspa(root, c); e != nil {
		t.Fatal(e)
	}
	if e = startNode(root, c); e != nil {
		t.Fatal(e)
	}
	if e = stopNode(root); e != nil {
		t.Fatal(e)
	}
	if !serviceActive(root, "kaspa") {
		t.Fatal("ZKas stop also stopped Kaspa")
	}
	if e = stopKaspa(root); e != nil {
		t.Fatal(e)
	}
	if hostActive(root) || serviceActive(root, "kaspa") {
		t.Fatal("host remained active")
	}
}
