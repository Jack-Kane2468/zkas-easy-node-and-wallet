package node

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const OnionGatewayPort = 18502

type SharingConfig struct {
	HTTPS          bool   `json:"https"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	PublicAPI      bool   `json:"publicAPI"`
	Token          string `json:"token"`
	CertFile       string `json:"certificateFile,omitempty"`
	KeyFile        string `json:"keyFile,omitempty"`
	TorMode        string `json:"torMode"` // off, personal (Tor client auth), public
	TorExecutable  string `json:"torExecutable,omitempty"`
	TorSHA256      string `json:"torSHA256,omitempty"`
	HostExecutable string `json:"hostExecutable,omitempty"`
}

func DefaultSharing() SharingConfig { return SharingConfig{Port: 8443, TorMode: "off"} }
func ReadSharing(root string) (SharingConfig, error) {
	c := DefaultSharing()
	b, e := os.ReadFile(filepath.Join(root, "sharing", "config.json"))
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	return c, e
}
func SaveSharing(root string, c SharingConfig) error {
	return AtomicJSON(filepath.Join(root, "sharing", "config.json"), c)
}

var hostName = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9.-]{0,251}[a-zA-Z0-9])?$`)

func (s SharingConfig) Validate(c Config) error {
	if s.TorMode != "off" && s.TorMode != "personal" && s.TorMode != "public" {
		return fmt.Errorf("choose a Tor access mode")
	}
	if (s.HTTPS || s.TorMode != "off") && !c.EnableWallet {
		return fmt.Errorf("enable the wallet API in Settings first")
	}
	if s.HTTPS {
		if s.Host == "" || (!hostName.MatchString(s.Host) && net.ParseIP(s.Host) == nil) || strings.ContainsAny(s.Host, "\r\n\x00") {
			return fmt.Errorf("enter a public IP or domain only, without https://, a port, or a path")
		}
		if strings.HasSuffix(strings.ToLower(s.Host), ".onion") {
			return fmt.Errorf("use Tor mode for onion addresses; Tor generates its address")
		}
		if s.Port < 1024 || s.Port > 65535 || s.Port == 16811 || s.Port == 5555 || s.Port == MiningStatsPort || s.Port == OnionGatewayPort || s.Port == c.GRPC || s.Port == c.WalletPort || s.Port == c.WebSocket {
			return fmt.Errorf("choose a distinct HTTPS port between 1024 and 65535")
		}
	}
	if (s.CertFile == "") != (s.KeyFile == "") {
		return fmt.Errorf("select both certificate and private-key PEM files, or leave both blank")
	}
	if s.TorMode != "off" {
		if c.WalletPort == OnionGatewayPort || c.GRPC == OnionGatewayPort || c.EnableWebSocket && c.WebSocket == OnionGatewayPort {
			return fmt.Errorf("Tor gateway port 18502 conflicts with a local API; choose different local API ports before using Tor")
		}
		if !filepath.IsAbs(s.TorExecutable) || !strings.EqualFold(filepath.Base(s.TorExecutable), "tor.exe") {
			return fmt.Errorf("select tor.exe from the official Windows Tor Expert Bundle")
		}
		hash, e := FileHash(s.TorExecutable)
		if e != nil {
			return e
		}
		if hash != s.TorSHA256 || len(hash) != 64 {
			return fmt.Errorf("Tor executable changed; select it again to approve the new version")
		}
	}
	return nil
}
func NewSharingToken() (string, error) {
	b := make([]byte, 32)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
func (s SharingConfig) URL() string {
	return "https://" + net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}
func (s SharingConfig) OnionDir(root string) string {
	return filepath.Join(root, "sharing", "onion-"+s.TorMode)
}

var onionHost = regexp.MustCompile(`^[a-z2-7]{56}\.onion$`)

func (s SharingConfig) OnionHost(root string) string {
	if s.TorMode == "off" {
		return ""
	}
	b, e := os.ReadFile(filepath.Join(s.OnionDir(root), "hostname"))
	if e != nil {
		return ""
	}
	h := strings.TrimSpace(string(b))
	if !onionHost.MatchString(h) {
		return ""
	}
	return h
}
func (s SharingConfig) TorConfig(root string) (string, error) {
	if strings.ContainsAny(root+s.TorExecutable, "\r\n\x00\"") {
		return "", fmt.Errorf("unsupported character in Tor path")
	}
	dir := s.OnionDir(root)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	if s.TorMode == "personal" {
		// Keep personal and public onion identities separate. Switching mode never
		// turns a previously private onion address into a public address.
		keyPath := filepath.Join(root, "sharing", "onion-client.key")
		data, e := os.ReadFile(keyPath)
		if os.IsNotExist(e) {
			key, err := ecdh.X25519().GenerateKey(rand.Reader)
			if err != nil {
				return "", err
			}
			data = key.Bytes()
			e = os.WriteFile(keyPath, data, 0600)
		}
		if e != nil {
			return "", e
		}
		key, e := ecdh.X25519().NewPrivateKey(data)
		if e != nil {
			return "", e
		}
		authDir := filepath.Join(dir, "authorized_clients")
		if e = os.MkdirAll(authDir, 0700); e != nil {
			return "", e
		}
		pub := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key.PublicKey().Bytes())
		if e = os.WriteFile(filepath.Join(authDir, "owner.auth"), []byte("descriptor:x25519:"+pub+"\n"), 0600); e != nil {
			return "", e
		}
	}
	quote := func(p string) string { return strconv.Quote(filepath.ToSlash(p)) }
	return "DataDirectory " + quote(filepath.Join(root, "sharing", "tor-data")) + "\nSocksPort 0\nControlPort 0\nLog notice stdout\nHiddenServiceDir " + quote(dir) + "\nHiddenServiceVersion 3\nHiddenServicePort 80 127.0.0.1:" + strconv.Itoa(OnionGatewayPort) + "\n", nil
}
func (s SharingConfig) OnionClientCredentials(root string) (string, error) {
	h := s.OnionHost(root)
	if s.TorMode != "personal" || h == "" {
		return "", fmt.Errorf("start personal Tor access and wait for its address first")
	}
	b, e := os.ReadFile(filepath.Join(root, "sharing", "onion-client.key"))
	if e != nil {
		return "", e
	}
	key := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	return strings.TrimSuffix(h, ".onion") + ":descriptor:x25519:" + key, e
}

