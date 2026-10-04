package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const StratumPort = 5555

// Mining has its own configuration, so adding it never rewrites node/wallet settings.
type MiningConfig struct {
	Executable        string `json:"executable"`
	SHA256            string `json:"sha256"`
	Version           string `json:"version"`
	HostExecutable    string `json:"hostExecutable"`
	Address           string `json:"payoutAddress"`
	Worker            string `json:"worker"`
	MinimumDifficulty uint32 `json:"minimumDifficulty"`
}

func DefaultMining() MiningConfig { return MiningConfig{Worker: "miner1", MinimumDifficulty: 8192} }
func ReadMining(root string) (MiningConfig, error) {
	c := DefaultMining()
	b, e := os.ReadFile(filepath.Join(root, "mining.json"))
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	return c, e
}
func SaveMining(root string, c MiningConfig) error {
	return AtomicJSON(filepath.Join(root, "mining.json"), c)
}

var miningAddress = regexp.MustCompile(`^zkas:[a-z0-9]{30,200}$`)
var workerName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func (c MiningConfig) Validate() error {
	if c.Address != "" && !miningAddress.MatchString(c.Address) {
		return errors.New("Use a mainnet zkas: payout address, without a worker suffix")
	}
	if !workerName.MatchString(c.Worker) {
		return errors.New("Worker name must be 1–32 letters, digits, underscores or hyphens")
	}
	if c.MinimumDifficulty == 0 {
		return errors.New("Minimum share difficulty must be at least 1")
	}
	return nil
}
func (c MiningConfig) Username() string {
	if c.Address == "" {
		return "YOUR_ZKAS_ADDRESS." + c.Worker
	}
	return c.Address + "." + c.Worker
}
func StratumURL(host string) string { return "stratum+tcp://" + net.JoinHostPort(host, "5555") }
func (c MiningConfig) Args(root string, grpcPort int) []string {
	return []string{"--node-mode=external", "--config=" + filepath.Join(root, "mining", "bridge-config.yaml"), "--kaspad-address=127.0.0.1:" + strconv.Itoa(grpcPort), "--stratum-port=0.0.0.0:5555", "--min-share-diff=" + strconv.FormatUint(uint64(c.MinimumDifficulty), 10), "--var-diff=true", "--shares-per-min=20", "--log-to-file=false", "--print-stats=true", "--web-dashboard-port=127.0.0.1:18888"}
}
func CheckMiningBinary(c MiningConfig) error {
	if c.Executable == "" {
		return errors.New("Mining bridge is not installed")
	}
	h, e := FileHash(c.Executable)
	if e != nil {
		return e
	}
	if h != c.SHA256 || len(h) != 64 {
		return errors.New("Mining bridge checksum changed; reinstall the bridge")
	}
	return nil
}
func CheckMiningPort() error {
	l, e := net.Listen("tcp4", "0.0.0.0:5555")
	if e != nil {
		return errors.New("TCP port 5555 is already in use; stop the other Stratum server first")
	}
	return l.Close()
}
func ReleaseAt(ctx context.Context, tag string) (Release, error) {
	var r Release
	if !tagPattern.MatchString(tag) {
		return r, errors.New("Invalid installed release tag")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/firecash/zkas-rusty/releases/tags/"+tag, nil)
	if e != nil {
		return r, e
	}
	req.Header.Set("User-Agent", "ZKas-Node-Manager/"+ManagerVersion)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return r, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return r, fmt.Errorf("GitHub release lookup returned %s", resp.Status)
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&r); e != nil {
		return r, e
	}
	if r.Tag != tag {
		return r, errors.New("Release did not match the installed node version")
	}
	_, e = r.WindowsAsset()
	return r, e
}

type LANAddress struct {
	IP    string
	Label string
}

func LANAddresses() []LANAddress {
	result := []LANAddress{}
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ip, _, e := net.ParseCIDR(address.String())
			if e != nil || ip.To4() == nil || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() {
				continue
			}
			result = append(result, LANAddress{ip.String(), iface.Name + " — " + ip.String()})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := net.ParseIP(result[i].IP).IsPrivate(), net.ParseIP(result[j].IP).IsPrivate()
		if a != b {
			return a
		}
		return strings.Compare(result[i].Label, result[j].Label) < 0
	})
	return append(result, LANAddress{"127.0.0.1", "This PC only — 127.0.0.1"})
}
