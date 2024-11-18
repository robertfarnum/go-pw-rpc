package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"

	hdlc "github.com/zaninime/go-hdlc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/robertfarnum/go_pw_rpc/pb"
)

const (
	k65599HashConstant = uint32(65599)
)

type ClientConn struct {
	endpoint string
	port     int
	conn     net.Conn
}

func NewClientConn(endpoint string, port int) *ClientConn {
	return &ClientConn{
		endpoint: endpoint,
		port:     port,
	}
}

func (cc *ClientConn) Connect() (err error) {
	// Connect to the server
	cc.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", cc.endpoint, cc.port))
	if err != nil {
		fmt.Println("Error connecting:", err)
		return err
	}

	return nil
}

func (cc *ClientConn) Close() {
	if cc != nil && cc.conn != nil {
		cc.conn.Close()
	}
}

// Invoke sends the RPC request on the wire and returns after response is
// received.  This is typically called by generated code.
//
// All errors returned by Invoke are compatible with the status package.
func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {

	return nil
}

func (cc *ClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return NewClientStream(ctx, desc, cc, method, opts...)
}

type ClientStream struct {
	desc   *grpc.StreamDesc
	method string
	opts   []grpc.CallOption
	cc     *ClientConn
}

func NewClientStream(ctx context.Context, desc *grpc.StreamDesc, cc grpc.ClientConnInterface, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	clientConn, ok := cc.(*ClientConn)
	if ok {
		return &ClientStream{
			desc:   desc,
			cc:     clientConn,
			method: method,
			opts:   opts,
		}, nil
	}

	return nil, fmt.Errorf("failed to cast *ClientConn")
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

func (cs *ClientStream) SendMsg(m any) error {
	payload, ok := m.(protoreflect.ProtoMessage)
	if !ok {
		return fmt.Errorf("message not a ProtoMessage")
	}
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return err
	}

	serviceId := hash("pw.unit_test.UnitTest")
	fmt.Printf("serviceId = %x\n", serviceId)

	methodId := hash("Run")
	fmt.Printf("methodId = %x\n", methodId)

	packet := &pb.RpcPacket{
		Type:      pb.PacketType_REQUEST,
		ChannelId: 1,
		ServiceId: serviceId,
		MethodId:  methodId,
		Payload:   payloadBytes,
	}
	request, err := proto.Marshal(packet)
	if err != nil {
		return err
	}

	fmt.Printf("request size = %d, data: ", len(request))
	for _, b := range request {
		fmt.Printf("%02X ", b)
	}
	fmt.Println()

	encoder := hdlc.NewEncoder(cs.cc.conn)
	frame := hdlc.Encapsulate(request, true, []byte{0xa5, 0x01})
	if n, err := encoder.WriteFrame(frame); err != nil {
		return err
	} else {
		fmt.Printf("Sent message: len = %d\n", n)
	}

	return nil
}

func (cs *ClientStream) RecvMsg(m any) error {
	decoder := hdlc.NewDecoder(cs.cc.conn)
	frame, err := decoder.ReadFrame()
	if err != nil {
		panic(err)
	}

	b, err := json.Marshal(frame)
	if err == nil {
		fmt.Println("Frame: ", string(b))
		fmt.Println("Payload: ", string(frame.Payload))
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

func main() {
	ctx := context.Background()
	clientConn := NewClientConn("localhost", 8111)
	err := clientConn.Connect()
	if err != nil {
		fmt.Println("Error connecting:", err)
		os.Exit(1)
	}
	defer clientConn.Close()

	utc := pb.NewUnitTestClient(clientConn)
	in := &pb.TestRunRequest{
		ReportPassedExpectations: true,
	}

	streamingClient, err := utc.Run(ctx, in)
	if err != nil {
		fmt.Println("Error running test:", err)
		os.Exit(1)
	}

	for {
		streamingClient.RecvMsg(nil)
	}
}
