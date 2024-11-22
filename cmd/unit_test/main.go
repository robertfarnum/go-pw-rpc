package main

import (
	"context"
	"fmt"
	"os"

	"github.com/robertfarnum/go_pw_rpc/cmd/unit_test/pb"
	"github.com/robertfarnum/go_pw_rpc/pkg/pw_rpc"
)

func main() {
	ctx := context.Background()
	clientConn := pw_rpc.NewSocketConn("localhost", 8111)
	err := clientConn.Connect()
	if err != nil {
		fmt.Println("Error connecting:", err)
		os.Exit(1)
	}
	defer clientConn.Close()

	utc := pb.NewUnitTestClient(clientConn)
	in := &pb.TestRunRequest{
		ReportPassedExpectations: true,
		TestSuite:                []string{"Passing", "Failing"},
	}

	streamingClient, err := utc.Run(ctx, in)
	if err != nil {
		fmt.Println("Error running test:", err)
		os.Exit(1)
	}

	for {
		out := &pb.Event{}
		err := streamingClient.RecvMsg(out)
		if err == nil {
			switch evt := out.GetType().(type) {
			case *pb.Event_TestRunStart:
				fmt.Println(evt.TestRunStart.String())
			case *pb.Event_TestRunEnd:
				fmt.Println(evt.TestRunEnd.String())
			case *pb.Event_TestCaseStart:
				fmt.Println(evt.TestCaseStart.String())
			case *pb.Event_TestCaseEnd:
				fmt.Println(evt.TestCaseEnd.String())
			case *pb.Event_TestCaseDisabled:
				fmt.Println(evt.TestCaseDisabled.String())
			case *pb.Event_TestCaseExpectation:
				fmt.Println(evt.TestCaseExpectation.String())
			}
		}
	}
}
