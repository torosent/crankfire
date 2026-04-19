package main

import (
	"fmt"
	"os"

	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/tui"
)

func main() {
	args := os.Args[1:]
	if len(args) >= 1 && args[0] == "tui" {
		var dataDir string
		rest := args[1:]
		for i := 0; i < len(rest); i++ {
			if rest[i] == "--data-dir" {
				if i+1 >= len(rest) {
					fmt.Fprintln(os.Stderr, "error: --data-dir requires a value")
					os.Exit(1)
				}
				dataDir = rest[i+1]
				i++
			}
		}
		if err := tui.Run(tui.Options{DataDir: dataDir}); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if err := cli.Run(args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
