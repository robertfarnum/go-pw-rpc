package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/robertfarnum/go_pw_rpc/cmd/benchmark/pb"
	"github.com/robertfarnum/go_pw_rpc/pkg/pw_rpc"
)

func RunUnary(ctx context.Context, bc pb.BenchmarkClient, count int) {
	for i := 0; i < count; i++ {
		str := fmt.Sprintf("Hello #%d", i)
		in := &pb.Payload{
			Payload: []byte(str),
		}

		fmt.Printf("Sending UnaryEcho = %s\n", str)

		out, err := bc.UnaryEcho(ctx, in)
		if err != nil {
			fmt.Println("UnaryEcho error:", err)
			os.Exit(1)
		}

		fmt.Printf("Received UnaryEcho = %s\n", string(out.Payload))

		time.Sleep(time.Second)
	}
}

func RunBiDirectional(ctx context.Context, bc pb.BenchmarkClient, count int) error {
	bid, err := bc.BidirectionalEcho(ctx)
	if err != nil {
		return err
	}

	go func() {
		for i := 0; i < count; i++ {
			str := fmt.Sprintf("Message %d", i)
			fmt.Printf("Sending BiDirectional = %s\n", str)
			bid.Send(&pb.Payload{
				Payload: []byte(str),
			})
		}
	}()

	for {
		out, err := bid.Recv()
		if err != nil {
			break
		}

		fmt.Printf("Received BiDirectional = %s\n", string(out.Payload))
	}

	return nil
}

func main() {
	ctx := context.Background()
	cc := pw_rpc.NewClientConn("localhost", 8111)
	defer cc.Close()

	bc := pb.NewBenchmarkClient(cc)

	go RunUnary(ctx, bc, 10)

	RunBiDirectional(ctx, bc, 10)
}
