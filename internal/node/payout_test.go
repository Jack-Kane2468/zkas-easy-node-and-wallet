package node

import (
	"google.golang.org/protobuf/encoding/protowire"
	"testing"
)

func templateResponse(sync bool, errText string) []byte {
	b := protowire.AppendTag(nil, 2, protowire.VarintType)
	v := uint64(0)
	if sync {
		v = 1
	}
	b = protowire.AppendVarint(b, v)
	b = protowire.AppendTag(b, 3, protowire.BytesType)
	b = protowire.AppendBytes(b, []byte{8, 1})
	if errText != "" {
		e := protowire.AppendTag(nil, 1, protowire.BytesType)
		e = protowire.AppendString(e, errText)
		b = protowire.AppendTag(b, 1000, protowire.BytesType)
		b = protowire.AppendBytes(b, e)
	}
	out := protowire.AppendTag(nil, 1006, protowire.BytesType)
	return protowire.AppendBytes(out, b)
}
func TestPayoutTemplateValidation(t *testing.T) {
	if e := decodePayoutTemplate(templateResponse(true, "")); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{nil, {255}, templateResponse(false, ""), templateResponse(true, "bad address checksum")} {
		if decodePayoutTemplate(b) == nil {
			t.Fatal("accepted invalid template")
		}
	}
}
