package rotatinglog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSharingViolationDoesNotBreakOutput(t *testing.T) {
	p := filepath.Join(t.TempDir(), "wallet-console.log")
	w, e := New(p, 8)
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	w.Write([]byte("before\n"))
	w.rename = func(string, string) error { return errors.New("sharing violation") }
	if _, e = w.Write([]byte("during\n")); e != nil {
		t.Fatal("log pipe broken", e)
	}
	if _, e = w.Write([]byte("after\n")); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "before\nduring\nafter\n" {
		t.Fatal(string(b))
	}
	w.retryAt = time.Time{}
	w.rename = os.Rename
	if _, e = w.Write([]byte("rotated\n")); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(p + ".1")
	if string(b) != "before\nduring\nafter\n" {
		t.Fatal("lost previous log")
	}
	b, _ = os.ReadFile(p)
	if string(b) != "rotated\n" {
		t.Fatal(string(b))
	}
}
