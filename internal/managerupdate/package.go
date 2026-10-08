package managerupdate

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var Required = []string{"ZKasNodeManager.exe", "wallet-runtime/node.exe", "wallet-runtime/signer.mjs", "wallet-runtime/official-signer.mjs", "wallet-runtime/official-signer.wasm", "wallet-runtime/view-tools.mjs", "wallet-runtime/view-tools.wasm", "wallet-runtime/kaspa.cjs", "wallet-runtime/kaspa_bg.wasm", "wallet-runtime/kaspa-wallet.cjs", "wallet-runtime/kaspa-addresses.cjs"}

func SafeName(s string) bool {
	if s == "" || strings.ContainsAny(s, "\\:\x00<>\"|?*") || strings.HasPrefix(s, "/") || path.Clean(s) != s {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "." || p == ".." || strings.HasSuffix(p, ".") || strings.HasSuffix(p, " ") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(p, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}
func ParseSums(data []byte) (map[string]string, error) {
	out := map[string]string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < 67 || line[64] != ' ' || (line[65] != ' ' && line[65] != '*') {
			return nil, fmt.Errorf("invalid checksum line")
		}
		name := line[66:]
		sum := strings.ToLower(line[:64])
		b, e := hex.DecodeString(sum)
		if e != nil || len(b) != 32 || !SafeName(name) || seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("invalid or duplicate checksum entry")
		}
		out[name] = sum
		seen[strings.ToLower(name)] = true
	}
	return out, nil
}
func Download(ctx context.Context, client *http.Client, c Candidate, dest string) error {
	res, e := request(ctx, client, c.Checksums.URL)
	if e != nil {
		return e
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	res.Body.Close()
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return fmt.Errorf("checksum file too large")
	}
	sums, e := ParseSums(b)
	if e != nil {
		return e
	}
	want, ok := sums[c.Archive.Name]
	if !ok {
		return fmt.Errorf("checksum does not name Windows ZIP")
	}
	res, e = request(ctx, client, c.Archive.URL)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	f, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, MaxArchive+1))
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil || n > MaxArchive || n != c.Archive.Size || hex.EncodeToString(h.Sum(nil)) != want {
		os.Remove(dest)
		return fmt.Errorf("download incomplete or checksum mismatch; current manager unchanged")
	}
	return nil
}
func Extract(archive, dest string) error {
	z, e := zip.OpenReader(archive)
	if e != nil {
		return e
	}
	defer z.Close()
	if len(z.File) > 2000 {
		return fmt.Errorf("too many ZIP entries")
	}
	files := map[string]*zip.File{}
	seen := map[string]bool{}
	var size uint64
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !SafeName(f.Name) || !f.Mode().IsRegular() || seen[strings.ToLower(f.Name)] {
			return fmt.Errorf("unsafe or duplicate ZIP entry")
		}
		seen[strings.ToLower(f.Name)] = true
		if f.UncompressedSize64 > 160<<20 {
			return fmt.Errorf("ZIP entry too large")
		}
		size += f.UncompressedSize64
		if size > 512<<20 {
			return fmt.Errorf("package too large")
		}
		files[f.Name] = f
	}
	mf, ok := files["SHA256SUMS.txt"]
	if !ok {
		return fmt.Errorf("missing internal checksums")
	}
	r, e := mf.Open()
	if e != nil {
		return e
	}
	b, e := io.ReadAll(io.LimitReader(r, (1<<20)+1))
	r.Close()
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return fmt.Errorf("manifest too large")
	}
	sums, e := ParseSums(b)
	if e != nil {
		return e
	}
	for _, name := range Required {
		if _, ok := files[name]; !ok {
			return fmt.Errorf("package missing %s", name)
		}
	}
	for name := range sums {
		if _, ok := files[name]; !ok {
			return fmt.Errorf("checksum references missing file")
		}
	}
	if e = os.Mkdir(dest, 0700); e != nil {
		return e
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(dest)
		}
	}()
	for name, f := range files {
		if name == "SHA256SUMS.txt" {
			continue
		}
		want, ok := sums[name]
		if !ok {
			return fmt.Errorf("unlisted package file")
		}
		out := filepath.Join(dest, filepath.FromSlash(name))
		if e = os.MkdirAll(filepath.Dir(out), 0700); e != nil {
			return e
		}
		r, e := f.Open()
		if e != nil {
			return e
		}
		w, e := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			r.Close()
			return e
		}
		h := sha256.New()
		_, e = io.Copy(io.MultiWriter(w, h), r)
		r.Close()
		ce := w.Close()
		if e == nil {
			e = ce
		}
		if e != nil || hex.EncodeToString(h.Sum(nil)) != want {
			return fmt.Errorf("checksum mismatch: %s", name)
		}
	}
	if e = os.WriteFile(filepath.Join(dest, "SHA256SUMS.txt"), b, 0600); e != nil {
		return e
	}
	success = true
	return nil
}
