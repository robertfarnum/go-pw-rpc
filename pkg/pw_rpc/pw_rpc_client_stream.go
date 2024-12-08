package pw_rpc

import (
	"context"
	"fmt"
	"io"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	k65599HashConstant = uint32(65599)
	kHDLCChannel       = 1
)

type ClientStream interface {
	grpc.ClientStream
	GetStream() Stream
}

type clientStream struct {
	s         Stream
	desc      *grpc.StreamDesc
	c         Client
	firstSend bool
	closeSend bool
}

// CloseSend implements Stream.
func (cs *clientStream) CloseSend() error {
	if cs.closeSend {
		return fmt.Errorf("CloseSend called multiple times")
	}

	cs.closeSend = true

	if cs.desc != nil && cs.desc.ClientStreams {
		return cs.s.Send(&emptypb.Empty{}, pb.StatusCode_OK, pb.PacketType_CLIENT_REQUEST_COMPLETION)
	}

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
	if m == nil {
		err := cs.s.Send(&emptypb.Empty{}, pb.StatusCode_CANCELLED, pb.PacketType_CLIENT_ERROR)

		cs.c.CloseStream(cs.s)

		return err
	}

	if cs.closeSend {
		return fmt.Errorf("SendMsg called after CloseSend")
	}

	if cs.desc != nil && cs.desc.ClientStreams {
		if !cs.firstSend {
			cs.firstSend = true
			return cs.s.Send(m, pb.StatusCode_OK, pb.PacketType_REQUEST)
		} else {
			return cs.s.Send(m, pb.StatusCode_OK, pb.PacketType_CLIENT_STREAM)
		}
	}

	if cs.desc != nil && cs.desc.ServerStreams {
		err := cs.s.Send(m, pb.StatusCode_OK, pb.PacketType_REQUEST)
		if err != nil {
			return err
		}

		return nil
	}

	return cs.s.Send(m, pb.StatusCode_OK, pb.PacketType_REQUEST)
}

func (cs *clientStream) RecvMsg(m any) error {
	if m == nil {
		err := cs.s.Send(&emptypb.Empty{}, pb.StatusCode_CANCELLED, pb.PacketType_CLIENT_ERROR)
		cs.c.CloseStream(cs.s)
		return err
	}

	if cs.desc != nil && cs.desc.ServerStreams {
		pt, _, err := cs.s.Recv(m)
		if err != nil {
			cs.c.CloseStream(cs.s)
			return err
		}
		switch pt {
		case pb.PacketType_RESPONSE:
			cs.c.CloseStream(cs.s)
			return io.EOF
		case pb.PacketType_SERVER_STREAM:
			return nil
		default:
			cs.c.CloseStream(cs.s)
			return fmt.Errorf("unexpected packet type: %s", pt)
		}
	}

	_, _, err := cs.s.Recv(m)

	return err
}

func (cs *clientStream) GetStream() Stream {
	return cs.s
}

func NewClientStream(ctx context.Context, desc *grpc.StreamDesc, c Client, method string, opts ...grpc.CallOption) (ClientStream, error) {
	s, err := NewStream(ctx, desc, c.GetConn(), method, opts...)
	if err != nil {
		return nil, err
	}

	return &clientStream{
		s:    s,
		desc: desc,
		c:    c,
	}, nil
}
