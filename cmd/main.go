package main

import (
	"os"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/kubectl-cwide/pkg/cmd"
)

func main() {
	streams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	root := cmd.NewCmdCwide(streams)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
