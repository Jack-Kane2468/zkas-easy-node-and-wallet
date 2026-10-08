package logformat

import (
	"regexp"
	"strings"
	"unicode/utf16"
)

var ansi = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")
var timestamp = regexp.MustCompile(`^(?:\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?\s*)`)
var severity = regexp.MustCompile(`(?i)\[\s*(ERROR|FATAL|PANIC|WARN(?:ING)?|INFO|DEBUG|TRACE)\s*\]|^(ERROR|FATAL|PANIC|WARN(?:ING)?|INFO|DEBUG|TRACE)\b`)

func Normalize(s string) string {
	return strings.ToValidUTF8(ansi.ReplaceAllString(strings.ReplaceAll(s, "\r", ""), ""), "�")
}
func Color(s string) int {
	m := severity.FindStringSubmatch(s)
	if len(m) > 0 {
		switch strings.ToUpper(m[1] + m[2]) {
		case "ERROR", "FATAL", "PANIC":
			return 3
		case "WARN", "WARNING":
			return 4
		case "DEBUG", "TRACE":
			return 2
		}
	}
	if strings.Contains(s, "IBD:") || strings.Contains(strings.ToLower(s), "sync progress") {
		return 5
	}
	if len(m) > 0 && strings.EqualFold(m[1]+m[2], "INFO") {
		return 6
	}
	return 1
}
func Units(s string) int32 {
	var n int32
	for _, r := range s {
		n += int32(utf16.RuneLen(r))
	}
	return n
}

type Run struct {
	Start, End int32
	Color      int
}

func Runs(s string) []Run {
	var out []Run
	var pos int32
	for _, line := range strings.Split(s, "\n") {
		stamp := timestamp.FindString(line)
		t := Units(stamp)
		end := pos + Units(line)
		if t > 0 {
			out = append(out, Run{pos, pos + t, 2})
		}
		if end > pos+t {
			out = append(out, Run{pos + t, end, Color(line[len(stamp):])})
		}
		pos = end + 1
	}
	return out
}

type Change struct {
	Drop  int
	Add   string
	Reset bool
}

// Find rolling-tail overlap in linear time, including repeated lines.
func Delta(old, next string) Change {
	if strings.HasPrefix(next, old) {
		return Change{Add: next[len(old):]}
	}
	if next == "" {
		return Change{Reset: true}
	}
	pi := make([]int32, len(next))
	for i, j := 1, 0; i < len(next); i++ {
		for j > 0 && next[i] != next[j] {
			j = int(pi[j-1])
		}
		if next[i] == next[j] {
			j++
		}
		pi[i] = int32(j)
	}
	j := 0
	for i := 0; i < len(old); i++ {
		for j > 0 && (j == len(next) || old[i] != next[j]) {
			j = int(pi[j-1])
		}
		if old[i] == next[j] {
			j++
		}
	}
	if j == 0 {
		return Change{Add: next, Reset: true}
	}
	return Change{Drop: len(old) - j, Add: next[j:]}
}
