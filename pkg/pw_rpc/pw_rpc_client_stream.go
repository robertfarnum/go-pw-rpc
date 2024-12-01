package pw_rpc

import (
	"context"
	"errors"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	k65599HashConstant = uint32(65599)
	kHDLCChannel       = 1
)

var (
	ErrBadAddress = errors.New("bad address")
)

type ClientStream interface {
	grpc.ClientStream
	GetStream() Stream
}

type clientStream struct {
	s    Stream
	conn Conn
}

// CloseSend implements Stream.
func (cs *clientStream) CloseSend() error {
	cs.s.Close()

	return nil
}

// Context implements Stream.
func (cs *clientStream) Context() context.Context {
	return cs.s.Context()
}

// Header implements Stream.
func (cs *clientStream) Header() (metadata.MD, error) {
	panic("unimplemented")
}

// Trailer implements Stream.
func (cs *clientStream) Trailer() metadata.MD {
	panic("unimplemented")
}

func (cs *clientStream) SendMsg(m any) error {
	return cs.s.Send(m, pb.PacketType_CLIENT_STREAM)
}

func (cs *clientStream) RecvMsg(m any) error {
	return cs.s.Recv(m)
}

func (cs *clientStream) Key() StreamKey {
	return cs.s.Key()
}

func (cs *clientStream) Close() {
	//cs.s.Send([]byte{}, pb.PacketType_CLIENT_REQUEST_COMPLETION)

	cs.s.Close()
}

func (cs *clientStream) GetStream() Stream {
	return cs.s
}

func NewClientStream(ctx context.Context, desc *grpc.StreamDesc, conn Conn, method string, opts ...grpc.CallOption) (ClientStream, error) {
	s, err := NewStream(ctx, desc, conn, method, opts...)
	if err != nil {
		return nil, err
	}

	return &clientStream{
		s:    s,
		conn: conn,
	}, nil
}
