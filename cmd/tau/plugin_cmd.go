//go:build !no_plugins

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/akzj/tau/core"
)

func pluginCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: tau plugin <list|install|remove|info> [args]\n")
		os.Exit(1)
	}

	dir := os.Getenv("TAU_PLUGIN_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".tau", "plugins")
	}

	switch args[0] {
	case "list":
		infos, err := core.ListPlugins(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "list: %v\n", err)
			os.Exit(1)
		}
		if len(infos) == 0 {
			fmt.Println("No plugins installed.")
			return
		}
		fmt.Printf("%-25s %-10s %-8s %s\n", "NAME", "VERSION", "TOOLS", "STATUS")
		for _, info := range infos {
			fmt.Printf("%-25s %-10s %-8d %s\n", info.Name, info.Version, len(info.Tools), info.Status)
		}

	case "install":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: tau plugin install <path>\n")
			os.Exit(1)
		}
		if err := core.InstallPlugin(args[1], dir); err != nil {
			fmt.Fprintf(os.Stderr, "install: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Installed %s\n", filepath.Base(args[1]))

	case "remove":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: tau plugin remove <name>\n")
			os.Exit(1)
		}
		if err := core.RemovePlugin(args[1], dir); err != nil {
			fmt.Fprintf(os.Stderr, "remove: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Removed %s\n", args[1])

	case "info":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: tau plugin info <name>\n")
			os.Exit(1)
		}
		infos, err := core.ListPlugins(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "info: %v\n", err)
			os.Exit(1)
		}
		for _, info := range infos {
			if info.Name == args[1] {
				fmt.Printf("Name:    %s\nVersion: %s\nStatus:  %s\nTools:   %s\n",
					info.Name, info.Version, info.Status, strings.Join(info.Tools, ", "))
				return
			}
		}
		fmt.Fprintf(os.Stderr, "Plugin not found: %s\n", args[1])
		os.Exit(1)

	default:
		fmt.Fprintf(os.Stderr, "Unknown plugin command: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "Usage: tau plugin <list|install|remove|info> [args]\n")
		os.Exit(1)
	}
}
