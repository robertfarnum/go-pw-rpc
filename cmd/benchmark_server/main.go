package main

import (
	"context"
	"fmt"

	"github.com/robertfarnum/go-pw-rpc/cmd/pb"
	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc"
	"google.golang.org/grpc"
)

type BenchmarkServer struct {
	pb.UnsafeBenchmarkServer
}

func (bs BenchmarkServer) UnaryEcho(ctx context.Context, payload *pb.Payload) (*pb.Payload, error) {
	fmt.Printf("Received UnaryEcho = %s\n", string(payload.Payload))

	return &pb.Payload{
		Payload: payload.GetPayload(),
	}, nil
}

func (bs BenchmarkServer) BidirectionalEcho(s grpc.BidiStreamingServer[pb.Payload, pb.Payload]) error {
	fmt.Printf("Received BidirectionalEcho\n")

	return nil
}

func main() {
	ctx := context.Background()

	s := pw_rpc.NewServer("localhost:8111")

	bs := &BenchmarkServer{}

	pb.RegisterBenchmarkServer(s, bs)

	s.Listen(ctx)
}
