package pw_rpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_hdlc"
	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

var (
	kDefaultRpcAddress = 'R'
	kDefaultLogAddress = 1
)

type ClientConn struct {
	endpoint string
	port     int
	conn     net.Conn
	encoder  pw_hdlc.Encoder
	decoder  pw_hdlc.Decoder
	streams  map[ClientStreamKey]*clientStream
	mu       sync.Mutex
}

func NewClientConn(endpoint string, port int) *ClientConn {
	return &ClientConn{
		endpoint: endpoint,
		port:     port,
		streams:  make(map[ClientStreamKey]*clientStream, 0),
	}
}

func (cc *ClientConn) processFrame(frame *pw_hdlc.Frame) error {
	switch frame.Address() {
	case uint64(kDefaultRpcAddress):
		packet := &pb.RpcPacket{}
		err := proto.Unmarshal(frame.Payload(), packet)
		if err != nil {
			return err
		}

		key := ClientStreamKey{
			serviceId: packet.ServiceId,
			methodId:  packet.MethodId,
		}

		stream := cc.streams[key]
		if stream != nil {
			stream.recv(packet)
		}

	case uint64(kDefaultLogAddress):
		fmt.Fprintf(os.Stderr, "Pigweed Log: %s\n", string(frame.Payload()))
	default:
		return ErrBadAddress
	}

	return nil
}

func (cc *ClientConn) connect() (err error) {
	cc.mu.Lock()
	if cc.conn == nil {
		cc.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", cc.endpoint, cc.port))
		if err != nil {
			cc.mu.Unlock()
			return err
		}

		cc.encoder = pw_hdlc.NewEncoder(cc.conn, uint64(kDefaultRpcAddress))
		cc.decoder = pw_hdlc.NewDecoder(cc.conn, uint64(kDefaultRpcAddress))

		go func() {
			for {
				frame, err := cc.decoder.Decode()
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					break
				}

				if frame != nil {
					cc.processFrame(frame)
				}
			}
			cc.cancel()
		}()
	}

	cc.mu.Unlock()

	return nil
}

func (cc *ClientConn) Send(message []byte) error {
	return cc.encoder.Encode(message)
}

func (cc *ClientConn) cancel() {
	for key := range cc.streams {
		stream := cc.streams[key]
		stream.cancel()
	}
	cc.streams = make(map[ClientStreamKey]*clientStream)
}

func (cc *ClientConn) Close() {
	if cc != nil && cc.conn != nil {
		cc.conn.Close()
		cc.conn = nil
	}
}

func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	cs, err := cc.NewStream(
		ctx,
		&grpc.StreamDesc{
			ServerStreams: false,
			ClientStreams: false,
		},
		method,
		opts...,
	)
	if err != nil {
		return err
	}
	if err := cs.SendMsg(args); err != nil {
		return err
	}

	err = cs.RecvMsg(reply)

	clientStream, ok := cs.(*clientStream)
	if !ok {
		return fmt.Errorf("failed to cast to clientStream")
	}

	clientStream.cancel()
	delete(cc.streams, clientStream.Key())

	return err
}

func (cc *ClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	cc.connect()

	stream, err := newClientStream(ctx, desc, cc, method, opts...)
	cc.streams[stream.Key()] = stream

	return stream, err
}
