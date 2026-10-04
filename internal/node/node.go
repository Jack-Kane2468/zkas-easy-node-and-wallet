// Package node contains the portable, testable installation and configuration logic.
package node

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Repository = "https://github.com/firecash/zkas-rusty"
const LatestAPI = "https://api.github.com/repos/firecash/zkas-rusty/releases/latest"
const ManagerVersion = "0.8.5-preview"

type Config struct {
	PublicP2P bool `json:"publicP2P,omitempty"`

	EnableWallet     bool      `json:"enableWalletBackend"`
	WalletPort       int       `json:"walletPort"`
	ResidentWallets  int       `json:"residentWallets"`
	WalletExecutable string    `json:"walletExecutable"`
	WalletSHA256     string    `json:"walletExecutableSha256"`
	DataDir          string    `json:"dataDir"`
	GRPC             int       `json:"grpcPort"`
	WebSocket        int       `json:"webSocketPort"`
	EnableWebSocket  bool      `json:"enableWebSocket"`
	Mode             string    `json:"mode"`
	AutoStart        bool      `json:"startAtSignIn"`
	Version          string    `json:"nodeVersion"`
	Executable       string    `json:"executable"`
	SHA256           string    `json:"executableSha256"`
	Published        time.Time `json:"published"`
}

func Defaults(root string) Config {
	return Config{DataDir: filepath.Join(root, "data"), GRPC: 16810, WebSocket: 18810, EnableWebSocket: true, Mode: "wallet", EnableWallet: true, WalletPort: 8501}
}
func (c Config) Validate() error {
	if !filepath.IsAbs(c.DataDir) || strings.ContainsAny(c.DataDir, "\r\n\x00") {
		return errors.New("Choose an absolute local data folder")
	}
	if c.GRPC < 1024 || c.GRPC > 65535 || (c.EnableWebSocket && (c.WebSocket < 1024 || c.WebSocket > 65535 || c.WebSocket == c.GRPC)) {
		return errors.New("Use distinct RPC ports between 1024 and 65535")
	}
	if c.GRPC == 16811 || c.EnableWebSocket && c.WebSocket == 16811 {
		return errors.New("Port 16811 is reserved for this node's peer listener")
	}
	if c.Mode != "wallet" && c.Mode != "pruned" && c.Mode != "archive" {
		return errors.New("Unknown storage mode")
	}
	if c.EnableWallet {
		if c.Mode == "pruned" {
			return errors.New("The wallet backend requires shielded history: choose Wallet / application backend or Archive")
		}
		if c.WalletPort < 1024 || c.WalletPort > 65535 || c.WalletPort == c.GRPC || c.WalletPort == 16811 || c.EnableWebSocket && c.WalletPort == c.WebSocket {
			return errors.New("Choose a distinct wallet REST port between 1024 and 65535")
		}
		if c.ResidentWallets < 0 || c.ResidentWallets > 10000 {
			return errors.New("Resident wallet limit must be 0 (automatic) through 10000")
		}
	}
	return nil
}
func (c Config) WalletArgs(root string) []string {
	a := []string{"--network=mainnet", "--rpc-server=127.0.0.1:" + strconv.Itoa(c.GRPC), "--listen=127.0.0.1:" + strconv.Itoa(c.WalletPort), "--wallet-dir=" + filepath.Join(root, "wallets"), "--no-custodial", "--active-sync-window=1800"}
	if c.ResidentWallets > 0 {
		a = append(a, "--max-resident-wallets="+strconv.Itoa(c.ResidentWallets))
	}
	return a
}
func (c Config) WalletURL() string { return "http://127.0.0.1:" + strconv.Itoa(c.WalletPort) }
func (c Config) Args(root string) []string {
	a := []string{"--appdir=" + c.DataDir, "--logdir=" + filepath.Join(root, "logs"), "--rpclisten=127.0.0.1:" + strconv.Itoa(c.GRPC), "--disable-upnp", "--utxoindex"}
	if c.PublicP2P {
		a = append(a, "--listen=0.0.0.0:16811")
	} else {
		a = append(a, "--listen=127.0.0.1:16811")
	}
	if c.EnableWebSocket {
		a = append(a, "--rpclisten-json=127.0.0.1:"+strconv.Itoa(c.WebSocket))
	}
	if c.Mode == "archive" {
		a = append(a, "--archival")
	}
	if c.Mode == "pruned" {
		a = append(a, "--shielded-history=off")
	} else {
		a = append(a, "--shielded-history=on", "--verify-shielded-history")
	}
	return a
}
func ReadConfig(root string) (Config, error) {
	c := Defaults(root)
	b, e := os.ReadFile(filepath.Join(root, "config.json"))
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	// Preserve the node-only behavior of an existing v0.1 installation until the
	// operator explicitly chooses to add the new component.
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) == nil {
		if _, present := fields["enableWalletBackend"]; !present {
			c.EnableWallet = false
		}
	}
	if e == nil {
		e = c.Validate()
	}
	return c, e
}
func Save(root string, c Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	return AtomicJSON(filepath.Join(root, "config.json"), c)
}
func AtomicJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".pending-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), path)
}
func FileHash(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}
func CheckBinary(c Config) error {
	h, e := FileHash(c.Executable)
	if e != nil {
		return e
	}
	if h != c.SHA256 || len(h) != 64 {
		return errors.New("Installed node checksum changed; reinstall the official release")
	}
	if c.EnableWallet {
		if c.WalletExecutable == "" {
			return errors.New("Wallet backend is not installed. Click Install / repair components")
		}
		h, e = FileHash(c.WalletExecutable)
		if e != nil {
			return e
		}
		if h != c.WalletSHA256 || len(h) != 64 {
			return errors.New("Installed wallet backend checksum changed; reinstall the components")
		}
	}
	return nil
}
func CheckPorts(c Config) error {
	ports := []int{c.GRPC, 16811}
	if c.EnableWebSocket {
		ports = append(ports, c.WebSocket)
	}
	if c.EnableWallet {
		ports = append(ports, c.WalletPort)
	}
	return CheckTCPPorts(ports...)
}
func CheckTCPPorts(ports ...int) error {
	var held []net.Listener
	defer func() {
		for _, l := range held {
			l.Close()
		}
	}()
	for _, p := range ports {
		l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if e != nil {
			return fmt.Errorf("port %d is already in use; stop the other node or change the RPC port", p)
		}
		held = append(held, l)
	}
	return nil
}

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}
type Release struct {
	Tag        string    `json:"tag_name"`
	Body       string    `json:"body"`
	Published  time.Time `json:"published_at"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Assets     []Asset   `json:"assets"`
}

var tagPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,90}$`)
var digestPattern = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)

