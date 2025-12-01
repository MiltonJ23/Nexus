package cmd

import (
	pb "Nexus/proto"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"time"
)

const SocketAddr = "unix:///var/run/nexus.sock"

func getClient() (pb.NexusControllerClient, *grpc.ClientConn, context.Context) {
	ctx := context.Background()

	conn, err := grpc.NewClient(SocketAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("\n -> Connection failed. Is the daemon running? (sudo ./nexus daemon)\n")
		fmt.Printf("   Error: %v\n", err)
		os.Exit(1)
	}
	// Test the connection with a timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn.Connect()

	return pb.NewNexusControllerClient(conn), conn, context.Background()
}
