package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	var planCmd = &cobra.Command{
		Use: "plan" + singleStackUsage + passThroughUsage,
		DisableFlagsInUseLine: true,
		Short: "Plan changes to a stack",
		Long:  `Runs "terraform plan" on the specified stack, displaying the output on stderr. To get accurate results, be sure to include the exact arguments you would specify to "terracanary apply" (e.g. input stacks).`,
		Run: func(cmd *cobra.Command, args []string) {
			passThroughCommand(cmd, "plan", args)
		},
	}

	takesSingleStack(planCmd)
	takesInputStacks(planCmd)
	RootCmd.AddCommand(planCmd)
}
