package managerupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Repository = "Jack-Kane2468/zkas-easy-node-and-wallet"
const MaxArchive = 160 << 20

type Version struct {
	Numbers [3]uint64
	Pre     string
}

var versionRE = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func ParseVersion(s string) (Version, error) {
	var v Version
	m := versionRE.FindStringSubmatch(s)
	if m == nil {
		return v, fmt.Errorf("invalid version")
	}
	for i := 0; i < 3; i++ {
		n, e := strconv.ParseUint(m[i+1], 10, 64)
		if e != nil {
			return v, e
		}
		v.Numbers[i] = n
	}
	v.Pre = m[4]
	for _, p := range strings.Split(v.Pre, ".") {
		if numeric(p) && len(p) > 1 && p[0] == '0' {
			return v, fmt.Errorf("invalid prerelease")
		}
	}
	return v, nil
}
func Compare(a, b Version) int {
	for i, n := range a.Numbers {
		if n < b.Numbers[i] {
			return -1
		}
		if n > b.Numbers[i] {
			return 1
		}
	}
	if a.Pre == b.Pre {
		return 0
	}
	if a.Pre == "" {
		return 1
	}
	if b.Pre == "" {
		return -1
	}
	aa, bb := strings.Split(a.Pre, "."), strings.Split(b.Pre, ".")
	for i, x := range aa {
		if i >= len(bb) {
			return 1
		}
		y := bb[i]
		if x == y {
			continue
		}
		xn, yn := numeric(x), numeric(y)
		if xn && yn {
			if len(x) < len(y) {
				return -1
			}
			if len(x) > len(y) {
				return 1
			}
		} else if xn != yn {
			if xn {
				return -1
			}
			return 1
		}
		if x < y {
			return -1
		}
		return 1
	}
	return -1
}

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}
type Release struct {
	Tag     string  `json:"tag_name"`
	Body    string  `json:"body"`
	Draft   bool    `json:"draft"`
	Preview bool    `json:"prerelease"`
	Assets  []Asset `json:"assets"`
}
type Candidate struct {
	Release
	Archive, Checksums Asset
}

func Select(rs []Release, current string, previews bool) (*Candidate, error) {
	bestV, e := ParseVersion(current)
	if e != nil {
		return nil, e
	}
	var best *Release
	for i := range rs {
		r := &rs[i]
		v, e := ParseVersion(r.Tag)
		if e != nil || r.Draft || (!previews && (r.Preview || v.Pre != "")) {
			continue
		}
		if Compare(v, bestV) > 0 {
			best = r
			bestV = v
		}
	}
	if best == nil {
		return nil, nil
	}
	c := &Candidate{Release: *best}
	expected := "ZKasNodeManager-Windows-x64-" + strings.TrimPrefix(best.Tag, "v") + ".zip"
	for _, a := range best.Assets {
		switch a.Name {
		case expected:
			if c.Archive.Name != "" {
				return nil, fmt.Errorf("duplicate archive")
			}
			c.Archive = a
		case "SHA256SUMS.txt":
			if c.Checksums.Name != "" {
				return nil, fmt.Errorf("duplicate checksums")
			}
			c.Checksums = a
		}
	}
	if c.Archive.Name == "" || c.Checksums.Name == "" {
		return nil, fmt.Errorf("Release %s needs both %s and SHA256SUMS.txt attached", best.Tag, expected)
	}
	if c.Archive.Size <= 0 || c.Archive.Size > MaxArchive || c.Checksums.Size <= 0 || c.Checksums.Size > 1<<20 {
		return nil, fmt.Errorf("invalid release asset size")
	}
	for _, a := range []Asset{c.Archive, c.Checksums} {
		u, e := url.Parse(a.URL)
		if e != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/"+Repository+"/releases/download/"+best.Tag+"/"+a.Name {
			return nil, fmt.Errorf("unexpected release asset URL")
		}
	}
	return c, nil
}
func HTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 6 {
			return fmt.Errorf("too many redirects")
		}
		switch r.URL.Host {
		case "github.com", "api.github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		default:
			return fmt.Errorf("untrusted download redirect")
		}
		if r.URL.Scheme != "https" || r.URL.User != nil {
			return fmt.Errorf("insecure redirect")
		}
		return nil
	}}
}
func request(ctx context.Context, c *http.Client, u string) (*http.Response, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", u, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "ZKas-Node-Manager-Updater")
	res, e := c.Do(req)
	if e != nil {
		return nil, e
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("GitHub returned HTTP %d; check your connection or retry later if rate-limited", res.StatusCode)
	}
	return res, nil
}
func Latest(ctx context.Context, c *http.Client, current string, previews bool) (*Candidate, error) {
	var rs []Release
	for page := 1; page <= 5; page++ {
		res, e := request(ctx, c, "https://api.github.com/repos/"+Repository+"/releases?per_page=100&page="+strconv.Itoa(page))
		if e != nil {
			return nil, e
		}
		b, e := io.ReadAll(io.LimitReader(res.Body, (5<<20)+1))
		res.Body.Close()
		if e != nil {
			return nil, e
		}
		if len(b) > 5<<20 {
			return nil, fmt.Errorf("release metadata too large")
		}
		var list []Release
		if e = json.Unmarshal(b, &list); e != nil {
			return nil, e
		}
		rs = append(rs, list...)
		if len(list) < 100 {
			break
		}
	}
	return Select(rs, current, previews)
}
