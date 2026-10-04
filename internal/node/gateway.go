package node

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"
)

// Public access is restricted to documented non-custodial API operations.
// Seed, custodial, admin and unknown endpoints never reach the daemon.
var sharedRoutes = map[string]bool{
	"GET /health": true, "GET /api/status": true,
	"POST /api/wallet/watch": true, "GET /api/wallet/address": true,
	"GET /api/wallet/balance": true, "GET /api/wallet/history": true,
	"POST /api/wallet/settings": true, "POST /api/wallet/rescan": true,
	"POST /api/wallet/prepare": true, "POST /api/wallet/submit": true,
	"POST /api/verify": true,
}

func GatewayHandler(walletPort int, token string, requireToken bool) http.Handler {
	target, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(walletPort))
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{Proxy: nil, ResponseHeaderTimeout: 5 * time.Minute, MaxIdleConnsPerHost: 16}
	original := proxy.Director
	proxy.Director = func(r *http.Request) {
		original(r)
		r.Header.Del("Authorization")
		r.Header.Del("Forwarded")
		r.Header.Del("X-Forwarded-For")
		r.Header.Del("X-Forwarded-Host")
		r.Header.Del("X-Forwarded-Proto")
		r.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "Local wallet API is unavailable", http.StatusBadGateway)
	}
	// Bound expensive concurrent proving/scan calls across clients.
	slots := make(chan struct{}, 16)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if requireToken && (len(token) < 32 || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="ZKas API"`)
			http.Error(w, "Access token required", 401)
			return
		}
		// Browser-based access must be integrated explicitly; no wildcard CORS.
		if r.Header.Get("Origin") != "" {
			http.Error(w, "Use a native API client", 403)
			return
		}
		if !sharedRoutes[r.Method+" "+r.URL.Path] {
			http.Error(w, "This endpoint is not shared", 403)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			w.Header().Set("Retry-After", "5")
			http.Error(w, "API busy; retry later", 429)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		proxy.ServeHTTP(w, r)
	})
}
func (s SharingConfig) ConnectionDetails(root string) string {
	result := ""
	if s.HTTPS {
		result += "Wallet REST API: " + s.URL() + "\r\n"
		if s.PublicAPI {
			result += "Public registration enabled. Each wallet still requires its own X-Wallet-Token.\r\n"
		} else {
			result += "Send Authorization: Bearer <your access token>. Copy the token separately.\r\n"
		}
		if s.CertFile == "" {
			result += "TLS: self-signed; clients must trust this certificate explicitly.\r\n"
		}
	}
	if s.TorMode != "off" {
		if h := s.OnionHost(root); h != "" {
			result += "Tor wallet REST API: http://" + h + "\r\n"
		} else {
			result += "Onion address pending: start sharing and wait for Tor.\r\n"
		}
		if s.TorMode == "personal" {
			result += "Personal onion: Tor client authorization key required.\r\n"
		} else {
			result += "Public onion: anyone with the address may register a watch-only wallet.\r\n"
		}
	}
	if result == "" {
		result = "API sharing disabled. Local APIs remain available."
	}
	return result + fmt.Sprint("\r\nRemote APIs expose watch-only operations; node gRPC and JSON wRPC remain local.")
}
