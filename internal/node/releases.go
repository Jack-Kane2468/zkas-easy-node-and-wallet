package node

import (
	"context"
	"fmt"
	"zkas-node-manager/internal/releasesite"
)

func siteRelease(p releasesite.Package) Release {
	return Release{Tag: p.Version, Body: "Official Windows download and SHA-256: https://zkas.info/downloads/node/\nChangelog: https://zkas.info/downloads/node/" + p.Version[len("zkas-v"):] + "/CHANGELOG.md", Assets: []Asset{{Name: p.Name, URL: p.URL, Digest: "sha256:" + p.SHA, Size: p.Size}}}
}
func Latest(ctx context.Context) (Release, error) {
	p, e := releasesite.Lookup(ctx, "node", "")
	if e == nil {
		r := siteRelease(p)
		_, e = r.WindowsAsset()
		return r, e
	}
	r, fallback := latestGitHub(ctx)
	if fallback != nil {
		return Release{}, fmt.Errorf("Official downloads: %v; GitHub fallback: %w", e, fallback)
	}
	return r, nil
}
func ReleaseAt(ctx context.Context, tag string) (Release, error) {
	if !tagPattern.MatchString(tag) {
		return Release{}, fmt.Errorf("Invalid installed release tag")
	}
	p, e := releasesite.Lookup(ctx, "node", tag)
	if e == nil {
		r := siteRelease(p)
		_, e = r.WindowsAsset()
		return r, e
	}
	r, fallback := releaseAtGitHub(ctx, tag)
	if fallback != nil {
		return Release{}, fmt.Errorf("Official downloads: %v; GitHub fallback: %w", e, fallback)
	}
	return r, nil
}
func ReleaseNewer(candidate, installed string) (bool, error) {
	return releasesite.Newer(candidate, installed)
}
