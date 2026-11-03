package chains

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"zkas-node-manager/internal/managerupdate"
	"zkas-node-manager/internal/node"
	"zkas-node-manager/internal/releasesite"
)

type BridgeRelease struct {
	Release node.Release
	Asset   node.Asset
	Dual    bool
}

func BridgeRepository(dual bool) string {
	if dual {
		return "firecash/solo-dual-mode"
	}
	return "kaspanet/rusty-kaspa"
}

var stableBridgeTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
var bridgeDigest = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)

func ValidateBridgeRelease(r node.Release, dual bool) (BridgeRelease, error) {
	out := BridgeRelease{Release: r, Dual: dual}
	if r.Draft || r.Prerelease || !stableBridgeTag.MatchString(r.Tag) {
		return out, fmt.Errorf("Expected a stable bridge release")
	}
	name := "rusty-kaspa-" + r.Tag + "-win64.zip"
	if dual {
		name = "solo-dual-mode-windows-x64.zip"
	}
	count := 0
	for _, a := range r.Assets {
		if a.Name == name {
			out.Asset = a
			count++
		}
	}
	a := out.Asset
	locationOK := a.URL == "https://github.com/"+BridgeRepository(dual)+"/releases/download/"+r.Tag+"/"+name
	if dual && a.URL != "" && a.URL == releasesite.PackageURL("bridge", r.Tag, name) {
		locationOK = true
	}
	if count != 1 || a.Size <= 0 || a.Size > 100<<20 || !bridgeDigest.MatchString(a.Digest) || !locationOK {
		return out, fmt.Errorf("Release is missing one verified Windows x64 package; no changes made")
	}
	return out, nil
}
func latestBridgeGitHub(ctx context.Context, dual bool) (BridgeRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/"+BridgeRepository(dual)+"/releases/latest", nil)
	if e != nil {
		return BridgeRelease{}, e
	}
	req.Header.Set("User-Agent", "ZKas-Node-Manager/"+node.ManagerVersion)
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, e := client.Do(req)
	if e != nil {
		return BridgeRelease{}, e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return BridgeRelease{}, fmt.Errorf("GitHub returned %s; check your connection or retry after the API rate limit resets", res.Status)
	}
	var r node.Release
	if e = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&r); e != nil {
		return BridgeRelease{}, e
	}
	return ValidateBridgeRelease(r, dual)
}
func BridgeNewer(candidate, installed string) (bool, error) {
	if installed == "" {
		return true, nil
	}
	a, e := managerupdate.ParseVersion(candidate)
	if e != nil {
		return false, e
	}
	b, e := managerupdate.ParseVersion(installed)
	if e != nil {
		return false, e
	}
	return managerupdate.Compare(a, b) > 0, nil
}
func InstallBridgeRelease(ctx context.Context, root string, r BridgeRelease, progress func(string)) (Binary, error) {
	checked, e := ValidateBridgeRelease(r.Release, r.Dual)
	if e != nil {
		return Binary{}, e
	}
	a := checked.Asset
	hash := strings.ToLower(strings.TrimPrefix(a.Digest, "sha256:"))
	kind := "kaspa-bridge-"
	if r.Dual {
		kind = "merged-"
	}
	// Separate download generations avoid overwriting any running executable.
	folder := kind + r.Release.Tag + "-" + hash[:12]
	binaries, e := installArchive(ctx, root, a.URL, hash, r.Release.Tag, folder, []string{"stratum-bridge.exe"}, progress)
	if e != nil {
		return Binary{}, e
	}
	b := binaries["stratum-bridge.exe"]
	b.ArchiveSHA = hash
	return b, nil
}

// Kaspa node and bridge share an upstream archive, but are installed and updated independently.
func LatestKaspaNode(ctx context.Context) (BridgeRelease, error) {
	return LatestBridge(ctx, false)
}

func InstallKaspaNodeRelease(ctx context.Context, root string, r BridgeRelease, progress func(string)) (Binary, error) {
	if r.Dual {
		return Binary{}, fmt.Errorf("A merged bridge release cannot update the Kaspa node")
	}
	checked, e := ValidateBridgeRelease(r.Release, false)
	if e != nil {
		return Binary{}, e
	}
	hash := strings.ToLower(strings.TrimPrefix(checked.Asset.Digest, "sha256:"))
	folder := "kaspa-node-" + checked.Release.Tag + "-" + hash[:12]
	b, e := installArchive(ctx, root, checked.Asset.URL, hash, checked.Release.Tag, folder, []string{"kaspad.exe"}, progress)
	if e != nil {
		return Binary{}, e
	}
	return b["kaspad.exe"], nil
}

func LatestBridge(ctx context.Context, dual bool) (BridgeRelease, error) {
	if !dual {
		return latestBridgeGitHub(ctx, false)
	}
	p, e := releasesite.Lookup(ctx, "bridge", "")
	if e == nil {
		return ValidateBridgeRelease(node.Release{Tag: p.Version, Assets: []node.Asset{{Name: p.Name, URL: p.URL, Digest: "sha256:" + p.SHA, Size: p.Size}}}, true)
	}
	r, fallback := latestBridgeGitHub(ctx, true)
	if fallback != nil {
		return BridgeRelease{}, fmt.Errorf("Official downloads: %v; GitHub fallback: %w", e, fallback)
	}
	return r, nil
}

// Rebuilt packages can retain a version tag. Record archive identity separately
// from the extracted executable hash, and offer verification for legacy installs.
func BridgeNeedsUpdate(r BridgeRelease, installed Binary) (bool, error) {
	newer, e := BridgeNewer(r.Release.Tag, installed.Version)
	if e != nil || newer {
		return newer, e
	}
	if r.Dual && r.Release.Tag == installed.Version {
		return !strings.EqualFold(installed.ArchiveSHA, strings.TrimPrefix(r.Asset.Digest, "sha256:")), nil
	}
	return false, nil
}
