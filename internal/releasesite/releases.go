// Package releasesite reads the project's official download indexes. Radicle
// carries source tags; binary discovery does not depend on Radicle releases.
package releasesite

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"zkas-node-manager/internal/managerupdate"
)

const Base = "https://zkas.info/downloads/"

type Package struct {
	Version, Name, URL, SHA string
	Size                    int64
}

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var hashPattern = regexp.MustCompile(`^sha256\s+([a-fA-F0-9]{64})$`)

func PackageURL(kind, tag, name string) string {
	version := strings.TrimPrefix(tag, "zkas-v")
	if kind == "bridge" {
		version = strings.TrimPrefix(tag, "v")
	}
	if !versionPattern.MatchString(version) {
		return ""
	}
	if kind == "node" && name == "zkas-zkas-v"+version+"-win64.zip" {
		return Base + "node/" + version + "/" + name
	}
	if kind == "bridge" && name == "solo-dual-mode-windows-x64.zip" {
		return Base + "bridge/v" + version + "/" + name
	}
	return ""
}
func text(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var s strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s.WriteString(text(c))
	}
	return s.String()
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// Pair each link only with the following checksum in the same table cell.
// Never borrow a hash from another platform, link or row.
func Parse(r io.Reader, kind string) ([]Package, error) {
	if kind != "node" && kind != "bridge" {
		return nil, fmt.Errorf("Unknown download category")
	}
	root, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(Base + kind + "/")
	var packages []Package
	seen := map[string]bool{}
	var parseErr error
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if parseErr != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "td" {
			var pending *Package
			var cell func(*html.Node)
			cell = func(c *html.Node) {
				if parseErr != nil {
					return
				}
				if c.Type == html.ElementNode && c.Data == "a" {
					if pending != nil {
						parseErr = fmt.Errorf("Windows download is missing its checksum")
						return
					}
					href := attr(c, "href")
					u, e := url.Parse(href)
					if e != nil {
						parseErr = e
						return
					}
					u = base.ResolveReference(u)
					name := u.Path[strings.LastIndex(u.Path, "/")+1:]
					windows := strings.HasSuffix(name, "-win64.zip")
					if kind == "bridge" {
						windows = name == "solo-dual-mode-windows-x64.zip"
					}
					if windows {
						parts := strings.Split(u.Path, "/")
						if len(parts) != 5 {
							parseErr = fmt.Errorf("Unexpected Windows download path")
							return
						}
						version := strings.TrimPrefix(parts[3], "v")
						tag := "zkas-v" + version
						if kind == "bridge" {
							tag = "v" + version
						}
						expected := PackageURL(kind, tag, name)
						if expected == "" || u.String() != expected {
							parseErr = fmt.Errorf("Unexpected Windows download location")
							return
						}
						pending = &Package{Version: tag, Name: name, URL: expected}
					}
				}
				if c.Type == html.ElementNode && c.Data == "code" && pending != nil {
					m := hashPattern.FindStringSubmatch(strings.TrimSpace(text(c)))
					if m == nil {
						parseErr = fmt.Errorf("Invalid Windows package checksum")
						return
					}
					if seen[pending.Version] {
						parseErr = fmt.Errorf("Duplicate Windows release %s", pending.Version)
						return
					}
					pending.SHA = strings.ToLower(m[1])
					packages = append(packages, *pending)
					seen[pending.Version] = true
					pending = nil
				}
				for ch := c.FirstChild; ch != nil; ch = ch.NextSibling {
					cell(ch)
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				cell(c)
			}
			if pending != nil {
				parseErr = fmt.Errorf("Windows download is missing its checksum")
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if parseErr != nil {
		return nil, parseErr
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("No verified Windows release listed at %s", base)
	}
	return packages, nil
}
func Newer(candidate, installed string) (bool, error) {
	a, e := managerupdate.ParseVersion(strings.TrimPrefix(candidate, "zkas-"))
	if e != nil {
		return false, e
	}
	if installed == "" {
		return true, nil
	}
	b, e := managerupdate.ParseVersion(strings.TrimPrefix(installed, "zkas-"))
	if e != nil {
		return false, e
	}
	return managerupdate.Compare(a, b) > 0, nil
}
func Lookup(ctx context.Context, kind, tag string) (Package, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return lookup(ctx, client, kind, tag)
}
func lookup(ctx context.Context, client *http.Client, kind, tag string) (Package, error) {
	var zero Package
	if kind != "node" && kind != "bridge" {
		return zero, fmt.Errorf("Unknown download category")
	}
	req, e := http.NewRequestWithContext(ctx, "GET", Base+kind+"/", nil)
	if e != nil {
		return zero, e
	}
	res, e := client.Do(req)
	if e != nil {
		return zero, e
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("Official download index returned %s", res.Status)
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if e != nil {
		return zero, e
	}
	if len(data) > 2<<20 {
		return zero, fmt.Errorf("Download index exceeds size limit")
	}
	packages, e := Parse(strings.NewReader(string(data)), kind)
	if e != nil {
		return zero, e
	}
	var selected Package
	for _, p := range packages {
		if tag != "" {
			if p.Version == tag {
				selected = p
				break
			}
			continue
		}
		newer, e := Newer(p.Version, selected.Version)
		if e != nil {
			return zero, e
		}
		if newer {
			selected = p
		}
	}
	if selected.URL == "" {
		return zero, fmt.Errorf("Windows release %s is not listed at %s%s/", tag, Base, kind)
	}
	req, e = http.NewRequestWithContext(ctx, "HEAD", selected.URL, nil)
	if e != nil {
		return zero, e
	}
	res, e = client.Do(req)
	if e != nil {
		return zero, e
	}
	defer res.Body.Close()
	limit := int64(512 << 20)
	if kind == "bridge" {
		limit = 100 << 20
	}
	if res.StatusCode != http.StatusOK || res.ContentLength <= 0 || res.ContentLength > limit {
		return zero, fmt.Errorf("Windows package has an invalid response or size")
	}
	selected.Size = res.ContentLength
	return selected, nil
}
