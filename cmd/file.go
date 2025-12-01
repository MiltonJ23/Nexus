/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"Nexus/internal/service"
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
		svc := service.NewFileService()
		meta, err := svc.UploadFile(args[0])
		if err != nil {
			fmt.Printf("-> Upload failed: %v\n", err)
			return
		}
		fmt.Printf("-> Upload success! File ID: %s\n", meta.ID)
	},
}

var downloadCmd = &cobra.Command{
	Use:   "download [file-id] [dest-path]",
	Short: "Reassemble and download a file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		svc := service.NewFileService()
		err := svc.DownloadFile(args[0], args[1])
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
		svc := service.NewFileService()
		files := svc.ListFiles()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSIZE\tCHUNKS")
		for _, f := range files {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", f.ID[:8], f.Name, f.Size, len(f.Chunks))
		}
		w.Flush()
	},
}

var deleteCmd = &cobra.Command{
	Use:   "rm [file-id]",
	Short: "Delete a file from the cloud",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		svc := service.NewFileService()
		err := svc.DeleteFile(args[0])
		if err != nil {
			fmt.Printf("-> Delete failed: %v\n", err)
			return
		}
		fmt.Printf("-> File deleted.\n")
	},
}

func init() {
	rootCmd.AddCommand(fileCmd)
	fileCmd.AddCommand(uploadCmd)
	fileCmd.AddCommand(downloadCmd)
	fileCmd.AddCommand(listCmd)
	fileCmd.AddCommand(deleteCmd)
}
