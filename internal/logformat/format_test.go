package logformat

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	in := "\x1b[31m[ERROR]\x1b[0m bad\r\r\nnext\r\n"
	if got := Normalize(in); got != "[ERROR] bad\nnext\n" {
		t.Fatal(got)
	}
}
func TestColors(t *testing.T) {
	for s, want := range map[string]int{"[ERROR] failed": 3, "[WARN ] slow": 4, "[INFO ] IBD: 50%": 5, "[INFO ] connected": 6, "[DEBUG] details": 2, "ordinary error count: 0": 1, "WARN slow": 4} {
		if got := Color(s); got != want {
			t.Fatalf("%s: %d", s, got)
		}
	}
}
func TestRTFEscapesLogContent(t *testing.T) {
	got := RTF("2026-10-05 10:20:30.001-05:00 [ERROR] {\\rtf1} café 😀\nnext")
	for _, s := range []string{"\\cf2 2026-10-05", "\\cf3 [ERROR]", "\\{\\\\rtf1\\}", "\\u233?", "\\u-10179?\\u-8704?", "\\par"} {
		if !strings.Contains(got, s) {
			t.Fatalf("missing %q: %s", s, got)
		}
	}
	if strings.Contains(got, "[ERROR] {\\rtf1}") {
		t.Fatal("unescaped RTF injection")
	}
}
