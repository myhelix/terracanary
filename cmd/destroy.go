package cmd

import (
	"bufio"
	"fmt"
	"github.com/myhelix/terraform-experimental/terracanary/canarrors"
	"github.com/myhelix/terraform-experimental/terracanary/stacks"
	"github.com/spf13/cobra"
	"log"
	"os"
	"strings"
)

func requireConfirmation() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintln(os.Stderr, "WARNING! Some stacks will be completely destroyed (all versions) by this action. Are you sure?")
	fmt.Fprintf(os.Stderr, "Type 'yes' to continue: ")
	yes, err := reader.ReadString('\n')
	exitIf(err)
	if strings.TrimSpace(yes) != "yes" {
		fmt.Println("Confirmation failed.")
		os.Exit(1)
	}
}

func init() {
	var destroyAll []string
	var exceptV []string
	var exceptU []string
	var leave []string
	var legacy, everything bool
	var force string
	var skipConfirmation bool

	var destroyCmd = &cobra.Command{
		Use: "destroy <flags>" + passThroughUsage,
		DisableFlagsInUseLine: true,
		Short: "Destroys one or more stacks",
		Long: `Destroys stacks according to the specified flags. Returns success only if everything requested was actually destroyed (or didn't exist to begin with). For a normal destroy, you will need to provide whatever inputs are normally required by the stack via -i/-I.

To bypass terraform definition errors, you can use --force to supply an empty-except-providers definition file to use during destruction. USING THIS OPTION WILL BYPASS THE prevent_destroy DIRECTIVE.

Unless --skip-confirmation is specified, terracanary will prompt for interactive confirmation if the destroy command would remove all versions of any currently existing stack (this means it always prompts for destruction of non-versioned stacks).`,
		Example: `terracanary destroy -s main:4 -i code:5
terracanary destroy -s code:5 -l module.task_definition.aws_ecs_task_definition.default
terracanary destroy -a main -a code -e main:6 -e code:6
terracanary destroy --legacy -l module.ecs_service.aws_route53_record.default
terracanary destroy -s main:4 -f main/providers.tf
terracanary destroy -A -f main/providers.tf --skip-confirmation`,
		Run: func(cmd *cobra.Command, args []string) {
			// May be useful for getting non-force destroy to run happily
			inputStacks := parseStackArgs(cmd, unversionedInputStacks, versionedInputStacks)

			destroyStacks := parseMultipleStacks(cmd)
			skip := parseStackArgs(cmd, exceptU, exceptV)

			for _, s := range destroyAll {
				all, err := stacks.All(s)
				exitIf(err)
				for _, stack := range all {
					destroyStacks = append(destroyStacks, stack)
				}
			}
			if legacy {
				destroyStacks = append(destroyStacks, stacks.Legacy)
			}
			if everything {
				all, err := stacks.All("")
				exitIf(err)
				// Append here to allow future error behavior about destroying non-existent stacks
				// to behave consistently, i.e. if I request all + foo, could return an error that
				// foo doesn't exist.
				destroyStacks = append(destroyStacks, all...)
			}

			if len(skip) > 0 {
				log.Println("Requested stacks:", destroyStacks)
				log.Println("Skipping stacks:", skip)
				destroyStacks = stacks.Subtract(destroyStacks, skip)
			}

			log.Println("Will destroy:", destroyStacks)

			// During normal operations, you wouldn't typically remove ALL versions of a given stack;
			// so check if that will be the case, and if so, ask for interactive confirmation.
			existingStacks, err := stacks.All("")
			exitIf(err)
			leftStacks := stacks.Subtract(existingStacks, destroyStacks)
			log.Println("Stacks that will be left:", leftStacks)
			if !skipConfirmation {
				willHave := make(map[string]bool)
				for _, left := range leftStacks {
					willHave[left.Subdir] = true
				}
				for _, had := range existingStacks {
					if !willHave[had.Subdir] {
						requireConfirmation()
						break
					}
				}
			}

			var anyFailure error
			for _, stack := range destroyStacks {
				exists, err := stack.Exists()
				exitIf(err)
				if !exists {
					log.Println("Skipping nonexistent stack:", stack)
					continue
				}

				// Remove stuff from state that we don't want to destroy
				if len(leave) > 0 {
					leaveResources := make(map[string]bool)
					for _, l := range leave {
						leaveResources[l] = true
					}

					resources, err := stack.StateList()
					exitIf(err)
					for _, r := range resources {
						if leaveResources[r] {
							log.Println("Avoiding destruction of:", r)
							exitIf(stacks.Command{
								Stack:  stack,
								Init:   true,
								Action: "state",
								Args:   []string{"rm", r},
							}.Run())
						} else {
							log.Println("To be destroyed:", r)
						}
					}
				}

				if force == "" {
					err = stack.Destroy(inputStacks, args...)
				} else {
					err = stack.ForceDestroy(force)
				}
				if err != nil {
					// If destruction failed in an expected way, keep going (but exit non-0)
					if canarrors.Is(err, canarrors.IncompleteDestruction) {
						anyFailure = err
					} else {
						exitWith(err)
					}
				}
			}

			exitIf(anyFailure)
		},
	}

	destroyCmd.Flags().StringArrayVarP(&destroyAll, "all", "a", []string{}, "destroy all versions of specified stack; may be repeated for multiple stacks")
	destroyCmd.Flags().StringArrayVarP(&leave, "leave", "l", []string{}, "skip destruction of named resource by removing from state before destroy")
	destroyCmd.Flags().StringVarP(&force, "force", "f", "", "override prevent_destroy and bypass terraform definition/input errors")
	destroyCmd.Flags().BoolVarP(&everything, "everything", "A", false, "destroy ALL stacks")
	destroyCmd.Flags().BoolVar(&legacy, "legacy", false, "destroy legacy stack (contents of base state filename)")
	destroyCmd.Flags().BoolVar(&skipConfirmation, "skip-confirmation", false, "don't ask for interactive confirmation if command would leave no versions of an existing stack")
	destroyCmd.Flags().StringArrayVarP(&exceptU, "except", "E", nil, "skip destroying specified unversioned stack; may repeat")
	destroyCmd.Flags().StringArrayVarP(&exceptV, "except-version", "e", nil, "skip destroying specified stack version; may repeat")

	takesMultipleStacks(destroyCmd)
	takesInputStacks(destroyCmd)

	RootCmd.AddCommand(destroyCmd)
}
