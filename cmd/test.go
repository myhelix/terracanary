package cmd

import (
	"bytes"
	"github.com/hashicorp/terraform/terraform"
	"github.com/myhelix/terraform-experimental/terracanary/canarrors"
	"github.com/spf13/cobra"
	"log"
)

func init() {
	var ignoreUpdate []string

	var testCmd = &cobra.Command{
		Use: "test" + singleStackUsage + passThroughUsage,
		DisableFlagsInUseLine: true,
		Short: "Check if there are any changes to a stack",
		Long: `Uses "terraform plan" to check if any changes are needed for the specified stack. To get accurate results, be sure to include the exact arguments you would specify to "terracanary apply" (e.g. input stacks).

Exit Codes:
	 0 - Success; plan succeeded with no changes
	15 - Plan succeeded, but had changes
	 * - Plan failed due to terraform or other errors`,
		Run: func(cmd *cobra.Command, args []string) {
			stack := parseSingleStack(cmd)
			inputStacks := parseStackArgs(cmd, unversionedInputStacks, versionedInputStacks)

			updateable := make(map[string]bool)
			for _, r := range ignoreUpdate {
				updateable[r] = true
			}

			// See what would happen if we applied to existing stack
			planBytes, err := stack.GeneratePlan(inputStacks, args...)
			exitIf(err)

			plan, err := terraform.ReadPlan(bytes.NewReader(planBytes))
			exitIf(err)

			for _, module := range plan.Diff.Modules {
				for name, resource := range module.Resources {
					switch resource.ChangeType() {
					case terraform.DiffNone:
						continue
					case terraform.DiffUpdate:
						if updateable[name] {
							log.Println("Found allowed update to:", name)
							continue
						}
					}
					exitWith(canarrors.PlanHasChanges.Details("Would change: ", name))
				}
			}
			log.Println("Test plan successful.")
		},
	}

	testCmd.Flags().StringArrayVarP(&ignoreUpdate, "ignore-update", "u", []string{}, "ignore updates to named resource")
	takesSingleStack(testCmd)
	takesInputStacks(testCmd)
	RootCmd.AddCommand(testCmd)
}
