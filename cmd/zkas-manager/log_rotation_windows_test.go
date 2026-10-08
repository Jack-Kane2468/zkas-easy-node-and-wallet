package main

import (
	"os"
	"path/filepath"
	"testing"
	"zkas-node-manager/internal/rotatinglog"
)

func TestLogViewerAllowsWindowsRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet-console.log")
	w, e := rotatinglog.New(path, 8)
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	if _, e = w.Write([]byte("before\n")); e != nil {
		t.Fatal(e)
	}
	reader, e := openSharedLog(path)
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	if _, e = w.Write([]byte("after\n")); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path + ".1")
	if e != nil || string(b) != "before\n" {
		t.Fatal("open viewer prevented rotation", e)
	}
}
