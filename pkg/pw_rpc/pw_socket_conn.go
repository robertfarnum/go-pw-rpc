package pw_rpc

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
)

type SocketConn struct {
	endpoint string
	port     int
	conn     net.Conn
}

func NewSocketConn(endpoint string, port int) *SocketConn {
	return &SocketConn{
		endpoint: endpoint,
		port:     port,
	}
}

func (sc *SocketConn) Connect() (err error) {
	// Connect to the server
	sc.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", sc.endpoint, sc.port))
	if err != nil {
		fmt.Println("Error connecting:", err)
		return err
	}

	return nil
}

func (sc *SocketConn) Close() {
	if sc != nil && sc.conn != nil {
		sc.conn.Close()
	}
}

func (sc *SocketConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	cs, err := NewClientStream(
		ctx,
		&grpc.StreamDesc{
			ServerStreams: false,
			ClientStreams: false,
		},
		sc,
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

func (sc *SocketConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return NewClientStream(ctx, desc, sc, method, opts...)
}
