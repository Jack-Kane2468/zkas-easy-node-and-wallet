package node

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func WalletCheckpoint(root, token string) (string, error) {
	if token == "" || token == "." || strings.ContainsAny(token, `/\:`) {
		return "", fmt.Errorf("invalid wallet token")
	}
	f, e := os.Open(filepath.Join(root, "wallets", token+".scan"))
	if e != nil {
		return "", e
	}
	defer f.Close()
	var b [77]byte
	if _, e = io.ReadFull(f, b[:]); e != nil {
		return "", e
	}
	if string(b[:4]) != "FCWS" || (b[4] != 3 && b[4] != 4) || hex.EncodeToString(b[5:37]) != MainnetGenesis {
		return "", fmt.Errorf("unsupported checkpoint header")
	}
	return hex.EncodeToString(b[37:69]), nil
}

type CheckpointProbe struct {
	mu           sync.Mutex
	key, message string
	checked      time.Time
}

func (p *CheckpointProbe) Check(ctx context.Context, root, token string, port int) string {
	cursor, e := WalletCheckpoint(root, token)
	if e != nil {
		return ""
	}
	key := fmt.Sprintf("%s:%d:%s:%s", root, port, token, cursor)
	p.mu.Lock()
	defer p.mu.Unlock()
	if key == p.key && time.Since(p.checked) < 30*time.Second {
		return p.message
	}
	p.key, p.checked, p.message = key, time.Now(), ""
	v, e := NewViewRPC(port)
	if e != nil {
		return ""
	}
	defer v.Close()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	b, e := v.call(ctx, 1120, 1121, bytesField(1, []byte(cursor)))
	if e != nil {
		if strings.Contains(e.Error(), "lies in peer-supplied history") {
			p.message = "Cannot verify this wallet's saved checkpoint: it lies in peer-supplied history. Use Rescan this wallet to rebuild from the beginning."
		}
	} else if string(wireOne(b, 1)) != cursor {
		p.message = "The node did not return this wallet's saved checkpoint. It may still be syncing or lack the required history. Balance is not verified."
	}
	return p.message
}
