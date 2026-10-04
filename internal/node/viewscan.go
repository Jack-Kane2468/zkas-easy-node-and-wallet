package node

import (
	"context"
	"encoding/hex"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protowire"
	"time"
)

const MainnetGenesis = "b63f7fe8e50402af34790265e299bb1ba63e943b91a59a670e5971b7a9e84e6f"

type ViewRPC struct{ conn *grpc.ClientConn }

func NewViewRPC(port int) (*ViewRPC, error) {
	c, e := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(rawCodec{}), grpc.MaxCallRecvMsgSize(64<<20)))
	return &ViewRPC{c}, e
}
func (v *ViewRPC) Close() {
	if v.conn != nil {
		v.conn.Close()
	}
}
func bytesField(n protowire.Number, b []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, n, protowire.BytesType), b)
}
func varField(n protowire.Number, b uint64) []byte {
	return protowire.AppendVarint(protowire.AppendTag(nil, n, protowire.VarintType), b)
}
func wireValues(b []byte, n protowire.Number) [][]byte {
	out := [][]byte{}
	eachField(b, func(k protowire.Number, t protowire.Type, x []byte) error {
		if k == n && t == protowire.BytesType {
			y, z := protowire.ConsumeBytes(x)
			if z >= 0 {
				out = append(out, y)
			}
		}
		return nil
	})
	return out
}
func wireOne(b []byte, n protowire.Number) []byte {
	v := wireValues(b, n)
	if len(v) > 0 {
		return v[0]
	}
	return nil
}
func wireNum(b []byte, n protowire.Number) uint64 {
	var v uint64
	eachField(b, func(k protowire.Number, t protowire.Type, x []byte) error {
		if k == n && t == protowire.VarintType {
			v, _ = protowire.ConsumeVarint(x)
		}
		return nil
	})
	return v
}
func (v *ViewRPC) call(ctx context.Context, request, response protowire.Number, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	s, e := v.conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, "/protowire.RPC/MessageStream")
	if e != nil {
		return nil, e
	}
	msg := bytesField(request, body)
	if e = s.SendMsg(&msg); e != nil {
		return nil, e
	}
	var raw []byte
	if e = s.RecvMsg(&raw); e != nil {
		return nil, e
	}
	if e = eachField(raw, func(protowire.Number, protowire.Type, []byte) error { return nil }); e != nil {
		return nil, e
	}
	b := wireOne(raw, response)
	if b == nil {
		return nil, fmt.Errorf("unexpected node RPC response")
	}
	if e = eachField(b, func(protowire.Number, protowire.Type, []byte) error { return nil }); e != nil {
		return nil, e
	}
	if err := wireOne(b, 1000); len(err) > 0 {
		return nil, fmt.Errorf("node RPC: %s", wireOne(err, 1))
	}
	return b, nil
}

type ViewBlock struct {
	Hash           string
	Timestamp, DAA uint64
	TxIDs          []string
}

func (v *ViewRPC) CheckGenesis(ctx context.Context) error {
	b, e := v.call(ctx, 1120, 1121, bytesField(1, []byte(MainnetGenesis)))
	if e != nil {
		return fmt.Errorf("cannot start a full scan: this node must serve mainnet history from genesis: %w", e)
	}
	if string(wireOne(b, 1)) != MainnetGenesis {
		return fmt.Errorf("node did not return the mainnet genesis checkpoint")
	}
	return nil
}
func (v *ViewRPC) Page(ctx context.Context, cursor string) ([]ViewBlock, error) {
	q := append(bytesField(1, []byte(cursor)), varField(2, 200)...)
	b, e := v.call(ctx, 1122, 1123, q)
	if e != nil {
		return nil, e
	}
	if wireNum(b, 2) != 0 {
		return nil, fmt.Errorf("chain reorganized; restart the OVK scan from genesis")
	}
	out := []ViewBlock{}
	for _, r := range wireValues(b, 1) {
		x := ViewBlock{Hash: string(wireOne(r, 1)), DAA: wireNum(r, 3), Timestamp: wireNum(r, 8)}
		for _, id := range wireValues(r, 7) {
			x.TxIDs = append(x.TxIDs, string(id))
		}
		if len(x.Hash) != 64 {
			return nil, fmt.Errorf("invalid scan cursor")
		}
		out = append(out, x)
	}
	return out, nil
}
func (v *ViewRPC) block(ctx context.Context, hash string) ([]byte, error) {
	q := append(bytesField(1, []byte(hash)), varField(3, 1)...)
	b, e := v.call(ctx, 1025, 1026, q)
	if e != nil {
		return nil, e
	}
	return wireOne(b, 3), nil
}

// Only bundles with transaction IDs in the canonical accepted stream are recovered.
func (v *ViewRPC) Bundles(ctx context.Context, block ViewBlock) (map[string]string, int, error) {
	found := map[string]string{}
	wanted := map[string]bool{}
	for _, id := range block.TxIDs {
		wanted[id] = true
	}
	if len(wanted) == 0 {
		return found, 0, nil
	}
	first, e := v.block(ctx, block.Hash)
	if e != nil {
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		return found, len(wanted), nil
	}
	find := func(b []byte) {
		for _, tx := range wireValues(b, 2) {
			id := string(wireOne(wireOne(tx, 9), 1))
			if wanted[id] {
				payload := string(wireOne(tx, 8))
				if payload != "" {
					if _, e := hex.DecodeString(payload); e == nil {
						found[id] = payload
					}
				}
			}
		}
	}
	find(first)
	verbose := wireOne(first, 3)
	candidates := append(wireValues(verbose, 18), wireValues(verbose, 19)...)
	for _, hash := range candidates {
		if len(found) == len(wanted) {
			break
		}
		b, e := v.block(ctx, string(hash))
		if e != nil {
			if ctx.Err() != nil {
				return nil, 0, ctx.Err()
			}
			continue
		}
		find(b)
	}
	return found, len(wanted) - len(found), nil
}
