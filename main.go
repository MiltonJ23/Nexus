/*
Copyright © 2025 Zingui Fred Mike <mikezingui@yahoo.com>
*/
package main

import (
	"Nexus/cmd"
	"os"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		// We bypass Cobra entirely.
		// libcontainer.Init() will read the config pipe, setup the FS,
		// perform PivotRoot/Chroot, and exec the user command.
		libcontainer.Init()
		return
	}
	cmd.Execute()
}
