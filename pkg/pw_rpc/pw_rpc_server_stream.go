package pw_rpc

import (
	"context"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ServerStream interface {
	grpc.ServerStream
}

type serverStream struct {
	s    Stream
	conn Conn
}

// Context implements grpc.ServerStream.
func (ss *serverStream) Context() context.Context {
	return ss.s.Context()
}

// SendHeader implements grpc.ServerStream.
func (ss *serverStream) SendHeader(metadata.MD) error {
	panic("unimplemented")
}

// SetHeader implements grpc.ServerStream.
func (ss *serverStream) SetHeader(metadata.MD) error {
	panic("unimplemented")
}

// SetTrailer implements grpc.ServerStream.
func (ss *serverStream) SetTrailer(metadata.MD) {
	panic("unimplemented")
}

func (ss *serverStream) SendMsg(m any) error {
	return ss.s.Send(m, pb.PacketType_SERVER_STREAM)
}

func (ss *serverStream) RecvMsg(m any) error {
	return ss.s.Recv(m)
}

func (ss *serverStream) Close() {
	//ss.s.Send([]byte{}, pb.PacketType_RESPONSE)

	ss.s.Close()
}

func NewServerStream(ctx context.Context, desc *grpc.StreamDesc, conn Conn, method string, opts ...grpc.CallOption) (ServerStream, error) {
	s, err := NewStream(ctx, desc, conn, method, opts...)
	if err != nil {
		return nil, err
	}

	stream := &serverStream{
		s:    s,
		conn: conn,
	}

	return stream, nil
}
