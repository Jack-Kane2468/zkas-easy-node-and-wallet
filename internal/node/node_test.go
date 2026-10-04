package node

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protowire"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigArgsAndValidation(t *testing.T) {
	c := Defaults(t.TempDir())
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	args := strings.Join(c.Args(t.TempDir()), " ")
	for _, s := range []string{"--rpclisten=127.0.0.1:16810", "--rpclisten-json=127.0.0.1:18810", "--utxoindex", "--shielded-history=on", "--verify-shielded-history", "--disable-upnp"} {
		if !strings.Contains(args, s) {
			t.Error("missing", s)
		}
	}
	if strings.Contains(args, "unsaferpc") {
		t.Fatal("unsafe RPC enabled")
	}
	c.WebSocket = c.GRPC
	if c.Validate() == nil {
		t.Fatal("accepted conflicting ports")
	}
	c.WebSocket = 18810
	c.GRPC = 16811
	if c.Validate() == nil {
		t.Fatal("accepted peer port collision")
	}
}
func TestConfigRoundTrip(t *testing.T) {
	root := t.TempDir()
	c := Defaults(root)
	if e := Save(root, c); e != nil {
		t.Fatal(e)
	}
	got, e := ReadConfig(root)
	if e != nil || got != c {
		t.Fatalf("roundtrip: %v %v", got, e)
	}
}
func validRelease() Release {
	return Release{Tag: "zkas-v1.0.9", Assets: []Asset{{Name: "zkas-zkas-v1.0.9-win64.zip", URL: Repository + "/releases/download/zkas-v1.0.9/zkas-zkas-v1.0.9-win64.zip", Digest: "sha256:" + strings.Repeat("a", 64), Size: 1024}}}
}
func TestRejectUntrustedReleases(t *testing.T) {
	for _, mutate := range []func(*Release){func(r *Release) { r.Prerelease = true }, func(r *Release) { r.Draft = true }, func(r *Release) { r.Tag = "../bad" }, func(r *Release) { r.Assets[0].Digest = "" }, func(r *Release) { r.Assets[0].URL = "https://example.com/node.zip" }, func(r *Release) { r.Assets = append(r.Assets, r.Assets[0]) }, func(r *Release) { r.Assets[0].Size = 1 << 40 }} {
		r := validRelease()
		mutate(&r)
		if _, e := r.WindowsAsset(); e == nil {
			t.Fatal("accepted invalid release")
		}
	}
	if _, e := validRelease().WindowsAsset(); e != nil {
		t.Fatal(e)
	}
}
func archive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, data := range entries {
		f, e := w.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		f.Write(data)
	}
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func fakeEXE() []byte { b := make([]byte, 2048); copy(b, "MZ"); return b }
func TestExtractionIgnoresTraversalAndRejectsInvalidEXE(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "p.zip")
	os.WriteFile(zipPath, archive(t, map[string][]byte{"kaspad.exe": fakeEXE(), "../escaped.exe": []byte("bad")}), 0600)
	if e := ExtractNode(zipPath, filepath.Join(root, "node.exe")); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(filepath.Dir(root), "escaped.exe")); e == nil {
		t.Fatal("zip escaped")
	}
	os.WriteFile(zipPath, archive(t, map[string][]byte{"kaspad.exe": make([]byte, 2048)}), 0600)
	if ExtractNode(zipPath, filepath.Join(root, "bad.exe")) == nil {
		t.Fatal("accepted invalid PE")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestStagedInstallAndChecksumFailurePreserveConfig(t *testing.T) {
	root := t.TempDir()
	original := Defaults(root)
	original.Version = "old"
	if e := Save(root, original); e != nil {
		t.Fatal(e)
	}
	payload := archive(t, map[string][]byte{"kaspad.exe": fakeEXE()})
	sum := sha256.Sum256(payload)
	r := validRelease()
	r.Assets[0].Size = int64(len(payload))
	r.Assets[0].Digest = "sha256:" + hex.EncodeToString(sum[:])
	oldClient := http.DefaultClient
	defer func() { http.DefaultClient = oldClient }()
	http.DefaultClient = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
	})}
	exe, hash, e := Install(context.Background(), root, r, false, func(string) {})
	if e != nil {
		t.Fatal(e)
	}
	if got, _ := FileHash(exe); got != hash {
		t.Fatal("wrong executable hash")
	}
	c, _ := ReadConfig(root)
	if c != original {
		t.Fatal("staging changed live config")
	}
	r.Assets[0].Digest = "sha256:" + strings.Repeat("0", 64)
	if _, _, e = Install(context.Background(), root, r, false, func(string) {}); e == nil {
		t.Fatal("accepted bad checksum")
	}
	c, _ = ReadConfig(root)
	if c != original {
		t.Fatal("failure changed live config")
	}
	os.WriteFile(exe, []byte("tampered"), 0600)
	if CheckBinary(Config{Executable: exe, SHA256: hash}) == nil {
		t.Fatal("accepted modified binary")
	}
}
func infoResponse(synced bool) []byte {
	b := protowire.AppendTag(nil, 3, protowire.BytesType)
	b = protowire.AppendString(b, "test-node")
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	if synced {
		b = protowire.AppendTag(b, 5, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	b = protowire.AppendTag(b, 999, protowire.VarintType)
	b = protowire.AppendVarint(b, 9)
	out := protowire.AppendTag(nil, 1064, protowire.BytesType)
	return protowire.AppendBytes(out, b)
}
func TestInfoDecoder(t *testing.T) {
	for _, synced := range []bool{true, false} {
		i, e := DecodeInfo(infoResponse(synced))
		if e != nil || i.Synced != synced || !i.Indexed || i.Version != "test-node" {
			t.Fatalf("bad decode: %+v %v", i, e)
		}
	}
	for _, b := range [][]byte{{0xff}, {}, protowire.AppendTag(nil, 1064, protowire.BytesType)} {
		if _, e := DecodeInfo(b); e == nil {
			t.Fatal("accepted malformed RPC")
		}
	}
}

type mockRPC interface{}

func TestGetInfoUsesUpstreamBidiStream(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := grpc.NewServer(grpc.ForceServerCodec(rawCodec{}))
	server.RegisterService(&grpc.ServiceDesc{ServiceName: "protowire.RPC", HandlerType: (*mockRPC)(nil), Streams: []grpc.StreamDesc{{StreamName: "MessageStream", ServerStreams: true, ClientStreams: true, Handler: func(_ any, s grpc.ServerStream) error {
		var req []byte
		if e := s.RecvMsg(&req); e != nil {
			return e
		}
		found := false
		e := eachField(req, func(n protowire.Number, _ protowire.Type, _ []byte) error {
			if n == 1063 {
				found = true
			}
			return nil
		})
		if e != nil || !found {
			return fmt.Errorf("missing GetInfo")
		}
		response := infoResponse(true)
		return s.SendMsg(&response)
	}}}}, struct{}{})
	go server.Serve(listener)
	defer server.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	i, e := GetInfo(ctx, listener.Addr().(*net.TCPAddr).Port)
	if e != nil || !i.Synced {
		t.Fatalf("RPC: %+v %v", i, e)
	}
}
func TestActualReleaseArchiveWhenProvided(t *testing.T) {
	path := os.Getenv("ZKAS_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set ZKAS_TEST_ARCHIVE to test downloaded official Windows package")
	}
	got, e := FileHash(path)
	if e != nil || got != "54e923dcdb96a7342c16ea1afe8275f326ed862696a72677c6a7a7d49041ff82" {
		t.Fatalf("official package mismatch: %s %v", got, e)
	}
	if e = ExtractNode(path, filepath.Join(t.TempDir(), "kaspad.exe")); e != nil {
		t.Fatal(e)
	}
	if e = ExtractExecutable(path, filepath.Join(t.TempDir(), "zkas-walletd.exe"), "zkas-walletd.exe"); e != nil {
		t.Fatal(e)
	}
	if e = ExtractExecutable(path, filepath.Join(t.TempDir(), "stratum-bridge.exe"), "stratum-bridge.exe"); e != nil {
		t.Fatal(e)
	}
}

func TestWalletProfileAndLegacyConfig(t *testing.T) {
	root := t.TempDir()
	c := Defaults(root)
	if !c.EnableWallet || c.WalletPort != 8501 {
		t.Fatal("missing wallet defaults")
	}
	args := strings.Join(c.WalletArgs(root), " ")
	for _, part := range []string{"--network=mainnet", "--rpc-server=127.0.0.1:16810", "--listen=127.0.0.1:8501", "--no-custodial", "--active-sync-window=1800"} {
		if !strings.Contains(args, part) {
			t.Fatal("missing", part)
		}
	}
	for _, part := range []string{"--allow-default-token", "--allow-remote", "--serve-public"} {
		if strings.Contains(args, part) {
			t.Fatal("unexpected", part)
		}
	}
	if c.WalletURL() != "http://127.0.0.1:8501" {
		t.Fatal("wrong wallet URL")
	}
	c.WalletPort = c.GRPC
	if c.Validate() == nil {
		t.Fatal("wallet port clash accepted")
	}
	c.WalletPort = 8501
	c.Mode = "pruned"
	if c.Validate() == nil {
		t.Fatal("wallet without history accepted")
	}
	// A v0.1 config has no enableWalletBackend field: upgrading the manager does
	// not silently change a running node-only installation.
	legacy := fmt.Sprintf(`{"dataDir":%q,"grpcPort":16810,"webSocketPort":18810,"mode":"pruned","nodeVersion":"old"}`, filepath.Join(root, "data"))
	os.WriteFile(filepath.Join(root, "config.json"), []byte(legacy), 0600)
	got, e := ReadConfig(root)
	if e != nil || got.EnableWallet {
		t.Fatalf("legacy migration failed: %+v %v", got, e)
	}
}
func TestBothExecutablesAreRequiredAndChecked(t *testing.T) {
	root := t.TempDir()
	payload := archive(t, map[string][]byte{"kaspad.exe": fakeEXE(), "zkas-walletd.exe": fakeEXE()})
	sum := sha256.Sum256(payload)
	r := validRelease()
	r.Assets[0].Size = int64(len(payload))
	r.Assets[0].Digest = "sha256:" + hex.EncodeToString(sum[:])
	oldClient := http.DefaultClient
	defer func() { http.DefaultClient = oldClient }()
	http.DefaultClient = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
	})}
	exe, hash, e := Install(context.Background(), root, r, true, func(string) {})
	if e != nil {
		t.Fatal(e)
	}
	c := Defaults(root)
	c.Executable = exe
	c.SHA256 = hash
	c.WalletExecutable = filepath.Join(filepath.Dir(exe), "zkas-walletd.exe")
	c.WalletSHA256, _ = FileHash(c.WalletExecutable)
	if e = CheckBinary(c); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(c.WalletExecutable, []byte("tampered"), 0600)
	if CheckBinary(c) == nil {
		t.Fatal("tampered wallet accepted")
	}
	onlyNode := filepath.Join(root, "only-node.zip")
	os.WriteFile(onlyNode, archive(t, map[string][]byte{"kaspad.exe": fakeEXE()}), 0600)
	if ExtractExecutable(onlyNode, filepath.Join(root, "missing.exe"), "zkas-walletd.exe") == nil {
		t.Fatal("missing wallet component accepted")
	}
}
func TestWalletHealthIsServiceSpecific(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Error("wrong health path")
		}
		io.WriteString(w, `{"ok":true,"service":"zkas-walletd"}`)
	})}
	go server.Serve(listener)
	defer server.Close()
	if e = WalletHealth(context.Background(), listener.Addr().(*net.TCPAddr).Port); e != nil {
		t.Fatal(e)
	}
}

