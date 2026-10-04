package node

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"net"
	"strings"
	"testing"
)

func TestViewingScanRPCUsesAcceptedTransactionsAndReportsMissing(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := grpc.NewServer(grpc.ForceServerCodec(rawCodec{}))
	defer server.Stop()
	hash := strings.Repeat("1", 64)
	merge := strings.Repeat("2", 64)
	tx := func(id, payload string) []byte {
		return append(bytesField(8, []byte(payload)), bytesField(9, bytesField(1, []byte(id)))...)
	}
	server.RegisterService(&grpc.ServiceDesc{ServiceName: "protowire.RPC", HandlerType: (*mockRPC)(nil), Streams: []grpc.StreamDesc{{StreamName: "MessageStream", ServerStreams: true, ClientStreams: true, Handler: func(_ any, s grpc.ServerStream) error {
		var req, reply []byte
		if e := s.RecvMsg(&req); e != nil {
			return e
		}
		if q := wireOne(req, 1120); q != nil {
			reply = bytesField(1121, bytesField(1, []byte(MainnetGenesis)))
		} else if q := wireOne(req, 1122); q != nil {
			b := append(bytesField(1, []byte(hash)), bytesField(7, []byte("accepted"))...)
			b = append(b, bytesField(7, []byte("missing"))...)
			reply = bytesField(1123, bytesField(1, b))
		} else if q := wireOne(req, 1025); q != nil {
			if wireNum(q, 3) != 1 {
				return fmt.Errorf("transactions not requested")
			}
			var b []byte
			if string(wireOne(q, 1)) == hash {
				b = append(bytesField(2, tx("not-accepted", "00")), bytesField(3, bytesField(18, []byte(merge)))...)
			} else {
				b = bytesField(2, tx("accepted", "abcd"))
			}
			reply = bytesField(1026, bytesField(3, b))
		} else {
			return fmt.Errorf("unexpected RPC")
		}
		return s.SendMsg(&reply)
	}}}}, struct{}{})
	go server.Serve(listener)
	rpc, e := NewViewRPC(listener.Addr().(*net.TCPAddr).Port)
	if e != nil {
		t.Fatal(e)
	}
	defer rpc.Close()
	ctx := context.Background()
	if e = rpc.CheckGenesis(ctx); e != nil {
		t.Fatal(e)
	}
	page, e := rpc.Page(ctx, MainnetGenesis)
	if e != nil || len(page) != 1 {
		t.Fatal(e)
	}
	found, missing, e := rpc.Bundles(ctx, page[0])
	if e != nil || missing != 1 || len(found) != 1 || found["accepted"] != "abcd" {
		t.Fatal(found, missing, e)
	}
}
