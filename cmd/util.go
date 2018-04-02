package cmd

import (
	"github.com/myhelix/terracanary/cmd/util/aws"
	"github.com/spf13/cobra"
)

func init() {
	var utilCmd = &cobra.Command{
		Use:   "util",
		Short: "General utilities to help deployment scripts",
	}
	utilCmd.AddCommand(aws.RootCmd)
	RootCmd.AddCommand(utilCmd)
}
