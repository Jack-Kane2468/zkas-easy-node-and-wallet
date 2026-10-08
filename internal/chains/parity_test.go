package chains

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestKaspaPurposesAndPorts(t *testing.T) {
	root := t.TempDir()
	c := Config{NodeMode: "basic", DataPath: filepath.Join(root, "custom-data"), GRPC: 26110, JSON: 28110, Borsh: 27110, P2P: 26111, RAMScale: 0.3}
	if e := Save(root, c); e != nil {
		t.Fatal(e)
	}
	if e := c.ValidateNode(); e != nil {
		t.Fatal(e)
	}
	a := strings.Join(KaspaArgs(root), " ")
	if strings.Contains(a, "--utxoindex") || strings.Contains(a, "--archival") || !strings.Contains(a, "--rpclisten=127.0.0.1:26110") {
		t.Fatal(a)
	}
	c.NodeMode = "archive"
	c.PublicP2P = true
	Save(root, c)
	a = strings.Join(KaspaArgs(root), " ")
	for _, want := range []string{"--utxoindex", "--archival", "--listen=0.0.0.0:26111", "--appdir=" + c.DataPath} {
		if !strings.Contains(a, want) {
			t.Fatal(a)
		}
	}
	if strings.Contains(a, "--unsaferpc") {
		t.Fatal("unsafe RPC enabled")
	}
	c.JSON = c.GRPC
	if c.ValidateNode() == nil {
		t.Fatal("duplicate port accepted")
	}
	c.JSON = 18503
	if c.ValidateNode() == nil {
		t.Fatal("Tor port accepted")
	}
}
func TestExistingConfigKeepsWalletModeAndDefaultPorts(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "chains.json"), []byte(`{"mode":"zkas","ramScale":0.3,"dualBridge":{"path":"installed.exe"}}`), 0600)
	c, e := Read(root)
	if e != nil || c.NodeMode != "wallet" || c.GRPC != 16110 || c.DualBridge.Path != "installed.exe" {
		t.Fatal(c, e)
	}
}
func TestWRPCGatewayAuthorizationAndFixedRoutes(t *testing.T) {
	hits := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("Authorization") != "" || r.URL.Path != "/" || r.URL.RawQuery != "" {
			t.Errorf("credentials/path forwarded: %s", r.URL)
		}
		w.WriteHeader(200)
	}))
	defer up.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(up.URL, "http://"))
	p, _ := strconv.Atoi(port)
	gateway := WRPCGateway(p, p, "private-test-token", true)
	cases := []struct {
		path, token, upgrade string
		want                 int
	}{{"/borsh", "", "websocket", 401}, {"/borsh", "wrong", "websocket", 401}, {"/admin", "private-test-token", "websocket", 400}, {"/json", "private-test-token", "", 400}, {"/borsh?target=http://example.invalid", "private-test-token", "websocket", 200}}
	for _, c := range cases {
		r := httptest.NewRequest("GET", c.path, nil)
		r.Header.Set("Authorization", "Bearer "+c.token)
		r.Header.Set("Upgrade", c.upgrade)
		r.Header.Set("Connection", "Upgrade")
		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Fatalf("%s: %d", c.path, w.Code)
		}
	}
	if hits != 1 {
		t.Fatal(hits)
	}
}
