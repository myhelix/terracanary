package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

func init() {
	var outputCmd = &cobra.Command{
		Use: "output" + singleStackUsage + " <output-name>...",
		DisableFlagsInUseLine: true,
		Short: "Retrieve terraform outputs from specified stack",
		Long: `Outputs a newline-separated list of terraform output values for the specified stack.

For example:

	terracanary output -s main:5 deployed_task_arn log_group

Is equivalent to:

	(
		cd main
		terraform init -backend-config=key=<STATE_FILE_PATH>-main-5 ...
		terraform output deployed_task_arn
		terraform output log_group
	)`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			stack := parseSingleStack(cmd)
			output, err := stack.Outputs(args...)
			exitIf(err)
			fmt.Println(strings.Join(output, " "))
		},
	}

	takesSingleStack(outputCmd)
	RootCmd.AddCommand(outputCmd)
}
