package pw_rpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Key uint32

type streamKey struct {
	serviceId Key
	methodId  Key
}

func NewKey(name string) Key {
	return Key(hash(name))
}

type StreamKey streamKey

func NewStreamKey(serviceName string, methodName string) StreamKey {
	serviceId := NewKey(serviceName)
	methodId := NewKey(methodName)
	return StreamKey{
		serviceId: serviceId,
		methodId:  methodId,
	}
}

type Stream interface {
	Key() StreamKey
	Context() context.Context
	Send(any, pb.PacketType) error
	Recv(any) error
	PacketReceived(*pb.RpcPacket)
	Close()
}

type stream struct {
	conn   Conn
	desc   *grpc.StreamDesc
	method string
	opts   []grpc.CallOption
	key    StreamKey
	ch     chan (*pb.RpcPacket)
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *stream) Context() context.Context {
	return s.ctx
}

func (s *stream) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *stream) Key() StreamKey {
	return s.key
}

func NewStream(ctx context.Context, desc *grpc.StreamDesc, conn Conn, method string, opts ...grpc.CallOption) (Stream, error) {
	methodParts := strings.Split(method, "/")
	if len(methodParts) != 3 {
		return nil, fmt.Errorf("invalid full method name")
	}

	serviceName := methodParts[1]
	methodName := methodParts[2]
	ctx, cancel := context.WithCancel(ctx)

	return &stream{
		conn:   conn,
		desc:   desc,
		method: method,
		opts:   opts,
		key:    NewStreamKey(serviceName, methodName),
		ch:     make(chan *pb.RpcPacket),
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (s *stream) Send(m any, packetType pb.PacketType) (err error) {
	payload := []byte{}

	if m != nil {
		pm, ok := m.(protoreflect.ProtoMessage)
		if !ok {
			return fmt.Errorf("message not a ProtoMessage")
		}
		payload, err = proto.Marshal(pm)
		if err != nil {
			return err
		}
	}

	packet := &pb.RpcPacket{
		Type:      packetType,
		ChannelId: kHDLCChannel,
		ServiceId: uint32(s.key.serviceId),
		MethodId:  uint32(s.key.methodId),
		Payload:   payload,
	}

	return s.conn.Send(s.ctx, packet)
}

func (s *stream) Recv(m any) error {
	pm, ok := m.(protoreflect.ProtoMessage)
	if !ok {
		return fmt.Errorf("message not a ProtoMessage")
	}

	for {
		select {
		case <-s.ctx.Done():
			return fmt.Errorf("cancel")
		case packet, ok := <-s.ch:
			if !ok || packet.ChannelId != kHDLCChannel || Key(packet.ServiceId) != s.key.serviceId || Key(packet.MethodId) != s.key.methodId {
				return fmt.Errorf("invalid packet received")
			}

			err := proto.Unmarshal(packet.Payload, pm)
			if err != nil {
				return err
			}

			return nil
		}
	}
}

func (s *stream) PacketReceived(packet *pb.RpcPacket) {
	s.ch <- packet
}

type streamsMap map[StreamKey]Stream

type StreamManager interface {
	GetStream(serviceId Key, methodId Key) Stream
	AddStream(Stream)
	RemoveStream(Stream)
	Reset()
}

type streamManager struct {
	streams streamsMap
}

func NewStreamManager() StreamManager {
	return &streamManager{
		streams: make(streamsMap),
	}
}

func (sm *streamManager) GetStream(serviceId Key, methodId Key) Stream {
	return sm.streams[StreamKey{
		serviceId: serviceId,
		methodId:  methodId,
	}]
}

func (sm *streamManager) AddStream(s Stream) {
	sm.streams[s.Key()] = s
}

func (sm *streamManager) RemoveStream(s Stream) {
	s.Close()

	delete(sm.streams, s.Key())
}

func (sm *streamManager) Reset() {
	for key := range sm.streams {
		s := sm.streams[key]
		sm.RemoveStream(s)
	}
	sm.streams = make(streamsMap)
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
