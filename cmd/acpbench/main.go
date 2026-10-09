package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	command "github.com/baldaworks/acpbench/cmd/acpbench/cmd"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := command.Command()
	return cmd.ExecuteContext(ctx)
}
