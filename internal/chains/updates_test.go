package chains

import (
	"context"
	"strings"
	"testing"
	"zkas-node-manager/internal/node"
)

func TestKaspaNodeRejectsWrongReleaseBeforeDownload(t *testing.T) {
	for _, r := range []BridgeRelease{{Dual: true}, {Release: node.Release{Tag: "v2.1.0"}}} {
		if _, e := InstallKaspaNodeRelease(context.Background(), t.TempDir(), r, func(string) { t.Fatal("download started for invalid release") }); e == nil {
			t.Fatal("invalid node release accepted")
		}
	}
}

func TestBridgeReleaseValidation(t *testing.T) {
	for _, dual := range []bool{false, true} {
		name := "rusty-kaspa-v2.1.0-win64.zip"
		if dual {
			name = "solo-dual-mode-windows-x64.zip"
		}
		valid := func() node.Release {
			return node.Release{Tag: "v2.1.0", Assets: []node.Asset{{Name: name, Size: 1234, Digest: "sha256:" + strings.Repeat("a", 64), URL: "https://github.com/" + BridgeRepository(dual) + "/releases/download/v2.1.0/" + name}}}
		}
		if _, e := ValidateBridgeRelease(valid(), dual); e != nil {
			t.Fatal(e)
		}
		changes := []func(*node.Release){
			func(r *node.Release) { r.Prerelease = true },
			func(r *node.Release) { r.Draft = true },
			func(r *node.Release) { r.Tag = "../../outside" },
			func(r *node.Release) {
				r.Assets[0].URL = "https://github.com/other/project/releases/download/v2.1.0/" + name
			},
			func(r *node.Release) { r.Assets[0].Digest = "" },
			func(r *node.Release) { r.Assets[0].Size = 101 << 20 },
			func(r *node.Release) { r.Assets = append(r.Assets, r.Assets[0]) },
			func(r *node.Release) { r.Assets = nil },
		}
		for i, change := range changes {
			r := valid()
			change(&r)
			if _, e := ValidateBridgeRelease(r, dual); e == nil {
				t.Fatalf("accepted case %d (dual=%v)", i, dual)
			}
		}
	}
}
func TestBridgeUpdateVersionOrder(t *testing.T) {
	for _, v := range []struct {
		candidate, installed string
		want                 bool
	}{{"v1.0.12", "v1.0.11", true}, {"v1.0.11", "v1.0.12", false}, {"v1.0.12", "v1.0.12", false}, {"v1.0.12", "", true}, {"v1.10.0", "v1.9.0", true}} {
		got, e := BridgeNewer(v.candidate, v.installed)
		if e != nil || got != v.want {
			t.Fatal(v, got, e)
		}
	}
	if _, e := BridgeNewer("v1.0.12", "unknown"); e == nil {
		t.Fatal("unrecognized installed version")
	}
}
