package node

import (
	"bytes"
	"crypto/ecdh"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSharingDefaultsPreserveExistingInstallation(t *testing.T) {
	root := t.TempDir()
	c := Defaults(root)
	if e := Save(root, c); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(root, "config.json"))
	s, e := ReadSharing(root)
	if e != nil || s.HTTPS || s.TorMode != "off" || s.PublicAPI {
		t.Fatalf("unsafe defaults %+v %v", s, e)
	}
	if _, e = s.CertificateFingerprint(root); e == nil {
		t.Fatal("unexpected certificate")
	}
	if _, e = os.Stat(filepath.Join(root, "sharing")); !os.IsNotExist(e) {
		t.Fatal("read-only operations created sharing state")
	}
	after, _ := os.ReadFile(filepath.Join(root, "config.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("changed node config")
	}
	private := strings.Join(c.Args(root), " ")
	if !strings.Contains(private, "--listen=127.0.0.1:16811") {
		t.Fatal(private)
	}
	c.PublicP2P = true
	public := strings.Join(c.Args(root), " ")
	if !strings.Contains(public, "--listen=0.0.0.0:16811") || !strings.Contains(public, "--rpclisten=127.0.0.1:") {
		t.Fatal(public)
	}
}
func TestGatewayAuthenticationAndEndpointBoundary(t *testing.T) {
	var gotAuth, gotWallet string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotWallet = r.Header.Get("X-Wallet-Token")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
	p, _ := strconv.Atoi(port)
	token := strings.Repeat("a", 64)
	handler := GatewayHandler(p, token, true)
	for _, test := range []struct {
		method, path, auth, origin string
		code                       int
	}{
		{"GET", "/api/status", "", "", 401}, {"GET", "/api/status", "Bearer wrong", "", 401},
		{"GET", "/api/status", "Bearer " + token, "", 200},
		{"POST", "/api/wallet/create", "Bearer " + token, "", 403},
		{"GET", "/api/wallet/reveal", "Bearer " + token, "", 403},
		{"POST", "/api/wallet/send", "Bearer " + token, "", 403},
		{"GET", "/api/config", "Bearer " + token, "", 403},
		{"POST", "/api/wallet/watch", "Bearer " + token, "https://untrusted.example", 403},
		{"POST", "/api/wallet/watch", "Bearer " + token, "", 200},
		{"POST", "/api/wallet/watch/../reveal", "Bearer " + token, "", 403},
	} {
		r := httptest.NewRequest(test.method, test.path, strings.NewReader("{}"))
		r.Header.Set("Authorization", test.auth)
		r.Header.Set("Origin", test.origin)
		r.Header.Set("X-Wallet-Token", "wallet-test-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.code {
			t.Fatalf("%s %s: got %d want %d", test.method, test.path, w.Code, test.code)
		}
	}
	if gotAuth != "" || gotWallet != "wallet-test-token" {
		t.Fatalf("incorrect credentials forwarding: %q %q", gotAuth, gotWallet)
	}
	public := GatewayHandler(p, "", false)
	w := httptest.NewRecorder()
	public.ServeHTTP(w, httptest.NewRequest("GET", "/api/status", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	closed := GatewayHandler(p, "", true)
	w = httptest.NewRecorder()
	closed.ServeHTTP(w, httptest.NewRequest("GET", "/api/status", nil))
	if w.Code != 401 {
		t.Fatal("empty personal token failed open")
	}
}
func TestPersonalOnionKeyAndSeparatePublicIdentity(t *testing.T) {
	root := t.TempDir()
	s := DefaultSharing()
	s.TorMode = "personal"
	conf, e := s.TorConfig(root)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(conf, "SocksPort 0") || !strings.Contains(conf, "127.0.0.1:18502") {
		t.Fatal(conf)
	}
	keyPath := filepath.Join(root, "sharing", "onion-client.key")
	keyBytes, _ := os.ReadFile(keyPath)
	key, e := ecdh.X25519().NewPrivateKey(keyBytes)
	if e != nil {
		t.Fatal(e)
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding)
	auth, _ := os.ReadFile(filepath.Join(s.OnionDir(root), "authorized_clients", "owner.auth"))
	if string(auth) != "descriptor:x25519:"+encoded.EncodeToString(key.PublicKey().Bytes())+"\n" {
		t.Fatal("invalid Tor server authorization")
	}
	host := strings.Repeat("a", 56) + ".onion"
	os.WriteFile(filepath.Join(s.OnionDir(root), "hostname"), []byte(host), 0600)
	creds, e := s.OnionClientCredentials(root)
	if e != nil || creds != strings.TrimSuffix(host, ".onion")+":descriptor:x25519:"+encoded.EncodeToString(keyBytes) {
		t.Fatal(creds, e)
	}
	if _, e = s.TorConfig(root); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(keyPath)
	if !bytes.Equal(keyBytes, after) {
		t.Fatal("onion key changed on restart")
	}
	privateDir := s.OnionDir(root)
	s.TorMode = "public"
	if s.OnionDir(root) == privateDir {
		t.Fatal("public identity reused private address")
	}
	if _, e = s.TorConfig(root); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(s.OnionDir(root), "authorized_clients", "owner.auth")); !os.IsNotExist(e) {
		t.Fatal("private authorization copied to public")
	}
	if _, e = s.OnionClientCredentials(root); e == nil {
		t.Fatal("public mode exported private key")
	}
}
func TestGeneratedHTTPSCertificateIdentityAndTrust(t *testing.T) {
	root := t.TempDir()
	s := DefaultSharing()
	s.HTTPS = true
	s.Host = "127.0.0.1"
	certPath, keyPath, fp, e := s.EnsureCertificate(root)
	if e != nil {
		t.Fatal(e)
	}
	_, _, next, e := s.EnsureCertificate(root)
	if e != nil || fp != next {
		t.Fatal("certificate regenerated")
	}
	pair, e := tls.LoadX509KeyPair(certPath, keyPath)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	b, _ := os.ReadFile(certPath)
	if !roots.AppendCertsFromPEM(b) {
		t.Fatal("invalid certificate")
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}}
	defer client.CloseIdleConnections()
	res, e := client.Get(server.URL)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(body) != "ok" {
		t.Fatal(string(body))
	}
	block, _ := pem.Decode(b)
	cert, _ := x509.ParseCertificate(block.Bytes)
	if cert.VerifyHostname("unrelated.example") == nil {
		t.Fatal("wrong hostname accepted")
	}
	s.Host = "wrong.example"
	s.CertFile = certPath
	s.KeyFile = keyPath
	if _, _, _, e = s.EnsureCertificate(root); e == nil {
		t.Fatal("mismatched custom certificate accepted")
	}
}
func TestSharingRejectsPortCollisionsAndBadHosts(t *testing.T) {
	c := Defaults(t.TempDir())
	s := DefaultSharing()
	s.HTTPS = true
	s.Host = "node.example.com"
	if e := s.Validate(c); e != nil {
		t.Fatal(e)
	}
	for _, p := range []int{c.WalletPort, c.GRPC, c.WebSocket, 16811, 5555, OnionGatewayPort, MiningStatsPort, 65536} {
		s.Port = p
		if s.Validate(c) == nil {
			t.Fatalf("accepted port %d", p)
		}
	}
	s.Port = 8443
	for _, host := range []string{"https://example.com", "example.com:8443", "example.com/a", "evil\nSocksPort 9999", "", "abc.onion"} {
		s.Host = host
		if s.Validate(c) == nil {
			t.Fatalf("accepted host %q", host)
		}
	}
}

func TestMiningStatsRealSchemaAndUnknownData(t *testing.T) {
	fixture := `{"activeWorkers":1,"totalBlocks":2,"totalShares":14,"bridgeUptime":120,"workers":[{"worker":"rig1","wallet":"zkas:test","status":"online","hashrate":125.5,"shares":14,"stale":1,"invalid":2,"blocks":2}],"blocks":[{"worker":"rig1","timestamp":"2026-10-01","hash":"abc123"}]}`
	s, e := DecodeMiningStats(strings.NewReader(fixture))
	if e != nil {
		t.Fatal(e)
	}
	if s.ActiveWorkers != 1 || s.Workers[0].Hashrate != 125.5 || s.TotalBlocks != 2 || !strings.Contains(s.Display(), "125.500 GH/s") || !strings.Contains(s.Display(), "abc123") {
		t.Fatal(s)
	}
	for _, bad := range []string{`{}`, `{"status":"ok"}`, `<html>error</html>`, `{"activeWorkers":"wrong","totalBlocks":0,"totalShares":0,"workers":[],"blocks":[]}`} {
		if _, e := DecodeMiningStats(strings.NewReader(bad)); e == nil {
			t.Fatalf("accepted unknown stats: %s", bad)
		}
	}
	args := strings.Join(DefaultMining().Args(t.TempDir(), 16810), " ")
	if !strings.Contains(args, "--web-dashboard-port=127.0.0.1:18888") {
		t.Fatal("metrics not local-only")
	}
}
