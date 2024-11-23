package main

import (
	"context"
	"fmt"
	"os"

	"github.com/robertfarnum/go_pw_rpc/cmd/benchmark/pb"
	"github.com/robertfarnum/go_pw_rpc/pkg/pw_rpc"
)

func RunUnary(ctx context.Context, bc pb.BenchmarkClient) {
	in := &pb.Payload{
		Payload: []byte("Hello"),
	}
	out, err := bc.UnaryEcho(ctx, in)
	if err != nil {
		fmt.Println("UnaryEcho error:", err)
		os.Exit(1)
	}

	fmt.Println("UnaryEcho() = " + string(out.Payload))
}

func RunBiDirectional(ctx context.Context, bc pb.BenchmarkClient) error {
	bid, err := bc.BidirectionalEcho(ctx)
	if err != nil {
		return err
	}

	go func() {
		for i := 0; i < 10; i++ {
			bid.Send(&pb.Payload{
				Payload: []byte(fmt.Sprintf("Message %d", i)),
			})
		}
	}()

	for {
		out, err := bid.Recv()
		if err != nil {
			return err
		}

		fmt.Println(string(out.Payload))
	}
}

func main() {
	ctx := context.Background()
	cc := pw_rpc.NewClientConn("localhost", 8111)
	defer cc.Close()

	bc := pb.NewBenchmarkClient(cc)

	RunUnary(ctx, bc)

	RunBiDirectional(ctx, bc)
}
