package cmd

import (
	pb "Nexus/proto"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var fsCmd = &cobra.Command{
	Use:   "fs",
	Short: "Interact with the Nexus Virtual File System",
	Long:  `Manage files and directories within the Nexus Cloud environment via the Daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help() // Check subcommands
	},
}

// 2. Subcommand: Mkdir
var fsMkdirCmd = &cobra.Command{
	Use:   "mkdir [path]",
	Short: "Create a directory in the VFS",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

		// UPDATED: Matches your client_utils.go signature (client, conn, ctx)
		client, conn, ctx := getClient()
		defer conn.Close()

		// Call gRPC
		_, err := client.FSMakeDir(ctx, &pb.FSRequest{
			Path:     path,
			Username: "cli-user", // Hardcoded for now, waiting for auth config
		})

		if err != nil {
			fmt.Printf("-> Error creating directory: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("-> Directory '%s' created.\n", path)
	},
}

// 3. Subcommand: Ls (List)
var fsListCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List files and directories",
	Args:  cobra.MaximumNArgs(1), // Path is optional (defaults to root)
	Run: func(cmd *cobra.Command, args []string) {
		path := "/"
		if len(args) > 0 {
			path = args[0]
		}

		// UPDATED: Matches your client_utils.go signature
		client, conn, ctx := getClient()
		defer conn.Close()

		resp, err := client.FSList(ctx, &pb.FSRequest{
			Path:     path,
			Username: "cli-user",
		})

		if err != nil {
			fmt.Printf("-> Error listing directory: %v\n", err)
			os.Exit(1)
		}

		// Print pretty table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "TYPE\tNAME\tSIZE")
		for _, item := range resp.Items {
			icon := "📄"
			if item.Type == "folder" {
				icon = "📁"
			}
			fmt.Fprintf(w, "%s\t%s\t%d bytes\n", icon, item.Name, item.Size)
		}
		w.Flush()
	},
}

// 4. Subcommand: Upload
var fsUploadCmd = &cobra.Command{
	Use:   "upload [local_path] [virtual_path]",
	Short: "Upload a file to the VFS",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		localPath, _ := filepath.Abs(args[0])
		virtualPath := args[1]

		// UPDATED: Matches your client_utils.go signature
		client, conn, ctx := getClient()
		defer conn.Close()

		_, err := client.FSUpload(ctx, &pb.FSUploadRequest{
			LocalPath:   localPath, // Daemon needs absolute path to read it
			VirtualPath: virtualPath,
			Username:    "cli-user",
		})

		if err != nil {
			fmt.Printf("-> Upload failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("-> Uploaded %s to %s\n", localPath, virtualPath)
	},
}

// init registers the fs command and its mkdir and ls subcommands with rootCmd.
func init() {
	rootCmd.AddCommand(fsCmd)
	fsCmd.AddCommand(fsMkdirCmd)
	fsCmd.AddCommand(fsListCmd)
}
