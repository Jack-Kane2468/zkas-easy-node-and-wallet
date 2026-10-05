package logformat

import (
	"fmt"
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
		level := strings.ToUpper(m[1] + m[2])
		switch level {
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
func escape(b *strings.Builder, s string) {
	for _, r := range s {
		switch r {
		case '\\', '{', '}':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\t':
			b.WriteString("\\tab ")
		default:
			if r >= 32 && r < 127 {
				b.WriteRune(r)
			} else if r >= 128 {
				for _, u := range utf16.Encode([]rune{r}) {
					fmt.Fprintf(b, "\\u%d?", int16(u))
				}
			}
		}
	}
}
func RTF(text string) string {
	var b strings.Builder
	b.WriteString(`{\rtf1\ansi\deff0\uc1{\fonttbl{\f0 Consolas;}}{\colortbl;\red226\green232\blue240;\red148\green163\blue184;\red255\green125\blue125;\red255\green203\blue107;\red103\green216\blue239;\red172\green221\blue188;}\f0\fs20 `)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\\par\n")
		}
		stamp := timestamp.FindString(line)
		if stamp != "" {
			b.WriteString("\\cf2 ")
			escape(&b, stamp)
			line = line[len(stamp):]
		}
		fmt.Fprintf(&b, "\\cf%d ", Color(line))
		escape(&b, line)
	}
	b.WriteByte('}')
	return b.String()
}
