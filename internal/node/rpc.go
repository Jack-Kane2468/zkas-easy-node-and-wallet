package node

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protowire"
	"time"
)

// Minimal read-only codec for the upstream protowire.RPC/MessageStream schema.
// Field IDs are pinned to zkas-v1.0.9 rpc/grpc/core/proto/{messages,rpc}.proto.
// Unknown fields are skipped for forward compatibility. No signing or unsafe RPCs.
type rawCodec struct{}

func (rawCodec) Name() string { return "proto" }
func (rawCodec) Marshal(v any) ([]byte, error) {
	p, ok := v.(*[]byte)
	if !ok {
		return nil, errors.New("invalid message")
	}
	return *p, nil
}
func (rawCodec) Unmarshal(b []byte, v any) error {
	p, ok := v.(*[]byte)
	if !ok {
		return errors.New("invalid message")
	}
	*p = append((*p)[:0], b...)
	return nil
}

type NodeInfo struct {
	Synced  bool
	Version string
	Indexed bool
}

// InfoMonitor is owned by one polling goroutine and reuses one RPC stream.
// Reconnecting every poll caused the noisy connect/disconnect pairs in node logs.
type InfoMonitor struct {
	client *grpc.ClientConn
	stream grpc.ClientStream
	cancel context.CancelFunc
	port   int
}

func (m *InfoMonitor) Close() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.client != nil {
		m.client.Close()
	}
	m.client = nil
	m.stream = nil
	m.cancel = nil
}
func GetInfo(ctx context.Context, port int) (NodeInfo, error) {
	m := &InfoMonitor{}
	defer m.Close()
	return m.Get(ctx, port)
}
func (m *InfoMonitor) Get(ctx context.Context, port int) (NodeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if m.client != nil && m.port != port {
		m.Close()
	}
	if m.client == nil {
		client, e := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(rawCodec{})))
		if e != nil {
			return NodeInfo{}, e
		}
		m.client = client
		m.port = port
		streamContext, stop := context.WithCancel(context.Background())
		m.cancel = stop
		stream, e := client.NewStream(streamContext, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, "/protowire.RPC/MessageStream")
		if e != nil {
			m.Close()
			return NodeInfo{}, e
		}
		m.stream = stream
	}
	type result struct {
		info NodeInfo
		err  error
	}
	done := make(chan result, 1)
	stream := m.stream
	go func() {
		request := protowire.AppendTag(nil, 101, protowire.VarintType)
		request = protowire.AppendVarint(request, 1)
		request = protowire.AppendTag(request, 1063, protowire.BytesType)
		request = protowire.AppendBytes(request, nil)
		if e := stream.SendMsg(&request); e != nil {
			done <- result{err: e}
			return
		}
		var response []byte
		if e := stream.RecvMsg(&response); e != nil {
			done <- result{err: e}
			return
		}
		info, e := DecodeInfo(response)
		done <- result{info, e}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			m.Close()
		}
		return r.info, r.err
	case <-ctx.Done():
		m.Close()
		<-done
		return NodeInfo{}, ctx.Err()
	}
}
func eachField(b []byte, fn func(protowire.Number, protowire.Type, []byte) error) error {
	for len(b) > 0 {
		n, t, k := protowire.ConsumeTag(b)
		if k < 0 {
			return errors.New("invalid RPC tag")
		}
		b = b[k:]
		l := protowire.ConsumeFieldValue(n, t, b)
		if l < 0 {
			return errors.New("invalid RPC value")
		}
		if e := fn(n, t, b[:l]); e != nil {
			return e
		}
		b = b[l:]
	}
	return nil
}
func DecodeInfo(b []byte) (NodeInfo, error) {
	var info NodeInfo
	found := false
	e := eachField(b, func(n protowire.Number, t protowire.Type, v []byte) error {
		if n != 1064 {
			return nil
		}
		if t != protowire.BytesType {
			return errors.New("invalid GetInfo response")
		}
		body, k := protowire.ConsumeBytes(v)
		if k < 0 {
			return errors.New("invalid GetInfo payload")
		}
		found = true
		return eachField(body, func(n protowire.Number, t protowire.Type, v []byte) error {
			switch n {
			case 3:
				if t != protowire.BytesType {
					return errors.New("invalid version field")
				}
				s, k := protowire.ConsumeBytes(v)
				if k < 0 {
					return errors.New("invalid version")
				}
				info.Version = string(s)
			case 4, 5:
				if t != protowire.VarintType {
					return errors.New("invalid status field")
				}
				x, k := binary.Uvarint(v)
				if k <= 0 {
					return errors.New("invalid bool")
				}
				if n == 4 {
					info.Indexed = x != 0
				} else {
					info.Synced = x != 0
				}
			case 1000:
				return errors.New("Node returned an RPC error")
			}
			return nil
		})
	})
	if e == nil && !found {
		e = errors.New("No GetInfo response")
	}
	return info, e
}
