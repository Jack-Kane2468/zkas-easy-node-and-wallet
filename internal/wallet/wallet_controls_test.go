package wallet

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestReportedNoteCount(t *testing.T) {
	for _, tc := range []struct{ body, want string }{{`{"has_wallet":true,"synced":true,"note_count":38}`, "Notes: 38"}, {`{"has_wallet":true,"note_count":0}`, "Notes: 0"}, {`{"has_wallet":true}`, "Notes: Not reported"}, {`{"has_wallet":true,"note_count":null}`, "Notes: Not reported"}} {
		var s Status
		if e := json.Unmarshal([]byte(tc.body), &s); e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(s.Display(), tc.want) {
			t.Fatal(s.Display())
		}
	}
}
func TestPasswordChangePreservesCompleteVault(t *testing.T) {
	root := t.TempDir()
	old, new := "old public test password", "new public test password"
	original := &Vault{Wallets: []Wallet{{ID: "test", Name: "Wallet", Seed: "public fixture", Secret: "public recovery fixture", Token: "public token", FVK: "public fvk", Address: "address", Account: 2, NextIndex: 3, Addresses: []Address{{Index: 2, Address: "other"}}, Receipts: []Receipt{{TxID: "public tx", State: "broadcast", Memo: "memo"}}}}}
	if e := Save(root, old, original); e != nil {
		t.Fatal(e)
	}
	unlocked, e := Load(root, old)
	if e != nil {
		t.Fatal(e)
	}
	if e = Save(root, new, unlocked); e != nil {
		t.Fatal(e)
	}
	if _, e = Load(root, old); e == nil {
		t.Fatal("old password still accepted")
	}
	restored, e := Load(root, new)
	if e != nil || !reflect.DeepEqual(original, restored) {
		t.Fatal("password change altered wallet data", e)
	}
	if e = Save(root, "short", restored); e == nil {
		t.Fatal("weak password accepted")
	}
	restored, e = Load(root, new)
	if e != nil || !reflect.DeepEqual(original, restored) {
		t.Fatal("failed password change damaged vault", e)
	}
}