func TestStatusMonitorReusesStream(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	opened := make(chan struct{}, 10)
	server := grpc.NewServer(grpc.ForceServerCodec(rawCodec{}))
	server.RegisterService(&grpc.ServiceDesc{ServiceName: "protowire.RPC", HandlerType: (*mockRPC)(nil), Streams: []grpc.StreamDesc{{StreamName: "MessageStream", ServerStreams: true, ClientStreams: true, Handler: func(_ any, s grpc.ServerStream) error {
		opened <- struct{}{}
		for {
			var request []byte
			if e := s.RecvMsg(&request); e != nil {
				return e
			}
			response := infoResponse(true)
			if e := s.SendMsg(&response); e != nil {
				return e
			}
		}
	}}}}, struct{}{})
	go server.Serve(listener)
	defer server.Stop()
	monitor := &InfoMonitor{}
	defer monitor.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	for i := 0; i < 4; i++ {
		info, e := monitor.Get(context.Background(), port)
		if e != nil || !info.Synced {
			t.Fatalf("poll %d: %v", i, e)
		}
	}
	if len(opened) != 1 {
		t.Fatalf("expected one RPC stream, got %d", len(opened))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	monitor.Get(ctx, port)
	if monitor.client != nil {
		monitor.Close()
	}
	// A later poll reconnects after cancellation or a lost connection.
	if _, e := monitor.Get(context.Background(), port); e != nil {
		t.Fatal(e)
	}
}

func TestMiningUsesExistingNodeAndSeparateConfig(t *testing.T) {
	root := t.TempDir()
	c := Defaults(root)
	if e := Save(root, c); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(root, "config.json"))
	mc := DefaultMining()
	if e := mc.Validate(); e != nil {
		t.Fatal(e)
	}
	if e := SaveMining(root, mc); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(root, "config.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("mining changed node configuration")
	}
	got, e := ReadMining(root)
	if e != nil || got != mc {
		t.Fatal("mining round trip", e)
	}
	args := strings.Join(mc.Args(root, 16810), " ")
	for _, part := range []string{"--node-mode=external", "--kaspad-address=127.0.0.1:16810", "--stratum-port=0.0.0.0:5555", "--var-diff=true", "--min-share-diff=8192"} {
		if !strings.Contains(args, part) {
			t.Fatal("missing", part)
		}
	}
	if StratumURL("192.168.1.25") != "stratum+tcp://192.168.1.25:5555" {
		t.Fatal("incorrect Stratum URL")
	}
	mc.Worker = "miner.1"
	if mc.Validate() == nil {
		t.Fatal("accepted ambiguous worker suffix")
	}
	mc.Worker = "miner1"
	mc.Address = "kaspa:wrong"
	if mc.Validate() == nil {
		t.Fatal("accepted non-ZKas payout prefix")
	}
}
func TestMiningPackageStaging(t *testing.T) {
	root := t.TempDir()
	payload := archive(t, map[string][]byte{"kaspad.exe": fakeEXE(), "stratum-bridge.exe": fakeEXE()})
	sum := sha256.Sum256(payload)
	r := validRelease()
	r.Assets[0].Size = int64(len(payload))
	r.Assets[0].Digest = "sha256:" + hex.EncodeToString(sum[:])
	oldClient := http.DefaultClient
	defer func() { http.DefaultClient = oldClient }()
	http.DefaultClient = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
	})}
	exe, _, e := Install(context.Background(), filepath.Join(root, "mining"), r, false, func(string) {}, "stratum-bridge.exe")
	if e != nil {
		t.Fatal(e)
	}
	mc := DefaultMining()
	mc.Executable = filepath.Join(filepath.Dir(exe), "stratum-bridge.exe")
	mc.SHA256, _ = FileHash(mc.Executable)
	if e = CheckMiningBinary(mc); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(root, "config.json")); !os.IsNotExist(e) {
		t.Fatal("created node config during mining staging")
	}
	os.WriteFile(mc.Executable, []byte("changed"), 0600)
	if CheckMiningBinary(mc) == nil {
		t.Fatal("accepted tampered bridge")
	}
}