// EnsureCertificate preserves the generated identity across launches. Custom
// certificates must match the configured host; no global TLS verification bypass.
func (s SharingConfig) EnsureCertificate(root string) (string, string, string, error) {
	certPath, keyPath := s.CertFile, s.KeyFile
	if certPath == "" {
		id := sha256.Sum256([]byte(s.Host))
		base := filepath.Join(root, "sharing", "tls-"+hex.EncodeToString(id[:8]))
		certPath, keyPath = base+".pem", base+".key"
		if _, e := os.Stat(certPath); os.IsNotExist(e) {
			key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return "", "", "", err
			}
			serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
			if err != nil {
				return "", "", "", err
			}
			cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: s.Host}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
			if ip := net.ParseIP(s.Host); ip != nil {
				cert.IPAddresses = []net.IP{ip}
			} else {
				cert.DNSNames = []string{s.Host}
			}
			der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
			if err != nil {
				return "", "", "", err
			}
			kb, err := x509.MarshalPKCS8PrivateKey(key)
			if err != nil {
				return "", "", "", err
			}
			if err = os.MkdirAll(filepath.Dir(certPath), 0700); err != nil {
				return "", "", "", err
			}
			if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kb}), 0600); err != nil {
				return "", "", "", err
			}
			if err = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
				return "", "", "", err
			}
		}
	}
	pair, e := tls.LoadX509KeyPair(certPath, keyPath)
	if e != nil {
		return "", "", "", e
	}
	cert, e := x509.ParseCertificate(pair.Certificate[0])
	if e != nil {
		return "", "", "", e
	}
	if e = cert.VerifyHostname(s.Host); e != nil {
		return "", "", "", e
	}
	if time.Now().After(cert.NotAfter) || time.Now().Before(cert.NotBefore) {
		return "", "", "", fmt.Errorf("TLS certificate is expired or not yet valid; replace the certificate")
	}
	h := sha256.Sum256(cert.Raw)
	return certPath, keyPath, hex.EncodeToString(h[:]), nil
}

// Read-only UI inspection must never generate a certificate or modify settings.
func (s SharingConfig) CertificateFingerprint(root string) (string, error) {
	path := s.CertFile
	if path == "" {
		id := sha256.Sum256([]byte(s.Host))
		path = filepath.Join(root, "sharing", "tls-"+hex.EncodeToString(id[:8])+".pem")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return "", fmt.Errorf("invalid certificate")
	}
	cert, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(h[:]), nil
}
