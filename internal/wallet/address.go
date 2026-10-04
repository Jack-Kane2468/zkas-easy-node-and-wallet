package wallet

import (
	"fmt"
	"strings"
)

// ZKas release address encoding: version 9, 43-byte Orchard receiver, cashaddr checksum.
func EncodeReceiver(raw []byte) (string, error) {
	if len(raw) != 43 {
		return "", fmt.Errorf("invalid Orchard receiver length")
	}
	data := append([]byte{9}, raw...)
	groups := []byte{}
	var acc uint32
	bits := uint(0)
	for _, b := range data {
		acc = acc<<8 | uint32(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			groups = append(groups, byte(acc>>bits)&31)
		}
	}
	if bits > 0 {
		groups = append(groups, byte(acc<<(5-bits))&31)
	}
	c := uint64(1)
	step := func(d byte) {
		top := c >> 35
		c = ((c & 0x07ffffffff) << 5) ^ uint64(d)
		for i, g := range []uint64{0x98f2bc8e61, 0x79b76d99e2, 0xf33e5fb3c4, 0xae2eabe2a8, 0x1e4f43e470} {
			if top&(1<<i) != 0 {
				c ^= g
			}
		}
	}
	for _, b := range []byte("zkas") {
		step(b & 31)
	}
	step(0)
	for _, b := range groups {
		step(b)
	}
	for i := 0; i < 8; i++ {
		step(0)
	}
	c ^= 1
	const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	var out strings.Builder
	out.WriteString("zkas:")
	for _, b := range groups {
		out.WriteByte(charset[b])
	}
	for i := 7; i >= 0; i-- {
		out.WriteByte(charset[(c>>uint(i*5))&31])
	}
	return out.String(), nil
}
