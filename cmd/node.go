/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"Nexus/internal/service"
	pb "Nexus/proto"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	memFlag     int64  // Memory in megabytes , it refers to the moemory of the node
	cpuFlag     uint64 // CPU weight , this refers to our cpu share s
	storageFlag string
	nodeService *service.NodeService //The service of our application
)
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage nexus nodes",
	Long:  `Parent command to manage the lifecycle of storage nodes (create, list, delete).`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// nodeCmd represents the node command
var nodeCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create and launch a nexus node with the specified allocated resources.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Connection to Daemon
		client, conn, ctx := getClient()
		defer conn.Close()

		// 2. Build Proto Request
		req := &pb.CreateNodeRequest{
			Name:        args[0],
			MemoryMb:    memFlag,
			CpuShares:   int64(cpuFlag),
			StorageSize: storageFlag,
		}

		fmt.Printf(" Sending request to Nexus Daemon...\n")
		// 3. RPC Call
		resp, err := client.CreateNode(ctx, req)
		if err != nil {
			fmt.Printf("-> RPC Error: %v\n", err)
			return
		}

		// 4. Display Response
		fmt.Printf("-> Node Created via gRPC!\n")
		fmt.Printf("   ID: %s | IP: %s | Status: %s\n", resp.Id, resp.Ip, resp.Status)
	},
}

// init registers the node management commands and defines CLI flags for the node create subcommand.
func init() {
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeCreateCmd)
	nodeCreateCmd.Flags().Int64Var(&memFlag, "mem", 128, "allocated memory in megabytes")
	nodeCreateCmd.Flags().Uint64Var(&cpuFlag, "cpu", 512, "cpu weight (0-1024)")
	nodeCreateCmd.Flags().StringVar(&storageFlag, "storage", "", "Size of persistent volume (e.g., 500M, 1G)")
}
