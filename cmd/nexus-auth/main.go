package main

import (
	"Nexus/internal/identity"
	pb "Nexus/proto/auth"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
)

const AuthSocket = "/var/run/nexus-auth.sock"

func main() {
	// 1. Prepare Socket
	if _, err := os.Stat(AuthSocket); err == nil {
		os.Remove(AuthSocket)
	}
	lis, err := net.Listen("unix", AuthSocket)
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}
	// 2. Permission (Allow API Gateway to talk to us)
	if err := os.Chmod(AuthSocket, 0777); err != nil {
		panic(err)
	}
	// 3. Start gRPC Server
	s := grpc.NewServer()
	authService, err := identity.NewAuthServer()
	if err != nil {
		panic(err)
	}
	pb.RegisterAuthServiceServer(s, authService)
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nStopping Auth Service...")
		s.GracefulStop()
		os.Remove(AuthSocket)
		os.Exit(0)
	}()

	fmt.Printf("🔐 Identity Service running on %s (SQLite DB: /var/lib/nexus/auth.db)\n", AuthSocket)
	if err := s.Serve(lis); err != nil {
		panic(err)
	}
}
