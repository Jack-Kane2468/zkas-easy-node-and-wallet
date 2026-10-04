package wallet

import (
	"encoding/hex"
	"fmt"
	"github.com/dchest/blake2b"
	"strings"
)

func CleanHex(s string, size int) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	b, e := hex.DecodeString(s)
	if e != nil || len(b) != size {
		return "", fmt.Errorf("Expected %d hexadecimal characters", size*2)
	}
	return s, nil
}

// Orchard external OVK: PRF^expand(rivk, 0x82 || ak || nk)[32..64].
// Matches orchard FullViewingKey::derive_dk_ovk and zcash_spec::PrfExpand.
func OVK(fvk string) (string, error) {
	s, e := CleanHex(fvk, 96)
	if e != nil {
		return "", e
	}
	b, _ := hex.DecodeString(s)
	h, e := blake2b.New(&blake2b.Config{Size: 64, Person: []byte("Zcash_ExpandSeed")})
	if e != nil {
		return "", e
	}
	h.Write(b[64:])
	h.Write([]byte{0x82})
	h.Write(b[:64])
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[32:]), nil
}
