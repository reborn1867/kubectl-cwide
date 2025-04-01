/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"flag"
	"os"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/kubectl-cwide/pkg/cmd"
	"github.com/kubectl-cwide/pkg/cmd/get"
	"github.com/kubectl-cwide/pkg/cmd/initialization"
)

var (
	templatePath string
	kubeconfig   string
)

func init() {
	flag.StringVar(&templatePath, "template-path", "./cwide", "Path to the template file")
}

func main() {
	flag.Parse()

	root := cmd.NewCmdCwide(genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})

	root.AddCommand(initialization.InitCMD)
	root.AddCommand(get.GetCMD)
	root.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
