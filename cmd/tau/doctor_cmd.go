//go:build !no_doctor

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/akzj/tau/core"
)

func doctorCommand(args []string) {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}

	configPath := ""
	if len(args) > 0 && args[0] != "--json" {
		configPath = args[0]
	}

	doctor := core.NewDoctor(configPath)
	results := doctor.RunAll()

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
		return
	}

	fmt.Println("tau doctor — System Diagnostics")
	fmt.Println("================================")
	for _, r := range results {
		icon := "✅"
		switch r.Status {
		case "warn":
			icon = "⚠️"
		case "error":
			icon = "❌"
		case "skipped":
			icon = "⏭️"
		}
		fmt.Printf("%s %-15s %s\n", icon, r.Name, r.Message)
	}
}
