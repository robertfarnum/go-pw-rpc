package pw_rpc

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_hdlc"
	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc/pb"
	"google.golang.org/protobuf/proto"
)

var (
	kDefaultRpcAddress = 'R'
	kDefaultLogAddress = 1
)

type PacketHandler interface {
	HandlePacket(context.Context, Conn, *pb.RpcPacket) error
}

type Conn interface {
	Recv(context.Context) error
	Send(context.Context, *pb.RpcPacket) error
	Close()
}

type conn struct {
	conn    net.Conn
	encoder pw_hdlc.Encoder
	decoder pw_hdlc.Decoder
	ph      PacketHandler
}

func NewConn(netConn net.Conn, ph PacketHandler) Conn {
	return &conn{
		conn:    netConn,
		encoder: pw_hdlc.NewEncoder(netConn, uint64(kDefaultRpcAddress)),
		decoder: pw_hdlc.NewDecoder(netConn, uint64(kDefaultRpcAddress)),
		ph:      ph,
	}
}

func (c *conn) Recv(ctx context.Context) error {
	defer c.Close()

	for {
		select {
		case <-ctx.Done():
			return ErrCancelled
		default:
			err := c.recv(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (c *conn) processFrame(ctx context.Context, frame *pw_hdlc.Frame) error {
	switch frame.Address() {
	case uint64(kDefaultRpcAddress):
		packet := &pb.RpcPacket{}
		err := proto.Unmarshal(frame.Payload(), packet)
		if err != nil {
			return err
		}

		if c.ph == nil {
			return fmt.Errorf("packet handler is nil")
		}

		return c.ph.HandlePacket(ctx, c, packet)
	case uint64(kDefaultLogAddress):
		fmt.Fprintf(os.Stderr, "Pigweed Log: %s\n", string(frame.Payload()))
	default:
		return ErrBadAddress
	}

	return nil
}

func (c *conn) recv(ctx context.Context) error {
	frame, err := c.decoder.Decode(ctx)
	if err != nil {
		return err
	}

	if frame != nil {
		return c.processFrame(ctx, frame)
	}

	return fmt.Errorf("no frame")
}

func (c *conn) Send(ctx context.Context, packet *pb.RpcPacket) error {
	buf, err := proto.Marshal(packet)
	if err != nil {
		return err
	}

	return c.encoder.Encode(buf)
}

func (c *conn) Close() {
	if c != nil && c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}
