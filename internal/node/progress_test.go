package node

import (
	"google.golang.org/protobuf/encoding/protowire"
	"testing"
)

func TestProgressDecode(t *testing.T) {
	var b []byte
	for n, v := range map[protowire.Number]uint64{2: 12, 3: 34, 9: 56} {
		b = protowire.AppendTag(b, n, protowire.VarintType)
		b = protowire.AppendVarint(b, v)
	}
	env := protowire.AppendTag(nil, 1036, protowire.BytesType)
	env = protowire.AppendBytes(env, b)
	p, e := DecodeProgress(env)
	if e != nil || p != (SyncProgress{12, 34, 56}) {
		t.Fatal(p, e)
	}
	if _, e := DecodeProgress(nil); e == nil {
		t.Fatal("missing accepted")
	}
	b = protowire.AppendTag(nil, 1000, protowire.BytesType)
	b = protowire.AppendBytes(b, nil)
	env = protowire.AppendTag(nil, 1036, protowire.BytesType)
	env = protowire.AppendBytes(env, b)
	if _, e := DecodeProgress(env); e == nil {
		t.Fatal("RPC error accepted")
	}
}
