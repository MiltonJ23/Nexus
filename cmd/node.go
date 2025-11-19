/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"Nexus/internal/service"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	memFlag     int64                // Memory in megabytes , it refers to the moemory of the node
	cpuFlag     uint64               // CPU weight , this refers to our cpu share s
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
		nodeName := args[0]
		fmt.Printf("Launching the node creation process  %s...\n", nodeName)
		// let's call the logic metier for creating a node
		state, err := nodeService.CreateNode(nodeName, memFlag, cpuFlag)
		if err != nil {
			fmt.Printf("Failed the node creation%s: %v\n", nodeName, err)
			fmt.Println("Make sure you use SUDO! (Ex: sudo ./nexus node create node-1)")
			return
		}
		fmt.Printf(" Node '%s' created and is currently running .\n", state.ID)
		fmt.Printf("   -> Memory limit of the node : %d MB\n", state.Memory)
		fmt.Printf("   -> number of nano cpus allocated : %d\n", state.CPUShares)
		fmt.Printf("   -> process id of the host: %d\n", state.PID)
	},
}

func init() {
	var err error
	nodeService, err = service.NewNodeService()
	if err != nil {
		// Faced a critical error unable to exchange with the kernel
		panic(fmt.Sprintf("Critical Error when initializing the service: %v", err))
	}
	// let's add the command create
	nodeCmd.AddCommand(nodeCreateCmd)
	//  let's add the flags for the create command
	nodeCreateCmd.Flags().Int64Var(&memFlag, "mem", 128, "allocated memory in megabytes (ex: 64, 256, 512)")
	nodeCreateCmd.Flags().Uint64Var(&cpuFlag, "cpu", 512, "cpu weight , to choose from (0 à 1024) , but  a  standard  is 512)")
}
