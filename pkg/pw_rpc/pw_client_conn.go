package pw_rpc

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
)

type ClientConn struct {
	endpoint string
	port     int
	conn     net.Conn
}

func NewClientConn(endpoint string, port int) *ClientConn {
	return &ClientConn{
		endpoint: endpoint,
		port:     port,
	}
}

func (cc *ClientConn) connect() (err error) {
	if cc.conn == nil {
		cc.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", cc.endpoint, cc.port))
		if err != nil {
			return err
		}
	}

	return nil
}

func (sc *ClientConn) Close() {
	if sc != nil && sc.conn != nil {
		sc.conn.Close()
	}
}

func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	cc.connect()

	cs, err := NewClientStream(
		ctx,
		&grpc.StreamDesc{
			ServerStreams: false,
			ClientStreams: false,
		},
		cc,
		method,
		opts...,
	)
	if err != nil {
		return err
	}
	if err := cs.SendMsg(args); err != nil {
		return err
	}

	return cs.RecvMsg(reply)
}

func (cc *ClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return NewClientStream(ctx, desc, cc, method, opts...)
}
