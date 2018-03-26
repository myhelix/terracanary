package cmd

import (
	"fmt"
	"github.com/myhelix/terracanary/stacks"
	"github.com/spf13/cobra"
)

func init() {
	//TODO: Support all flags from destroy for selecting stacks
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List all stacks",
		Long:  `Outputs a list of stacks with existent state files, one per line, ordered by version.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			all, err := stacks.All("")
			exitIf(err)

			for _, s := range all {
				fmt.Println(s)
			}
		},
	}

	RootCmd.AddCommand(listCmd)
}
