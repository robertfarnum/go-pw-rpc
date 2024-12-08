package pw_rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
	"google.golang.org/grpc"
)

const ()

var (
	ErrClientIsNil = errors.New("client is nil")
)

type Client interface {
	grpc.ClientConnInterface
	PacketHandler
	GetConn() Conn
	CloseStream(Stream)
	Close()
}

type client struct {
	endpoint      string
	conn          Conn
	streamManager StreamManager
	mu            sync.Mutex
}

func NewClient(endpoint string) Client {
	return &client{
		endpoint:      endpoint,
		conn:          nil,
		streamManager: NewStreamManager(),
	}
}

func (c *client) connectAttempt(ctx context.Context) (conn net.Conn, err error) {
	if c == nil {
		return nil, ErrClientIsNil
	}

	select {
	case <-ctx.Done():
		return nil, ErrCancelled
	default:
		conn, err = net.Dial("tcp", c.endpoint)
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

func (c *client) connect(ctx context.Context) error {
	if c == nil {
		return ErrClientIsNil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	conn, err := c.connectAttempt(ctx)
	for err != nil {
		select {
		case <-ctx.Done():
			return ErrCancelled
		case <-time.After(time.Second):
			conn, err = c.connectAttempt(ctx)
		}
	}

	c.conn = NewConn(conn, c)

	go func() {
		c.mu.Lock()
		if c.conn == nil {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		ctx, cancel := context.WithCancel(ctx)
		err := c.conn.Recv(ctx)
		if err != nil {
			fmt.Printf("Server Disconnect: %s\n", err)
			c.mu.Lock()
			c.conn.Close()
			c.conn = nil
			c.mu.Unlock()
		}
		cancel()
	}()

	return err
}

func (c *client) GetConn() Conn {
	return c.conn
}

func (c *client) CloseStream(stream Stream) {
	c.streamManager.RemoveStream(stream)
}

func (c *client) HandlePacket(ctx context.Context, conn Conn, packet *pb.RpcPacket) error {
	switch packet.Type {
	case pb.PacketType_REQUEST:
		return fmt.Errorf("client received request packet")
	case pb.PacketType_CLIENT_ERROR:
		return fmt.Errorf("client received client error packet")
	case pb.PacketType_CLIENT_STREAM:
		return fmt.Errorf("client received client stream packet")
	case pb.PacketType_RESPONSE, pb.PacketType_SERVER_STREAM, pb.PacketType_SERVER_ERROR:
		s := c.streamManager.GetStream(Key(packet.ServiceId), Key(packet.MethodId))
		if s == nil {
			return fmt.Errorf("stream not found: %d %d", packet.ServiceId, packet.MethodId)
		}

		s.PacketReceived(packet)

		return nil
	}

	return fmt.Errorf("invalid packet type: %s", packet.Type)
}

func (c *client) Close() {
	if c == nil {
		return
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *client) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	err := c.connect(ctx)
	if err != nil {
		return err
	}

	stream, err := NewStream(ctx, nil, c.conn, method, opts...)
	if err != nil {
		return err
	}

	c.streamManager.AddStream(stream)

	if err := stream.Send(args, pb.StatusCode_OK, pb.PacketType_REQUEST); err != nil {
		return err
	}

	_, _, err = stream.Recv(reply)

	c.streamManager.RemoveStream(stream)

	return err
}

func (c *client) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	err := c.connect(ctx)
	if err != nil {
		return nil, err
	}

	if c.conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	stream, err := NewClientStream(ctx, desc, c, method, opts...)
	if err != nil {
		return nil, err
	}

	c.streamManager.AddStream(stream.GetStream())

	return stream, err
}