func (r Release) WindowsAsset() (Asset, error) {
	if r.Draft || r.Prerelease || !tagPattern.MatchString(r.Tag) {
		return Asset{}, errors.New("Release is not a supported stable release")
	}
	var found []Asset
	for _, a := range r.Assets {
		if strings.HasSuffix(a.Name, "-win64.zip") {
			found = append(found, a)
		}
	}
	if len(found) != 1 {
		return Asset{}, errors.New("Expected exactly one official -win64.zip asset")
	}
	a := found[0]
	if !digestPattern.MatchString(a.Digest) {
		return Asset{}, errors.New("This release has no SHA-256 digest; installation was refused")
	}
	if a.URL != Repository+"/releases/download/"+r.Tag+"/"+a.Name || strings.ContainsAny(a.Name, "/\\") {
		return Asset{}, errors.New("Unexpected release download location")
	}
	if a.Size <= 0 || a.Size > 512<<20 {
		return Asset{}, errors.New("Unexpected package size")
	}
	return a, nil
}
func Latest(ctx context.Context) (Release, error) {
	var r Release
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", LatestAPI, nil)
	if e != nil {
		return r, e
	}
	req.Header.Set("User-Agent", "ZKas-Node-Manager/"+ManagerVersion)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return r, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return r, fmt.Errorf("GitHub returned %s; retry later (public API rate limits may apply)", resp.Status)
	}
	e = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&r)
	if e == nil {
		_, e = r.WindowsAsset()
	}
	return r, e
}

