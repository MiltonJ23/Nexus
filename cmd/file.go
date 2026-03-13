/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	pb "Nexus/proto"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "Manage distributed files (upload, download, list)",
}

var uploadCmd = &cobra.Command{
	Use:   "upload [local-path]",
	Short: "Upload and distribute a file across nodes",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, conn, ctx := getClient()
		defer conn.Close()

		req := &pb.UploadFileRequest{
			LocalPath: args[0],
		}

		resp, err := client.UploadFile(ctx, req)
		if err != nil {
			fmt.Printf("-> Upload failed: %v\n", err)
			return
		}

		fmt.Printf("-> Upload success! File ID: %s\n", resp.Id)
	},
}

var downloadCmd = &cobra.Command{
	Use:   "download [file-id] [dest-path]",
	Short: "Reassemble and download a file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client, conn, ctx := getClient()
		defer conn.Close()

		req := &pb.DownloadFileRequest{
			FileId:   args[0],
			DestPath: args[1],
		}

		_, err := client.DownloadFile(ctx, req)
		if err != nil {
			fmt.Printf("-> Download failed: %v\n", err)
			return
		}

		fmt.Printf("-> Download success! Saved to %s\n", args[1])
	},
}

var listCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all files in the cloud",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn, ctx := getClient()
		defer conn.Close()

		resp, err := client.ListFiles(ctx, &pb.Empty{})
		if err != nil {
			fmt.Printf("-> Error: %v\n", err)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSIZE\tCHUNKS")
		for _, f := range resp.Files {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", f.Id[:8], f.Name, f.Size, f.ChunksCount)
		}
		w.Flush()
	},
}

// init registers the file command and its subcommands (upload, download, list) with the root command.
func init() {
	rootCmd.AddCommand(fileCmd)
	fileCmd.AddCommand(uploadCmd)
	fileCmd.AddCommand(downloadCmd)
	fileCmd.AddCommand(listCmd)
}
