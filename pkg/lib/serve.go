package lib

import (
	"fmt"
	"github.com/speza/runner/proto"
	"google.golang.org/grpc"
	"net"
)

func Serve(executor proto.ExecutorServer) error {
	listen, err := net.Listen("tcp", ":5300")
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	server := grpc.NewServer()
	proto.RegisterExecutorServer(server, executor)

	if err := server.Serve(listen); err != nil {
		return fmt.Errorf("failed to serve grpc server: %w", err)
	}
	return nil
}
