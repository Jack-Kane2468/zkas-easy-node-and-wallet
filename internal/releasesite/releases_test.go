package releasesite

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, kind string) string {
	t.Helper()
	b, e := os.ReadFile("testdata/" + kind + ".html")
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestOfficialIndexes(t *testing.T) {
	for _, tc := range []struct{ kind, tag, hash string }{
		{"node", "zkas-v1.0.13", "bb4dea9613d90e0ed2d44d2c28b852c6373c2d6ac80ef51d47b6c1ac17f63948"},
		{"bridge", "v1.0.12", "97691d1f093c4fff5fe8ef88934893a062081bd99335ff7c243523ec0a81a00d"},
	} {
		p, e := Parse(strings.NewReader(fixture(t, tc.kind)), tc.kind)
		if e != nil || len(p) != 1 {
			t.Fatalf("%s: %v %v", tc.kind, p, e)
		}
		if p[0].Version != tc.tag || p[0].SHA != tc.hash {
			t.Fatalf("Wrong Windows asset: %+v", p[0])
		}
	}
}
func TestRejectMalformedIndex(t *testing.T) {
	good := fixture(t, "bridge")
	for name, bad := range map[string]string{
		"foreign host": strings.ReplaceAll(good, `href="v1.0.12/solo-dual-mode-windows-x64.zip"`, `href="https://evil.test/downloads/bridge/v1.0.12/solo-dual-mode-windows-x64.zip"`),
		"query":        strings.ReplaceAll(good, `windows-x64.zip"`, `windows-x64.zip?download=1"`),
		"bad hash":     strings.ReplaceAll(good, "97691d1f093c4fff5fe8ef88934893a062081bd99335ff7c243523ec0a81a00d", "bad"),
		"missing hash": strings.ReplaceAll(good, "<code>sha256 97691d1f093c4fff5fe8ef88934893a062081bd99335ff7c243523ec0a81a00d</code>", ""),
		"duplicate":    good + good,
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := Parse(strings.NewReader(bad), "bridge"); e == nil {
				t.Fatal("accepted malformed index")
			}
		})
	}
	// A hash from a different link cannot authenticate a Windows package.
	bad := `<table><tr><td><a href="v1.0.12/solo-dual-mode-windows-x64.zip">Windows</a><a href="linux.zip">Linux</a><code>sha256 ` + strings.Repeat("a", 64) + `</code></td></tr></table>`
	if _, e := Parse(strings.NewReader(bad), "bridge"); e == nil {
		t.Fatal("borrowed another asset's checksum")
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestLookupSelectionAndSize(t *testing.T) {
	page := fixture(t, "bridge")
	page += strings.ReplaceAll(page, "v1.0.12", "v1.0.9") // older version appears last
	for _, tc := range []struct {
		tag, want string
		size      int64
		fail      bool
	}{
		{"", "v1.0.12", 6354355, false}, {"v1.0.9", "v1.0.9", 6354355, false},
		{"v1.0.8", "", 6354355, true}, {"", "", -1, true}, {"", "", 101 << 20, true},
	} {
		client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
			if r.Method == "GET" {
				if r.URL.String() != Base+"bridge/" {
					t.Fatal(r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(page))}, nil
			}
			if r.Method != "HEAD" || r.URL.String() != PackageURL("bridge", tc.want, "solo-dual-mode-windows-x64.zip") && tc.want != "" {
				t.Fatalf("Unexpected request: %s %s", r.Method, r.URL)
			}
			return &http.Response{StatusCode: 200, ContentLength: tc.size, Body: io.NopCloser(strings.NewReader(""))}, nil
		})}
		p, e := lookup(context.Background(), client, "bridge", tc.tag)
		if (e != nil) != tc.fail {
			t.Fatalf("%+v: %v", tc, e)
		}
		if !tc.fail && (p.Version != tc.want || p.Size != tc.size) {
			t.Fatal(p)
		}
	}
}
func TestVersionOrdering(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"zkas-v1.0.13", "zkas-v1.0.9", true}, {"zkas-v1.0.13", "zkas-v1.0.13", false},
		{"zkas-v1.0.13", "zkas-v1.0.14", false}, {"v1.0.12", "v1.0.9", true},
	} {
		got, e := Newer(tc.a, tc.b)
		if e != nil || got != tc.want {
			t.Fatalf("%+v: %v %v", tc, got, e)
		}
	}
	if _, e := Newer("garbage", "zkas-v1.0.9"); e == nil {
		t.Fatal("invalid version accepted")
	}
}
