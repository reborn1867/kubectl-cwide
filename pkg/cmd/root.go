package cmd

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericiooptions"
)

func NewCmdCwide(ioStreams genericiooptions.IOStreams) *cobra.Command {
	command := &cobra.Command{
		Use:   "kubectl cwide",
		Short: "Custom wide output format",
		RunE: func(c *cobra.Command, args []string) error {
			c.Usage()
			return nil
		},
	}
	return command
}
