package cmd

import (
	"github.com/myhelix/terracanary/config"
	"github.com/spf13/cobra"
)

func init() {
	var argsCmd = &cobra.Command{
		Use: "args -- <terraform-flags>...",
		DisableFlagsInUseLine: true,
		Short: "Set args that will be passed to terraform for plan/apply/destroy",
		Long:  `This persistently configures terracanary to pass a set of arbitrary arguments through to terraform when running commands that require input variables (plan, apply, destroy). It should be run after 'terracanary init' but before any other terracanary commands; see the init help for a complete example.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			for _, arg := range args {
				config.Global.TerraformArgs = append(config.Global.TerraformArgs, arg)
			}
			config.Write()
		},
	}

	RootCmd.AddCommand(argsCmd)
}
