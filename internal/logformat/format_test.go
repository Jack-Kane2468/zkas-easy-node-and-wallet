package logformat

import (
	"math/rand"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	if s := Normalize("\x1b[31m[ERROR]\x1b[0m bad\r\r\nnext\r\n"); s != "[ERROR] bad\nnext\n" {
		t.Fatal(s)
	}
}
func TestColors(t *testing.T) {
	for s, w := range map[string]int{"[ERROR] failed": 3, "[WARN ] slow": 4, "[INFO ] IBD: 50%": 5, "[INFO ] connected": 6, "[DEBUG] details": 2, "ordinary error count: 0": 1} {
		if Color(s) != w {
			t.Fatal(s)
		}
	}
}
func apply(old string, c Change) string {
	if c.Reset {
		return c.Add
	}
	return old[c.Drop:] + c.Add
}
func TestDelta(t *testing.T) {
	for _, p := range [][2]string{{"", "first\n"}, {"first\n", "first\nsecond\n"}, {"old\nkeep\n", "keep\nnew\n"}, {"abcd", "xyz"}, {"abc", ""}, {"aaa", "aaaa"}, {"ababab", "abab"}, {"α\n😀\n", "😀\nβ\n"}, {"partial", "partial line\n"}} {
		c := Delta(p[0], p[1])
		if got := apply(p[0], c); got != p[1] {
			t.Fatalf("%q to %q: %+v => %q", p[0], p[1], c, got)
		}
	}
	c := Delta("existing\n", "existing\nnew\n")
	if c.Reset || c.Drop != 0 || c.Add != "new\n" {
		t.Fatal(c)
	}
}
func TestRollingTail(t *testing.T) {
	r := rand.New(rand.NewSource(9))
	old := ""
	for i := 0; i < 5000; i++ {
		next := old + strings.Repeat(string(rune('a'+r.Intn(4))), r.Intn(30)+1) + "\n"
		if len(next) > 4096 {
			next = next[strings.Index(next[512:], "\n")+513:]
		}
		if i%197 == 0 {
			next = "rotated\n"
		}
		c := Delta(old, next)
		if apply(old, c) != next {
			t.Fatal(i, c)
		}
		old = next
	}
}
func TestUnicodeFormatting(t *testing.T) {
	text := "2026-10-05 10:20:30 [ERROR] café 😀\n[WARN] next"
	r := Runs(text)
	if len(r) != 3 {
		t.Fatal(r)
	}
	if r[0].Color != 2 || r[1].Color != 3 || r[2].Color != 4 {
		t.Fatal(r)
	}
	if r[2].Start != Units(strings.Split(text, "\n")[0])+1 || r[2].End != Units(text) {
		t.Fatal(r)
	}
}
func TestUnits(t *testing.T) {
	if Units("a😀\nβ") != 5 {
		t.Fatal(Units("a😀\nβ"))
	}
}
