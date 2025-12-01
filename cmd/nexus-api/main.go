package main

import (
	"Nexus/internal/gateway"
	pb "Nexus/proto"
	auth_pb "Nexus/proto/auth"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 1. Connect to Infrastructure Daemon (nexusd)
	// We use WithBlock to ensure connection before starting
	fmt.Println("-> Connecting to Nexus Daemon...")
	nexusConn, err := grpc.Dial("unix:///var/run/nexus.sock",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Nexus Daemon: %v", err))
	}
	nexusClient := pb.NewNexusControllerClient(nexusConn)

	// 2. Connect to Identity Service (nexus-auth)
	fmt.Println("🔌 Connecting to Identity Service...")
	authConn, err := grpc.Dial("unix:///var/run/nexus-auth.sock",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Auth Service: %v", err))
	}
	authClient := auth_pb.NewAuthServiceClient(authConn)

	// 3. Initialize Gateway Logic
	server := gateway.NewServer(nexusClient, authClient)
	router := server.SetupRouter()

	// 4. Start HTTP Server
	port := "8080"
	fmt.Printf("-> Nexus API Gateway is LIVE at http://localhost:%s\n", port)

	if err := router.Run(":" + port); err != nil {
		panic(err)
	}
}