// Install stages and verifies the new executable before any running node is stopped.
func Install(ctx context.Context, root string, r Release, includeWallet bool, progress func(string), extra ...string) (string, string, error) {
	a, e := r.WindowsAsset()
	if e != nil {
		return "", "", e
	}
	base := filepath.Join(root, "versions")
	if e = os.MkdirAll(base, 0700); e != nil {
		return "", "", e
	}
	stage, e := os.MkdirTemp(base, "stage-")
	if e != nil {
		return "", "", e
	}
	defer os.RemoveAll(stage)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
	if e != nil {
		return "", "", e
	}
	req.Header.Set("User-Agent", "ZKas-Node-Manager/"+ManagerVersion)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return "", "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("Download returned %s", resp.Status)
	}
	zipPath := filepath.Join(stage, "package.zip")
	f, e := os.Create(zipPath)
	if e != nil {
		return "", "", e
	}
	h := sha256.New()
	buf := make([]byte, 128*1024)
	var n int64
	last := -1
	for {
		count, readErr := resp.Body.Read(buf)
		if count > 0 {
			n += int64(count)
			if n > a.Size {
				e = errors.New("Package exceeds its advertised size")
				break
			}
			if _, e = f.Write(buf[:count]); e != nil {
				break
			}
			h.Write(buf[:count])
			p := int(n * 100 / a.Size)
			if p != last {
				progress(fmt.Sprintf("Downloading %s — %d%%", r.Tag, p))
				last = p
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			e = readErr
			break
		}
	}
	ce := f.Close()
	if e != nil {
		return "", "", e
	}
	if ce != nil {
		return "", "", ce
	}
	if n != a.Size || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), strings.TrimPrefix(a.Digest, "sha256:")) {
		return "", "", errors.New("Package checksum or size mismatch; nothing was installed")
	}
	progress("Verified SHA-256. Extracting node…")
	exe := filepath.Join(stage, "kaspad.exe")
	if e = ExtractNode(zipPath, exe); e != nil {
		return "", "", e
	}
	if includeWallet {
		if e = ExtractExecutable(zipPath, filepath.Join(stage, "zkas-walletd.exe"), "zkas-walletd.exe"); e != nil {
			return "", "", e
		}
	}
	for _, name := range extra {
		if name != "stratum-bridge.exe" {
			return "", "", errors.New("Unsupported extra component")
		}
		if e = ExtractExecutable(zipPath, filepath.Join(stage, name), name); e != nil {
			return "", "", e
		}
	}
	hash, e := FileHash(exe)
	if e != nil {
		return "", "", e
	}
	os.Remove(zipPath)
	// Unique destination prevents replacing an executable used by an existing process.
	dest := filepath.Join(base, r.Tag+"-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if e = os.Rename(stage, dest); e != nil {
		return "", "", e
	}
	return filepath.Join(dest, "kaspad.exe"), hash, nil
}
func ExtractNode(zipPath, dest string) error { return ExtractExecutable(zipPath, dest, "kaspad.exe") }
func ExtractExecutable(zipPath, dest, name string) error {
	z, e := zip.OpenReader(zipPath)
	if e != nil {
		return e
	}
	defer z.Close()
	var candidates []*zip.File
	for _, f := range z.File {
		if f.Name == name {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) != 1 {
		return fmt.Errorf("Release must contain exactly one root %s", name)
	}
	f := candidates[0]
	if f.Mode()&os.ModeSymlink != 0 || f.UncompressedSize64 < 1024 || f.UncompressedSize64 > 256<<20 {
		return errors.New("Invalid node executable in archive")
	}
	in, e := f.Open()
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if e != nil {
		return e
	}
	n, e := io.Copy(out, io.LimitReader(in, 256<<20+1))
	ce := out.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if uint64(n) != f.UncompressedSize64 {
		return errors.New("Invalid extracted size")
	}
	b, e := os.ReadFile(dest)
	if e != nil {
		return e
	}
	if len(b) < 64 || string(b[:2]) != "MZ" {
		return errors.New("Node is not a Windows executable")
	}
	return nil
}
