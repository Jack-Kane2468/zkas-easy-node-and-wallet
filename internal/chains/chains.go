package chains

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"zkas-node-manager/internal/node"
)

const KaspaVersion = "v2.1.0"
const DualVersion = "v1.0.11"
const KaspaSHA = "fb25743a4b432d376c4ca49d8799769dbc6d5b070f0b502350ce32cfdb38ec0f"
const DualSHA = "f3ab6de48ae0da79c1f22073444e7d5a3a4caf047dbdf9fb0f68b772d2416610"
const KaspaURL = "https://github.com/kaspanet/rusty-kaspa/releases/download/" + KaspaVersion + "/rusty-kaspa-" + KaspaVersion + "-win64.zip"
const DualURL = "https://github.com/firecash/solo-dual-mode/releases/download/" + DualVersion + "/solo-dual-mode-windows-x64.zip"

type Binary struct {
	Path    string `json:"path"`
	SHA     string `json:"sha256"`
	Version string `json:"version"`
}
type Config struct {
	NodeMode  string `json:"nodeMode"`
	DataPath  string `json:"dataPath,omitempty"`
	GRPC      int    `json:"grpc"`
	JSON      int    `json:"json"`
	Borsh     int    `json:"borsh"`
	P2P       int    `json:"p2p"`
	PublicP2P bool   `json:"publicP2P"`
	AutoStart bool   `json:"autoStart"`

	RAMScale     float64 `json:"ramScale"`
	Wallet       Binary  `json:"wallet"`
	Kaspa        Binary  `json:"kaspa"`
	KaspaBridge  Binary  `json:"kaspaBridge"`
	DualBridge   Binary  `json:"dualBridge"`
	Host         string  `json:"host"`
	Mode         string  `json:"mode"`
	KaspaAddress string  `json:"kaspaAddress"`
	ZKasAddress  string  `json:"zkasAddress"`
	LAN          bool    `json:"lan"`
}

func Read(root string) (Config, error) {
	c := (Config{Mode: "merged", RAMScale: 0.3}).Defaults()
	b, e := os.ReadFile(filepath.Join(root, "chains.json"))
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	c = c.Defaults()
	if c.RAMScale == 0 {
		c.RAMScale = 0.3
	}
	if e == nil && (c.RAMScale < 0.1 || c.RAMScale > 2) {
		e = errors.New("Kaspa memory scale must be between 0.1 and 2")
	}
	return c, e
}
func Save(root string, c Config) error { return node.AtomicJSON(filepath.Join(root, "chains.json"), c) }
func (b Binary) Check() error {
	if b.Path == "" {
		return errors.New("Component is not installed. Click Install first")
	}
	h, e := node.FileHash(b.Path)
	if e != nil {
		return e
	}
	if h != b.SHA || len(h) != 64 {
		return errors.New("Component checksum changed; reinstall before starting")
	}
	return nil
}
func (c Config) Defaults() Config {
	if c.NodeMode == "" {
		c.NodeMode = "wallet"
	}
	if c.GRPC == 0 {
		c.GRPC = 16110
	}
	if c.JSON == 0 {
		c.JSON = 18110
	}
	if c.Borsh == 0 {
		c.Borsh = 17110
	}
	if c.P2P == 0 {
		c.P2P = 16111
	}
	return c
}
func (c Config) ValidateNode() error {
	c = c.Defaults()
	if c.NodeMode != "wallet" && c.NodeMode != "basic" && c.NodeMode != "archive" {
		return errors.New("Choose a Kaspa node purpose")
	}
	seen := map[int]bool{}
	for _, p := range []int{c.GRPC, c.JSON, c.Borsh, c.P2P} {
		if p < 1024 || p > 65535 || seen[p] || p == 5555 || p == 18888 || p == 18889 || p == 18081 || p == 18115 || p == 18502 || p == 18503 {
			return errors.New("Choose distinct Kaspa ports (1024–65535) outside mining/Tor ports")
		}
		seen[p] = true
	}
	return nil
}
func DataDir(root string) string {
	c, e := Read(root)
	if e == nil && c.DataPath != "" {
		return c.DataPath
	}
	return filepath.Join(root, "kaspa-data")
}
func KaspaArgs(root string) []string {
	c, _ := Read(root)
	c = c.Defaults()
	scale := c.RAMScale
	if scale == 0 {
		scale = 0.3
	}
	listen := "127.0.0.1"
	if c.PublicP2P {
		listen = "0.0.0.0"
	}
	args := []string{"--appdir=" + DataDir(root), "--logdir=" + filepath.Join(root, "logs", "kaspa"), fmt.Sprintf("--rpclisten=127.0.0.1:%d", c.GRPC), fmt.Sprintf("--rpclisten-json=127.0.0.1:%d", c.JSON), fmt.Sprintf("--rpclisten-borsh=127.0.0.1:%d", c.Borsh), fmt.Sprintf("--listen=%s:%d", listen, c.P2P), "--disable-upnp", fmt.Sprintf("--ram-scale=%.1f", scale), "--yes"}
	if c.NodeMode != "basic" {
		args = append(args, "--utxoindex")
	}
	if c.NodeMode == "archive" {
		args = append(args, "--archival")
	}
	return args
}

var addressBody = regexp.MustCompile(`^[a-z0-9]{40,200}$`)

