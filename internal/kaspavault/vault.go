package kaspavault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"unicode/utf8"
	"zkas-node-manager/internal/node"
)

type Wallet struct {
	Receipts                       []string
	ID, Name, Password, Passphrase string
	Backup                         []byte
}
type Vault struct{ Wallets []Wallet }
type Envelope struct {
	Version int    `json:"version"`
	Salt    string `json:"salt"`
	Nonce   string `json:"nonce"`
	Data    string `json:"data"`
}

func Path(root string) string { return filepath.Join(root, "kaspa-vault", "vault.json") }
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
	if len(data) > 31<<20 {
		return errors.New("Kaspa vault exceeds its backup size limit")
	}
	sealed := gcm.Seal(nil, nonce, data, []byte("ZKasNodeManager Kaspa vault v1"))
	return node.AtomicJSON(Path(root), Envelope{1, hex.EncodeToString(salt), hex.EncodeToString(nonce), hex.EncodeToString(sealed)})
}
func Load(root, password string) (*Vault, error) {
	return LoadFile(Path(root), password)
}
func LoadFile(path, password string) (*Vault, error) {
	st, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	if st.Size() > 64<<20 {
		return nil, errors.New("Kaspa vault exceeds 64 MB")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	if len(b) > 64<<20 {
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
	plain, e := gcm.Open(nil, nonce, sealed, []byte("ZKasNodeManager Kaspa vault v1"))
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
