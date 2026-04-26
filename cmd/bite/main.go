// Command bite is the entry point for the bite CLI.
//
// All command logic lives under internal/cli — this file is intentionally thin
// so future packaging (manpages, plugins, embed) has one obvious place to extend.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/nicksan222/bite/internal/cli"
)

// Set by GoReleaser at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	cli.SetBuildInfo(version, commit)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Execute(ctx); err != nil {
		os.Exit(1)
	}
}
