/*
Copyright © 2025 Zingui Fred Mike <mikezingui@yahoo.com>
*/
package cmd

import (
	"Nexus/internal/service"
	"Nexus/internal/state"
	pb "Nexus/proto"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcserver "Nexus/internal/grpc"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

const SocketPath = "/var/run/nexus.sock"

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the Nexus Control Plane (Requires Root)",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Vérification Root (Subtilité critique)
		if os.Geteuid() != 0 {
			fmt.Println("-> Error: Daemon must run as root to manage containers/mounts.")
			os.Exit(1)
		}
		// let's clean in case there was a previous socket coming from another execution
		if _, err := os.Stat(SocketPath); err == nil {
			os.Remove(SocketPath)
		}
		// let's create the unix listenener
		lis, err := net.Listen("unix", SocketPath)
		if err != nil {
			panic(fmt.Sprintf("failed to listen on socket: %v", err))
		}
		grpcServer := grpc.NewServer()
		nexusService, err := grpcserver.NewNexusServer()
		if err != nil {
			panic(fmt.Sprintf("failed to init services: %v", err))
		}
		pb.RegisterNexusControllerServer(grpcServer, nexusService)
		// Initialize Metrics Service
		metricsSvc := service.NewMetricsService(state.GlobalState)
		metricsSvc.Start(5 * time.Second) // Collect every 5s
		go func() {
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			<-sigChan
			fmt.Println("\n🔻 Stopping Nexus Daemon...")
			grpcServer.GracefulStop()
			metricsSvc.Stop()
			os.Remove(SocketPath)
			os.Exit(0)
		}()

		fmt.Printf("-> Nexus Daemon listening on %s\n", SocketPath)

		// 6. Permission du Socket (Subtilité critique)
		// Par défaut, le socket root est 700. Le CLI utilisateur ne pourra pas écrire.
		// On met 777 (ou 666) pour permettre à tout utilisateur local de parler au daemon.
		// Dans un vrai prod, on utiliserait un groupe 'docker' ou 'nexus'.
		if err := os.Chmod(SocketPath, 0777); err != nil {
			panic(err)
		}

		if err := grpcServer.Serve(lis); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
