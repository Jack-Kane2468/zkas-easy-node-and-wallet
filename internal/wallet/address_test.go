package wallet

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestReceiverEncodingMatchesOfficialSigner(t *testing.T) {
	b, e := os.ReadFile("testdata/receiver-vectors.json")
	if e != nil {
		t.Fatal(e)
	}
	var vectors []struct{ Raw, Address string }
	if e = json.Unmarshal(b, &vectors); e != nil {
		t.Fatal(e)
	}
	for _, v := range vectors {
		raw, e := hex.DecodeString(v.Raw)
		if e != nil {
			t.Fatal(e)
		}
		got, e := EncodeReceiver(raw)
		if e != nil || got != v.Address {
			t.Fatal(got, v.Address, e)
		}
	}
}
