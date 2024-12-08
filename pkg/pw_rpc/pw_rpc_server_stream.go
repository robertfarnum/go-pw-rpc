package pw_rpc

import (
	"context"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ServerStream interface {
	grpc.ServerStream
	GetStream() Stream
}

type serverStream struct {
	s Stream
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
	return ss.s.Send(m, pb.StatusCode_OK, pb.PacketType_SERVER_STREAM)
}

func (ss *serverStream) RecvMsg(m any) error {
	_, _, err := ss.s.Recv(m)

	return err
}

func (ss *serverStream) Close() {
	ss.s.Send([]byte{}, pb.StatusCode_OK, pb.PacketType_RESPONSE)

	ss.s.Close()
}

func (ss *serverStream) GetStream() Stream {
	return ss.s
}

func NewServerStream(ctx context.Context, desc *grpc.StreamDesc, server Server, method string, opts ...grpc.CallOption) (ServerStream, error) {
	stream, err := NewStream(ctx, desc, server.GetConn(), method, opts...)
	if err != nil {
		return nil, err
	}

	serverStream := &serverStream{
		s: stream,
	}

	return serverStream, nil
}
