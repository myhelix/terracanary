package cmd

import (
	"bufio"
	"fmt"
	"github.com/myhelix/terracanary/canarrors"
	"github.com/myhelix/terracanary/stacks"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
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

Unless --skip-confirmation is specified, terracanary will prompt for interactive confirmation if the destroy command would remove all versions of any currently existing stack (this means it always prompts for destruction of non-versioned stacks).

Because it's very common for the first attempt at destroying a complex stack to fail due to ordering issues, terracanary will automatically retry once if resources are left over after the first destroy. If a stack requested for destruction still has resources remaining after 2 attempts, terracanary will continue to process other stacks requested for destruction, but will exit with code ` + canarrors.IncompleteDestruction.ExitCodeString() + ` at the end. Unexpected failures will exit immediately with various other codes.`,
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
				if force == "" {
					canarrors.ExitWith(fmt.Errorf("Must specify --force when destroying legacy stack."))
				}
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

				if force == "" {
					log.Println("Attempting to force destruction using blank config.")

					destroyPlayground, err := ioutil.TempDir("", "terracanary-destroy")
					exitIf(err)
					defer os.RemoveAll(destroyPlayground) // clean up

					// We need a basic config with provider definitions to accomplish our destruction
					// If terraform doesn't have a provider, it will just ignore the resources in
					// the state file, and think it actually did destroy everything despite doing
					// nothing.
					err = exec.Command("cp", force, destroyPlayground).Run()
					exitIf(err)

					stack.WorkingDirectory = destroyPlayground
				}

				// Remove stuff from state that we don't want to destroy
				err = stack.RemoveFromState(leave)
				exitIf(err)

				doDestroy := func() {
					err = stack.Destroy(inputStacks, args...)
					if err != nil && !canarrors.Is(err, canarrors.IncompleteDestruction) {
						// Unexpected failure; exit immediately
						exitWith(err)
					}
				}
				doDestroy()
				if err != nil {
					log.Println("Retrying destroy of:", stack)
					doDestroy()
				}
				if err != nil {
					// If destruction failed in an expected way, keep going (but exit non-0 eventually)
					// Unexpected errors were checked for above in doDestroy()
					anyFailure = err
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
