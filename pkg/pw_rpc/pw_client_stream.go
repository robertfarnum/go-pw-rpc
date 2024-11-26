package pw_rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
)

const (
	k65599HashConstant = uint32(65599)
	kHDLCChannel       = 1
)

var (
	ErrBadAddress = errors.New("bad address")
)

type ClientStreamKey struct {
	serviceId uint32
	methodId  uint32
}
type clientStream struct {
	desc   *grpc.StreamDesc
	method string
	key    ClientStreamKey
	opts   []grpc.CallOption
	cc     *ClientConn
	ch     chan (*pb.RpcPacket)
	ctx    context.Context
	cancel context.CancelFunc
}

func newClientStream(ctx context.Context, desc *grpc.StreamDesc, cc grpc.ClientConnInterface, method string, opts ...grpc.CallOption) (*clientStream, error) {
	clientConn, ok := cc.(*ClientConn)
	if !ok {
		return nil, fmt.Errorf("failed to cast *ClientConn")
	}

	methodParts := strings.Split(method, "/")
	if len(methodParts) != 3 {
		return nil, fmt.Errorf("invalid full method name")
	}

	key := ClientStreamKey{
		serviceId: hash(methodParts[1]),
		methodId:  hash(methodParts[2]),
	}

	ctx, cancel := context.WithCancel(ctx)

	cs := &clientStream{
		desc:   desc,
		cc:     clientConn,
		method: method,
		key:    key,
		opts:   opts,
		ch:     make(chan *pb.RpcPacket),
		ctx:    ctx,
		cancel: cancel,
	}

	return cs, nil

}

func (cs *clientStream) Key() ClientStreamKey {
	return cs.key
}

func (cs *clientStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (cs *clientStream) Trailer() metadata.MD {
	return nil
}

func (cs *clientStream) CloseSend() error {
	return nil
}

func (cs *clientStream) Context() context.Context {
	return cs.ctx
}

func (cs *clientStream) SendMsg(m any) error {
	payload, ok := m.(protoreflect.ProtoMessage)
	if !ok {
		return fmt.Errorf("message not a ProtoMessage")
	}
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return err
	}

	packetType := pb.PacketType_REQUEST

	if cs.desc.ClientStreams {
		err = cs.send([]byte{}, packetType)
		if err != nil {
			return err
		}
		packetType = pb.PacketType_CLIENT_STREAM
	}

	return cs.send(payloadBytes, packetType)
}

func (cs *clientStream) RecvMsg(m any) error {
	result, ok := m.(protoreflect.ProtoMessage)
	if !ok {
		return fmt.Errorf("message not a ProtoMessage")
	}

	select {
	case <-cs.ctx.Done():
		return fmt.Errorf("cancel")
	case packet, ok := <-cs.ch:
		if !ok || packet.ChannelId != kHDLCChannel || packet.ServiceId != cs.key.serviceId || packet.MethodId != cs.key.methodId {
			return fmt.Errorf("invalid packet received")
		}

		err := proto.Unmarshal(packet.Payload, result)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cs *clientStream) recv(packet *pb.RpcPacket) {
	cs.ch <- packet
}

func (cs *clientStream) send(payloadBytes []byte, packetType pb.PacketType) error {

	packet := &pb.RpcPacket{
		Type:      packetType,
		ChannelId: kHDLCChannel,
		ServiceId: cs.key.serviceId,
		MethodId:  cs.key.methodId,
		Payload:   payloadBytes,
	}
	buf, err := proto.Marshal(packet)
	if err != nil {
		return err
	}

	err = cs.cc.Send(buf)
	if err != nil {
		return err
	}

	return nil
}

func hash(s string) uint32 {
	hash := uint32(len(s))
	coefficient := k65599HashConstant
	for _, ch := range s {
		hash += coefficient * uint32(ch)
		coefficient *= k65599HashConstant
	}

	return hash
}
