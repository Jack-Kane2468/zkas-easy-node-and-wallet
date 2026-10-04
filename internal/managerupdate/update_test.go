package managerupdate

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersions(t *testing.T) {
	vs := []string{"0.8.3-preview", "0.8.3", "0.8.4-preview.2", "0.8.4-preview.10", "0.8.4", "0.9.0", "0.10.0", "1.0.0"}
	for i := range vs {
		a, e := ParseVersion(vs[i])
		if e != nil {
			t.Fatal(e)
		}
		for j := range vs {
			b, _ := ParseVersion(vs[j])
			n := Compare(a, b)
			if i < j && n >= 0 || i == j && n != 0 || i > j && n <= 0 {
				t.Fatalf("%s / %s", vs[i], vs[j])
			}
		}
	}
	for _, s := range []string{"../x", "1.2", "01.2.3", "1.2.3-01"} {
		if _, e := ParseVersion(s); e == nil {
			t.Fatal(s)
		}
	}
}
func release(tag string, pre bool) Release {
	n := "ZKasNodeManager-Windows-x64-" + strings.TrimPrefix(tag, "v") + ".zip"
	b := "https://github.com/" + Repository + "/releases/download/" + tag + "/"
	return Release{Tag: tag, Preview: pre, Assets: []Asset{{Name: n, URL: b + n, Size: 200}, {Name: "SHA256SUMS.txt", URL: b + "SHA256SUMS.txt", Size: 110}}}
}
func TestSelection(t *testing.T) {
	rs := []Release{release("v0.8.4-preview", true), release("v0.8.3", false)}
	c, e := Select(rs, "0.8.3-preview", true)
	if e != nil || c.Tag != "v0.8.4-preview" {
		t.Fatal(c, e)
	}
	c, e = Select(rs, "0.8.3-preview", false)
	if e != nil || c.Tag != "v0.8.3" {
		t.Fatal(c, e)
	}
	c, e = Select(rs, "0.9.0", true)
	if e != nil || c != nil {
		t.Fatal(c, e)
	}
	rs[0].Assets[0].URL = "https://evil.example/x"
	if _, e = Select(rs, "0.8.3", true); e == nil {
		t.Fatal("foreign asset")
	}
	rs[0] = release("v0.8.4-preview", true)
	rs[0].Assets = rs[0].Assets[:1]
	if _, e = Select(rs, "0.8.3", true); e == nil {
		t.Fatal("missing checksum")
	}
}
func fixture(t *testing.T, bad string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "release.zip")
	f, e := os.Create(name)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	var sums strings.Builder
	for _, n := range Required {
		if bad == "missing" && n == Required[1] {
			continue
		}
		b := []byte("fixture:" + n)
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(b), n)
		w, _ := z.Create(n)
		if bad == "corrupt" && n == Required[0] {
			b = []byte("changed")
		}
		w.Write(b)
	}
	for _, n := range []string{"../escape", "zkasnodemanager.exe"} {
		if bad == n {
			w, _ := z.Create(n)
			w.Write([]byte("bad"))
		}
	}
	if bad == "symlink" {
		h := &zip.FileHeader{Name: "link"}
		h.SetMode(os.ModeSymlink | 0777)
		w, _ := z.CreateHeader(h)
		w.Write([]byte("../outside"))
	}
	w, _ := z.Create("SHA256SUMS.txt")
	w.Write([]byte(sums.String()))
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	f.Close()
	return name
}
func TestPackage(t *testing.T) {
	for _, bad := range []string{"", "missing", "corrupt", "../escape", "zkasnodemanager.exe", "symlink"} {
		t.Run(bad, func(t *testing.T) {
			d := filepath.Join(t.TempDir(), "new")
			e := Extract(fixture(t, bad), d)
			if bad == "" {
				if e != nil {
					t.Fatal(e)
				}
			} else {
				if e == nil {
					t.Fatal("unsafe package accepted")
				}
				if _, e = os.Stat(d); !os.IsNotExist(e) {
					t.Fatal("partial retained")
				}
			}
		})
	}
}
func TestPaths(t *testing.T) {
	for _, s := range []string{"../x", "/x", "C:/x", "a\\b", "a/../b", "a/CON.txt", "x.", "x ", "a:b", "x?", "x*", "x|", "x<", "x>", "x\""} {
		if SafeName(s) {
			t.Fatal(s)
		}
	}
	if !SafeName("wallet-runtime/node.exe") {
		t.Fatal("valid path")
	}
}
func TestDownload(t *testing.T) {
	data := []byte("test release")
	for _, bad := range []bool{false, true} {
		t.Run(fmt.Sprint(bad), func(t *testing.T) {
			sum := fmt.Sprintf("%x", sha256.Sum256(data))
			if bad {
				sum = strings.Repeat("0", 64)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/sums" {
					fmt.Fprintf(w, "%s  release.zip\n", sum)
				} else {
					w.Write(data)
				}
			}))
			defer server.Close()
			dest := filepath.Join(t.TempDir(), "release.zip")
			c := Candidate{Archive: Asset{Name: "release.zip", URL: server.URL + "/zip", Size: int64(len(data))}, Checksums: Asset{URL: server.URL + "/sums"}}
			e := Download(context.Background(), server.Client(), c, dest)
			if bad {
				if e == nil {
					t.Fatal("bad hash accepted")
				}
				if _, e = os.Stat(dest); !os.IsNotExist(e) {
					t.Fatal("bad download retained")
				}
			} else if e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestReleaseFixture(t *testing.T) {
	p := os.Getenv("ZKAS_MANAGER_UPDATE_ZIP")
	if p == "" {
		t.Skip("optional packaged fixture")
	}
	if e := Extract(p, filepath.Join(t.TempDir(), "release")); e != nil {
		t.Fatal(e)
	}
}
