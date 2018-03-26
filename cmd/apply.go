package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	var applyCmd = &cobra.Command{
		Use: "apply" + singleStackUsage + passThroughUsage,
		DisableFlagsInUseLine: true,
		Short: "Apply changes to a stack",
		Long: `Initialize the selected stack and perform 'terraform apply' on it. You may specify other stacks from which to provide the location of the state files as inputs, allowing coordination between specified versions of different stacks. This works by automatically supplying the configuration for terraform_remote_state resources that reference other stacks. For instance, the terraform definition for a stack with inputs of a versioned "code" stack and an unversioned "database" stack would look like:

	variable "code_stack_version" {}

	variable "code_stack_state" {
	  type        = "map"
	}

	data "terraform_remote_state" "code" {
	  backend = "s3"
	  config  = "${var.code_stack_state}"
	}

	variable "database_stack_state" {
	  type        = "map"
	}

	data "terraform_remote_state" "database" {
	  backend = "s3"
	  config  = "${var.database_stack_state}"
	}

Note that two input variables are provided for each input stack -- a _stack_state variable that can be passed directly to terraform_remote_state as the config, and a _stack_version variable (for versioned stacks only), that's just the integer version number for that stack. The stack version inputs are mostly useful to provide as outputs to allow interrogating the deployed state.

For versioned stacks, you may also supply an alias, which will be used as the prefix for the input variables instead of the stack name. This allows passing different versions of the same stack in with different names (e.g. "stable" and "testing" stack versions during a canary deployment).
`,
		Example: `terracanary apply -S database
terracanary apply -s code:$CODE_VERSION
terracanary apply -s main:$MAIN_VERSION -I database -i code:$CODE_VERSION`,
		Run: func(cmd *cobra.Command, args []string) {
			passThroughCommand(cmd, "apply", args)
		},
	}

	takesSingleStack(applyCmd)
	takesInputStacks(applyCmd)
	RootCmd.AddCommand(applyCmd)
}
