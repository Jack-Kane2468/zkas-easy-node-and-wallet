package chains

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// WRPCGateway exposes only fixed local wRPC endpoints, never arbitrary URLs.
// Kaspad's unsafe RPC flag stays disabled. Authorization protects the upgrade.
func WRPCGateway(borsh, jsonPort int, token string, private bool) http.Handler {
	proxies := map[string]*httputil.ReverseProxy{}
	for p, port := range map[string]int{"/borsh": borsh, "/json": jsonPort} {
		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
		proxy := httputil.NewSingleHostReverseProxy(target)
		original := proxy.Director
		proxy.Director = func(r *http.Request) {
			original(r)
			r.URL.Path = "/"
			r.URL.RawPath = ""
			r.URL.RawQuery = ""
			r.Header.Del("Authorization")
			r.Header.Del("Cookie")
			r.Header.Del("X-Forwarded-For")
			r.Host = target.Host
		}
		proxy.Transport = &http.Transport{Proxy: nil}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			http.Error(w, "Local Kaspa node unavailable", http.StatusBadGateway)
		}
		proxies[p] = proxy
	}
	slots := make(chan struct{}, 64)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if private && (token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1) {
			http.Error(w, "Access token required", http.StatusUnauthorized)
			return
		}
		proxy := proxies[r.URL.Path]
		if proxy == nil || r.Method != "GET" || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "Use a WebSocket client with /borsh or /json", http.StatusBadRequest)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "Connection limit reached", http.StatusServiceUnavailable)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}
