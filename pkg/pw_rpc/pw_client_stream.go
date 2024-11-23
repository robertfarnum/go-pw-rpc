package pw_rpc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/robertfarnum/go_pw_rpc/pkg/pw_hdlc"
	"github.com/robertfarnum/go_pw_rpc/pkg/pw_rpc/pb"
)

const (
	k65599HashConstant = uint32(65599)
)

var (
	kDefaultRpcAddress = 'R'
	kDefaultLogAddress = 1

	ErrBadAddress = errors.New("bad address")
)

type ClientStream struct {
	desc      *grpc.StreamDesc
	method    string
	opts      []grpc.CallOption
	cc        *ClientConn
	firstSend bool
}

func NewClientStream(ctx context.Context, desc *grpc.StreamDesc, cc grpc.ClientConnInterface, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	clientConn, ok := cc.(*ClientConn)
	if !ok {
		return nil, fmt.Errorf("failed to cast *ClientConn")
	}

	cs := &ClientStream{
		desc:      desc,
		cc:        clientConn,
		method:    method,
		opts:      opts,
		firstSend: true,
	}

	return cs, nil

}

func (cs *ClientStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (cs *ClientStream) Trailer() metadata.MD {
	return nil
}

func (cs *ClientStream) CloseSend() error {
	return nil
}

func (cs *ClientStream) Context() context.Context {
	return nil
}

func (cs *ClientStream) send(payloadBytes []byte, packetType pb.PacketType) error {
	methodParts := strings.Split(cs.method, "/")
	if len(methodParts) != 3 {
		return fmt.Errorf("invalid full method name")
	}

	serviceId := hash(methodParts[1])
	methodId := hash(methodParts[2])

	packet := &pb.RpcPacket{
		Type:      packetType,
		ChannelId: 1,
		ServiceId: serviceId,
		MethodId:  methodId,
		Payload:   payloadBytes,
	}
	request, err := proto.Marshal(packet)
	if err != nil {
		return err
	}

	encoder := pw_hdlc.NewEncoder(cs.cc.conn, uint64(kDefaultRpcAddress))
	_, err = encoder.WritePayload(request)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ClientStream) SendMsg(m any) error {
	if cs.firstSend {
		cs.send([]byte{}, pb.PacketType_REQUEST)
		cs.firstSend = false
	}

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
		packetType = pb.PacketType_CLIENT_STREAM
	}

	return cs.send(payloadBytes, packetType)
}

func (cs *ClientStream) RecvMsg(m any) error {
	result, ok := m.(protoreflect.ProtoMessage)
	if !ok {
		return fmt.Errorf("message not a ProtoMessage")
	}

	decoder := pw_hdlc.NewDecoder(cs.cc.conn, uint64(kDefaultRpcAddress))
	frame, err := decoder.Process()
	if err != nil {
		return err
	}

	switch frame.Address() {
	case uint64(kDefaultRpcAddress):
		packet := &pb.RpcPacket{}
		err = proto.Unmarshal(frame.Payload(), packet)
		if err != nil {
			return err
		}

		err = proto.Unmarshal(packet.Payload, result)
		if err != nil {
			return err
		}
	case uint64(kDefaultLogAddress):
		fmt.Fprintf(os.Stderr, "Pigweed Log: %s\n", string(frame.Payload()))
	default:
		return ErrBadAddress
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
