package node

import (
	"context"
	"errors"
	"google.golang.org/protobuf/encoding/protowire"
	"time"
)

type SyncProgress struct{ Blocks, Headers, DAA uint64 }

// GetBlockDagInfo fields from official zkas-v1.0.11 rpc/{messages,rpc}.proto.
// Use only after Get on this goroutine; reuse the same stream to avoid log spam.
func (m *InfoMonitor) Progress(ctx context.Context) (SyncProgress, error) {
	if m.stream == nil {
		return SyncProgress{}, errors.New("RPC not connected")
	}
	q, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	type result struct {
		p SyncProgress
		e error
	}
	done := make(chan result, 1)
	stream := m.stream
	go func() {
		req := protowire.AppendTag(nil, 1035, protowire.BytesType)
		req = protowire.AppendBytes(req, nil)
		if e := stream.SendMsg(&req); e != nil {
			done <- result{e: e}
			return
		}
		var b []byte
		if e := stream.RecvMsg(&b); e != nil {
			done <- result{e: e}
			return
		}
		p, e := DecodeProgress(b)
		done <- result{p, e}
	}()
	select {
	case r := <-done:
		if r.e != nil {
			m.Close()
		}
		return r.p, r.e
	case <-q.Done():
		m.Close()
		<-done
		return SyncProgress{}, q.Err()
	}
}
func DecodeProgress(b []byte) (SyncProgress, error) {
	var p SyncProgress
	found := false
	err := eachField(b, func(n protowire.Number, t protowire.Type, v []byte) error {
		if n != 1036 {
			return nil
		}
		if t != protowire.BytesType {
			return errors.New("invalid DAG response")
		}
		body, k := protowire.ConsumeBytes(v)
		if k < 0 {
			return errors.New("invalid DAG body")
		}
		found = true
		return eachField(body, func(n protowire.Number, t protowire.Type, v []byte) error {
			if n == 1000 {
				return errors.New("DAG RPC error")
			}
			if n != 2 && n != 3 && n != 9 {
				return nil
			}
			if t != protowire.VarintType {
				return errors.New("invalid DAG count")
			}
			x, k := protowire.ConsumeVarint(v)
			if k < 0 {
				return errors.New("invalid DAG number")
			}
			switch n {
			case 2:
				p.Blocks = x
			case 3:
				p.Headers = x
			case 9:
				p.DAA = x
			}
			return nil
		})
	})
	if err == nil && !found {
		err = errors.New("DAG response missing")
	}
	return p, err
}
