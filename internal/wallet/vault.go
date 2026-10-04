package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
	"zkas-node-manager/internal/node"
)

type Address struct {
	Index   uint32 `json:"index"`
	Address string `json:"address"`
}
type Receipt struct {
	Time   string `json:"time"`
	To     string `json:"to"`
	Amount string `json:"amount"`
	Fee    string `json:"fee"`
	TxID   string `json:"txid"`
	State  string `json:"state"`
}
type Wallet struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Secret    string    `json:"secret,omitempty"`
	Seed      string    `json:"seed,omitempty"`
	Account   uint32    `json:"account"`
	FVK       string    `json:"fvk"`
	Address   string    `json:"address"`
	Token     string    `json:"token"`
	Addresses []Address `json:"addresses"`
	NextIndex uint32    `json:"nextIndex"`
	SignedOut bool      `json:"signedOut"`
	Receipts  []Receipt `json:"receipts"`
}
type Vault struct {
	Wallets []Wallet `json:"wallets"`
}
type Envelope struct {
	Version int    `json:"version"`
	Salt    string `json:"salt"`
	Nonce   string `json:"nonce"`
	Data    string `json:"data"`
}

func Path(root string) string { return filepath.Join(root, "personal-wallets", "vault.json") }
func Exists(root string) bool { _, e := os.Stat(Path(root)); return e == nil }
func key(password string, salt []byte) ([]byte, error) {
	return pbkdf2.Key(sha256.New, password, salt, 600000, 32)
}
func Save(root, password string, v *Vault) error {
	if utf8.RuneCountInString(password) < 12 {
		return errors.New("Use a vault password with at least 12 characters")
	}
	salt := make([]byte, 32)
	if _, e := rand.Read(salt); e != nil {
		return e
	}
	k, e := key(password, salt)
	if e != nil {
		return e
	}
	defer clear(k)
	block, e := aes.NewCipher(k)
	if e != nil {
		return e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return e
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return e
	}
	data, e := json.Marshal(v)
	if e != nil {
		return e
	}
	defer clear(data)
	sealed := gcm.Seal(nil, nonce, data, []byte("ZKasNodeManager wallet vault v1"))
	return node.AtomicJSON(Path(root), Envelope{1, hex.EncodeToString(salt), hex.EncodeToString(nonce), hex.EncodeToString(sealed)})
}
func Load(root, password string) (*Vault, error) {
	b, e := os.ReadFile(Path(root))
	if e != nil {
		return nil, e
	}
	if len(b) > 16<<20 {
		return nil, errors.New("Wallet vault is too large")
	}
	var env Envelope
	if e = json.Unmarshal(b, &env); e != nil {
		return nil, e
	}
	if env.Version != 1 {
		return nil, errors.New("Unsupported vault version")
	}
	salt, e := hex.DecodeString(env.Salt)
	if e != nil || len(salt) != 32 {
		return nil, errors.New("Invalid vault salt")
	}
	nonce, e := hex.DecodeString(env.Nonce)
	if e != nil || len(nonce) != 12 {
		return nil, errors.New("Invalid vault nonce")
	}
	sealed, e := hex.DecodeString(env.Data)
	if e != nil {
		return nil, e
	}
	k, e := key(password, salt)
	if e != nil {
		return nil, e
	}
	defer clear(k)
	block, e := aes.NewCipher(k)
	if e != nil {
		return nil, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	plain, e := gcm.Open(nil, nonce, sealed, []byte("ZKasNodeManager wallet vault v1"))
	if e != nil {
		return nil, errors.New("Incorrect password or damaged wallet vault")
	}
	defer clear(plain)
	var v Vault
	e = json.Unmarshal(plain, &v)
	return &v, e
}
func (v *Vault) Find(id string) *Wallet {
	for i := range v.Wallets {
		if v.Wallets[i].ID == id {
			return &v.Wallets[i]
		}
	}
	return nil
}
func NewID() (string, error) { return node.NewSharingToken() }
func ParseAmount(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("Enter an amount like 1.25")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 8 {
		return 0, errors.New("Use at most 8 decimal places")
	}
	digits := parts[0] + fraction + strings.Repeat("0", 8-len(fraction))
	var n uint64
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, errors.New("Use digits and a decimal point only")
		}
		d := uint64(c - '0')
		if n > (^uint64(0)-d)/10 {
			return 0, errors.New("Amount is too large")
		}
		n = n*10 + d
	}
	if n == 0 {
		return 0, errors.New("Amount must be greater than zero")
	}
	return n, nil
}
func FormatAmount(n uint64) string { return fmt.Sprintf("%d.%08d", n/100000000, n%100000000) }
