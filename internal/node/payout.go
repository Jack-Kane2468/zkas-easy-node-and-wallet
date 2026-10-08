package node

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protowire"
	"time"
)

// Construct, never submit, a block template to validate the payout and sync state.
func ValidatePayout(ctx context.Context, port int, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, e := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(rawCodec{}), grpc.MaxCallRecvMsgSize(16<<20)))
	if e != nil {
		return e
	}
	defer c.Close()
	s, e := c.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, "/protowire.RPC/MessageStream")
	if e != nil {
		return e
	}
	body := protowire.AppendTag(nil, 1, protowire.BytesType)
	body = protowire.AppendString(body, address)
	req := protowire.AppendTag(nil, 101, protowire.VarintType)
	req = protowire.AppendVarint(req, 1)
	req = protowire.AppendTag(req, 1005, protowire.BytesType)
	req = protowire.AppendBytes(req, body)
	if e = s.SendMsg(&req); e != nil {
		return e
	}
	var response []byte
	if e = s.RecvMsg(&response); e != nil {
		return e
	}
	return decodePayoutTemplate(response)
}
func decodePayoutTemplate(response []byte) error {
	found, synced, block := false, false, false
	e := eachField(response, func(n protowire.Number, t protowire.Type, v []byte) error {
		if n != 1006 {
			return nil
		}
		if t != protowire.BytesType {
			return errors.New("Invalid template response")
		}
		b, k := protowire.ConsumeBytes(v)
		if k < 0 {
			return errors.New("Invalid template payload")
		}
		found = true
		return eachField(b, func(n protowire.Number, t protowire.Type, v []byte) error {
			switch n {
			case 1000:
				if t != protowire.BytesType {
					return errors.New("Invalid RPC error")
				}
				b, k := protowire.ConsumeBytes(v)
				if k < 0 {
					return errors.New("Invalid RPC error")
				}
				msg := "Node rejected the payout address or template request"
				eachField(b, func(n protowire.Number, t protowire.Type, v []byte) error {
					if n == 1 && t == protowire.BytesType {
						s, k := protowire.ConsumeBytes(v)
						if k >= 0 {
							msg = string(s)
						}
					}
					return nil
				})
				return errors.New(msg)
			case 2:
				if t != protowire.VarintType {
					return errors.New("Invalid sync status")
				}
				x, k := protowire.ConsumeVarint(v)
				synced = k > 0 && x != 0
			case 3:
				if t == protowire.BytesType {
					b, k := protowire.ConsumeBytes(v)
					block = k >= 0 && len(b) > 0
				}
			}
			return nil
		})
	})
	if e != nil {
		return e
	}
	if !found || !block {
		return errors.New("Node did not return a mining template")
	}
	if !synced {
		return errors.New("Node is still syncing. Wait until synced before mining")
	}
	return nil
}
