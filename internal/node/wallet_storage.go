package node

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WalletStorageReport is read-only: filenames are not evidence that recovery
// copies are no longer needed. In particular, shared stores must stay intact.
func WalletStorageReport(root string) (string, error) {
	dir := filepath.Join(root, "wallets")
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return "No wallet data directory exists yet.", nil
		}
		return "", err
	}
	var total, active, retired, other int64
	var count int
	var copies []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		total += info.Size()
		count++
		name := d.Name()
		rel, e := filepath.Rel(dir, path)
		if e != nil {
			return e
		}
		switch {
		case strings.HasSuffix(name, ".scan"):
			active += info.Size()
		case isRetiredCheckpoint(name):
			retired += info.Size()
			if len(copies) < 1000 {
				copies = append(copies, fmt.Sprintf("%s  |  %.2f MiB  |  %s", rel, float64(info.Size())/(1<<20), info.ModTime().Format("2006-01-02 15:04")))
			}
		default:
			other += info.Size()
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(copies)
	text := fmt.Sprintf("Wallet data: %s\r\nFiles: %d\r\nTotal: %.2f GiB\r\nActive .scan files: %.2f GiB\r\nRetired checkpoint copies: %.2f GiB\r\nShared stores and other files: %.2f GiB\r\n\r\n", dir, count, float64(total)/(1<<30), float64(active)/(1<<30), float64(retired)/(1<<30), float64(other)/(1<<30))
	text += "This is a size inventory, not a safe-to-delete verdict. Stop ZKas services and back up the complete wallet directory before manual cleanup. Retired copies can be needed for recovery; retain them while investigating missing funds or scan problems. Never delete active .scan files, wallet registrations, __nullifiers__.nfs, or shared chain-tree data to save space.\r\n\r\nRetired checkpoint copies (up to 1,000):\r\n"
	if len(copies) == 0 {
		text += "None found."
	} else {
		text += strings.Join(copies, "\r\n")
	}
	return text, nil
}
func isRetiredCheckpoint(name string) bool {
	_, suffix, ok := strings.Cut(name, ".scan.")
	if !ok {
		return false
	}
	return suffix == "bak" || strings.HasPrefix(suffix, "bak.") || strings.HasPrefix(suffix, "stale-") || strings.HasPrefix(suffix, "divergent-")
}
