package node

import (
	"os"
	"path/filepath"
	"testing"
	"zkas-node-manager/internal/releasesite"
)

func TestOfficialNodeRelease(t *testing.T) {
	p := releasesite.Package{Version: "zkas-v1.0.13", Name: "zkas-zkas-v1.0.13-win64.zip", SHA: "bb4dea9613d90e0ed2d44d2c28b852c6373c2d6ac80ef51d47b6c1ac17f63948", Size: 78561470}
	p.URL = releasesite.PackageURL("node", p.Version, p.Name)
	r := siteRelease(p)
	if _, e := r.WindowsAsset(); e != nil {
		t.Fatal(e)
	}
	if !r.Published.IsZero() {
		t.Fatal("invented publication date")
	}
	r.Assets[0].URL += "?foreign=1"
	if _, e := r.WindowsAsset(); e == nil {
		t.Fatal("accepted modified download location")
	}
}

func TestBinLayoutAndDuplicateRejection(t *testing.T) {
	for _, tc := range []struct {
		entries map[string][]byte
		fail    bool
	}{
		{map[string][]byte{"bin/kaspad.exe": fakeEXE()}, false},
		{map[string][]byte{"bin/kaspad.exe": fakeEXE(), "kaspad.exe": fakeEXE()}, true},
		{map[string][]byte{"other/kaspad.exe": fakeEXE()}, true},
		{map[string][]byte{"bin/../kaspad.exe": fakeEXE()}, true},
	} {
		root := t.TempDir()
		p := filepath.Join(root, "package.zip")
		if e := os.WriteFile(p, archive(t, tc.entries), 0600); e != nil {
			t.Fatal(e)
		}
		e := ExtractNode(p, filepath.Join(root, "node.exe"))
		if (e != nil) != tc.fail {
			t.Fatalf("%v: %v", tc.entries, e)
		}
	}
}
