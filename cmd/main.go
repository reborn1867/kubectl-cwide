package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/kubectl-cwide/pkg/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	streams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	root := cmd.NewCmdCwide(streams)
	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
