package pw_rpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
)

type Server interface {
	PacketHandler

	RegisterService(desc *grpc.ServiceDesc, impl any)
	Listen(ctx context.Context) error
	Close()
}

// serviceInfo wraps information about a service. It is very similar to
// ServiceDesc and is constructed from it for internal purposes.
type serviceInfo struct {
	// Contains the implementation for the methods in this service.
	serviceImpl any
	name        string
	methods     map[Key]*grpc.MethodDesc
	streams     map[Key]*grpc.StreamDesc
	mdata       any
}

type server struct {
	endpoint      string
	lis           net.Listener
	services      map[Key]*serviceInfo // service name -> service info
	streamManager StreamManager
	conn          Conn
	mu            sync.Mutex
}

func NewServer(endpoint string) Server {
	return &server{
		endpoint:      endpoint,
		services:      make(map[Key]*serviceInfo),
		streamManager: NewStreamManager(),
	}
}

func (s *server) HandleRequestPacket(ctx context.Context, conn Conn, packet *pb.RpcPacket) error {
	service, ok := s.services[Key(packet.ServiceId)]
	if !ok {
		return fmt.Errorf("service not found: %d", packet.ServiceId)
	}

	method, ok := service.methods[Key(packet.MethodId)]
	if ok {
		res, err := method.Handler(service.serviceImpl, ctx, func(in interface{}) error {
			payload, ok := in.(protoreflect.ProtoMessage)
			if !ok {
				return fmt.Errorf("invalid payload type: %T", in)
			}

			err := proto.Unmarshal(packet.Payload, payload)
			if err != nil {
				return err
			}

			return nil
		}, nil)
		if err != nil {
			return err
		}

		payload, ok := res.(protoreflect.ProtoMessage)
		if !ok {
			return fmt.Errorf("invalid payload type: %T", res)
		}

		method := fmt.Sprintf("/%s/%s", service.name, method.MethodName)
		stream, err := NewStream(ctx, nil, conn, method, nil)
		if err != nil {
			return err
		}

		return stream.Send(payload, pb.PacketType_RESPONSE)
	}

	desc, ok := service.streams[Key(packet.MethodId)]
	if ok {
		method := fmt.Sprintf("/%s/%s", service.name, desc.StreamName)
		stream, err := NewServerStream(ctx, desc, conn, method, nil)
		if err != nil {
			return err
		}

		s.streamManager.AddStream(stream.GetStream())

		go func() {
			err := desc.Handler(service.serviceImpl, stream)
			if err != nil {
				fmt.Printf("Error handling stream: %s\n", err)
			}
		}()

		return nil
	}

	return fmt.Errorf("method and stream not found: %d", packet.MethodId)
}

func (s *server) HandlePacket(ctx context.Context, conn Conn, packet *pb.RpcPacket) error {
	switch packet.Type {
	case pb.PacketType_REQUEST:

		err := s.HandleRequestPacket(ctx, conn, packet)
		if err != nil {
			fmt.Printf("Error handling request packet: %s\n", err)
		}

		return nil
	case pb.PacketType_CLIENT_STREAM, pb.PacketType_CLIENT_REQUEST_COMPLETION, pb.PacketType_CLIENT_ERROR:
		s := s.streamManager.GetStream(Key(packet.ServiceId), Key(packet.MethodId))
		if s == nil {
			return fmt.Errorf("stream not found: %d %d", packet.ServiceId, packet.MethodId)
		}

		s.PacketReceived(packet)

		return nil
	case pb.PacketType_RESPONSE:
		return fmt.Errorf("server received request packet")
	case pb.PacketType_SERVER_STREAM:
		return fmt.Errorf("server received client stream packet")
	}

	return fmt.Errorf("invalid packet type: %s", packet.Type)
}

func (s *server) RegisterService(sd *grpc.ServiceDesc, ss any) {
	if s != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			log.Fatalf("grpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
		}
	}
	s.register(sd, ss)
}

func (s *server) register(sd *grpc.ServiceDesc, ss any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("RegisterService(%q)", sd.ServiceName)
	if s.lis != nil {
		log.Fatalf("grpc: Server.RegisterService after Server.Serve for %q", sd.ServiceName)
	}
	if _, ok := s.services[NewKey(sd.ServiceName)]; ok {
		log.Fatalf("grpc: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
	}
	info := &serviceInfo{
		serviceImpl: ss,
		name:        sd.ServiceName,
		methods:     make(map[Key]*grpc.MethodDesc),
		streams:     make(map[Key]*grpc.StreamDesc),
		mdata:       sd.Metadata,
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		info.methods[NewKey(d.MethodName)] = d
	}
	for i := range sd.Streams {
		d := &sd.Streams[i]
		info.streams[NewKey(d.StreamName)] = d
	}
	s.services[NewKey(sd.ServiceName)] = info
}

func (s *server) Listen(ctx context.Context) (err error) {
	if s.lis == nil {
		s.lis, err = net.Listen("tcp", s.endpoint)
		if err != nil {
			fmt.Println("Error listening:", err.Error())
			return err
		}

		defer func() {
			s.lis.Close()
			s.lis = nil
		}()

		fmt.Printf("Server listening: %s\n", s.endpoint)

		for {
			if s.lis == nil {
				break
			}

			// Accept incoming connections
			conn, err := s.lis.Accept()
			if err != nil {
				fmt.Println("Error accepting connection:", err.Error())
				continue
			}

			s.conn = NewConn(conn, s)

			go func() {
				err := s.conn.Recv(ctx)
				if err != nil {
					fmt.Printf("Client Disconnect: %s\n", err)
					return
				}
			}()
		}
	}

	return nil
}

func (s *server) Close() {
	s.lis.Close()
	s.lis = nil
}
