package main

import (
	"context"

	"github.com/robertfarnum/go-pw-rpc/cmd/pb"
	"github.com/robertfarnum/go-pw-rpc/pkg/pw_rpc"
	"google.golang.org/grpc"
)

type UnitTestServer struct {
	pb.UnsafeUnitTestServer
}

// Run implements pb.UnitTestServer.
func (u UnitTestServer) Run(request *pb.TestRunRequest, server grpc.ServerStreamingServer[pb.Event]) error {
	for _, suite := range request.TestSuite {
		err := server.SendMsg(&pb.Event{
			Type: &pb.Event_TestRunStart{
				TestRunStart: &pb.TestRunStart{},
			},
		})
		if err != nil {
			return err
		}

		if suite == "Failing" {
			err := server.Send(&pb.Event{
				Type: &pb.Event_TestCaseExpectation{
					TestCaseExpectation: &pb.TestCaseExpectation{
						Expression: "1 == 2",
						Success:    false,
					},
				},
			})
			if err != nil {
				return err
			}
		} else {
			for _, test := range []string{"Test 1", "Test 2"} {
				err := server.Send(&pb.Event{
					Type: &pb.Event_TestCaseStart{
						TestCaseStart: &pb.TestCaseDescriptor{
							SuiteName: suite,
							TestName:  test,
							FileName:  "File",
						},
					},
				})
				if err != nil {
					return err
				}
				server.Send(&pb.Event{
					Type: &pb.Event_TestCaseEnd{},
				})
			}
			err := server.Send(&pb.Event{
				Type: &pb.Event_TestRunEnd{},
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func main() {
	ctx := context.Background()

	s := pw_rpc.NewServer("localhost:8112")

	uts := UnitTestServer{}

	pb.RegisterUnitTestServer(s, uts)

	s.Listen(ctx)
}
