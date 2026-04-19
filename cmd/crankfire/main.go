package main

import (
	"fmt"
	"os"

	"context"

	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/store"
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
	if len(args) >= 1 && args[0] == "set" {
		dir, err := store.ResolveDataDir("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "data dir: %v\n", err)
			os.Exit(cli.ExitRunnerError)
		}
		st, err := store.NewFS(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "store: %v\n", err)
			os.Exit(cli.ExitRunnerError)
		}
		os.Exit(cli.RunSet(context.Background(), st, args[1:], os.Stdout, os.Stderr))
	}
	if err := cli.Run(args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
