package cmd

import (
	"fmt"
	"github.com/myhelix/terracanary/stacks"
	"github.com/spf13/cobra"
)

func init() {
	// TODO: Support filtering like destroy/list?
	var nextCmd = &cobra.Command{
		Use:   "next",
		Short: "Output next unused version number (across all stacks)",
		Long:  `Outputs the next version number not currently used by any stack.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			next, err := stacks.Next("")
			exitIf(err)

			fmt.Println(next)
		},
	}

	RootCmd.AddCommand(nextCmd)
}