func AddressShape(a, p string) bool {
	return strings.HasPrefix(a, p+":") && addressBody.MatchString(strings.TrimPrefix(a, p+":"))
}
func (c Config) ValidateMining() error {
	if c.Mode != "merged" && c.Mode != "kaspa" && c.Mode != "zkas" {
		return errors.New("Choose merged mining, Kaspa only or ZKas only")
	}
	if c.Mode != "zkas" && !AddressShape(c.KaspaAddress, "kaspa") {
		return errors.New("Enter your full mainnet kaspa: receiving address, never a key or seed")
	}
	if c.Mode != "kaspa" && !AddressShape(c.ZKasAddress, "zkas") {
		return errors.New("Enter your full mainnet zkas: receiving address")
	}
	return nil
}
func (c Config) Bridge() Binary {
	if c.Mode == "kaspa" {
		return c.KaspaBridge
	}
	return c.DualBridge
}
func (c Config) BridgeArgs(root string) []string {
	return []string{"--node-mode=external", "--config=" + filepath.Join(root, "components", "bridge.yaml")}
}

// JSON is valid YAML; encoding prevents configuration injection through inputs.
func (c Config) BridgeConfig(grpc int) ([]byte, error) {
	c = c.Defaults()
	if e := c.ValidateMining(); e != nil {
		return nil, e
	}
	bind := "127.0.0.1:5555"
	if c.LAN {
		bind = "0.0.0.0:5555"
	}
	rpc := fmt.Sprintf("127.0.0.1:%d", c.GRPC)
	if c.Mode != "kaspa" {
		rpc = fmt.Sprintf("127.0.0.1:%d", grpc)
	}
	v := map[string]any{"kaspad_address": rpc, "print_stats": true, "log_to_file": false, "web_dashboard_port": "127.0.0.1:18889", "health_check_port": "127.0.0.1:18081", "var_diff": true, "shares_per_min": 20, "instances": []any{map[string]any{"stratum_port": bind, "min_share_diff": 8192, "var_diff": true, "shares_per_min": 20, "log_to_file": false, "prom_port": "127.0.0.1:18115"}}}
	if c.Mode == "merged" {
		v["merged_kaspa_address"] = fmt.Sprintf("127.0.0.1:%d", c.GRPC)
		v["merged_kaspa_pay_address"] = c.KaspaAddress
	}
	return json.MarshalIndent(v, "", "  ")
}

// Immutable reviewed release pins. No downloaded scripts execute.
func Install(ctx context.Context, root string, dual bool, progress func(string), namesOverride ...string) (map[string]Binary, error) {
	url, hash, version, folder := KaspaURL, KaspaSHA, KaspaVersion, "kaspa-"+KaspaVersion
	names := []string{"kaspad.exe", "stratum-bridge.exe", "kaspa-wallet.exe"}
	if dual {
		url, hash, version, folder = DualURL, DualSHA, DualVersion, "merged-"+DualVersion
		names = []string{"stratum-bridge.exe"}
	}
	if len(namesOverride) > 0 {
		names = namesOverride
	}
	progress("Downloading " + folder + "…")
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", url, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "ZKas-Node-Manager/"+node.ManagerVersion)
	client := &http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error {
		h := r.URL.Hostname()
		if len(via) > 8 || r.URL.Scheme != "https" || (h != "github.com" && h != "release-assets.githubusercontent.com" && h != "objects.githubusercontent.com") {
			return errors.New("Unexpected download redirect")
		}
		return nil
	}}
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Download returned %s", resp.Status)
	}
	dir := filepath.Join(root, "components")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	f, e := os.CreateTemp(dir, "download-*.zip")
	if e != nil {
		return nil, e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, (100<<20)+1))
	if e != nil {
		return nil, e
	}
	if n > 100<<20 || hex.EncodeToString(h.Sum(nil)) != hash {
		return nil, errors.New("Download checksum or size check failed; nothing installed")
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	progress("Installing verified " + folder + "…")
	return Extract(f.Name(), filepath.Join(dir, folder), version, names)
}
func Extract(archive, dest, version string, names []string) (map[string]Binary, error) {
	z, e := zip.OpenReader(archive)
	if e != nil {
		return nil, e
	}
	defer z.Close()
	if len(z.File) > 100 {
		return nil, errors.New("Unexpected archive size")
	}
	entries := map[string]*zip.File{}
	for _, name := range names {
		for _, f := range z.File {
			if f.Name == name {
				if entries[name] != nil {
					return nil, errors.New("Duplicate executable")
				}
				entries[name] = f
			}
		}
		f := entries[name]
		if f == nil || f.UncompressedSize64 > 200<<20 || !f.Mode().IsRegular() {
			return nil, fmt.Errorf("Missing or invalid %s", name)
		}
	}
	if e = os.MkdirAll(dest, 0700); e != nil {
		return nil, e
	}
	out := map[string]Binary{}
	for _, name := range names {
		r, e := entries[name].Open()
		if e != nil {
			return nil, e
		}
		b, e := io.ReadAll(io.LimitReader(r, (200<<20)+1))
		r.Close()
		if e != nil {
			return nil, e
		}
		if len(b) > 200<<20 || len(b) < 2 || string(b[:2]) != "MZ" {
			return nil, errors.New("Invalid Windows executable")
		}
		h := sha256.Sum256(b)
		digest := hex.EncodeToString(h[:])
		path := filepath.Join(dest, name)
		if old, _ := node.FileHash(path); old != digest {
			if e = os.WriteFile(path+".new", b, 0700); e != nil {
				return nil, e
			}
			if e = os.Rename(path+".new", path); e != nil {
				return nil, e
			}
		}
		out[name] = Binary{path, digest, version}
	}
	return out, nil
}
