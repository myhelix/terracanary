package cmd

import (
	"github.com/myhelix/terraform-experimental/terracanary/canarrors"
	"github.com/myhelix/terraform-experimental/terracanary/stacks"
	"github.com/spf13/cobra"
	"strings"
)

const passThroughUsage = " [-- <terraform-args>...]"
const singleStackUsage = " (-s <stack>:<version> | -S <stack>) [<flags>...]"

func parseSingleStack(cmd *cobra.Command) stacks.Stack {
	stacks := parseStackArgs(cmd, []string{unversionedStack}, []string{versionedStack})
	if l := len(stacks); l != 1 {
		cmd.Usage()
		exitWith(canarrors.InvalidStack.Details("Command requires 1 stack argument, found ", l))
	}
	return stacks[0]
}

func parseMultipleStacks(cmd *cobra.Command) (ret []stacks.Stack) {
	return parseStackArgs(cmd, unversionedStacks, versionedStacks)
}

func parseStackArgs(cmd *cobra.Command, unversioned, versioned []string) (ret []stacks.Stack) {
	for _, str := range unversioned {
		if str == "" {
			continue
		}
		stack, err := stacks.Parse(str, "")
		if err != nil {
			cmd.Usage()
			exitWith(err)
		}
		ret = append(ret, stack)
	}
	for _, str := range versioned {
		if str == "" {
			continue
		}
		parts := strings.Split(str, ":")
		if len(parts) < 2 || len(parts) > 3 {
			cmd.Usage()
			exitWith(canarrors.InvalidStack.Details("Versioned stack format is '<stack>:<version>[:<alias>]'."))
		}
		stack, err := stacks.Parse(parts[0], parts[1])
		if err != nil {
			cmd.Usage()
			exitWith(err)
		}
		if len(parts) > 2 {
			stack.InputAlias = parts[2]
		}
		ret = append(ret, stack)
	}
	return
}

var unversionedStack string
var versionedStack string
var unversionedStacks []string
var versionedStacks []string
var unversionedInputStacks []string
var versionedInputStacks []string

func takesSingleStack(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&unversionedStack, "stack", "S", "", "Name of unversioned stack to operate on")
	cmd.Flags().StringVarP(&versionedStack, "stack-version", "s", "", "Stack version to operate on as <stack>:<version>")
}

func takesMultipleStacks(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&unversionedStacks, "stack", "S", nil, "Name of unversioned stack to operate on; may repeat argument for multiple stacks")
	cmd.Flags().StringArrayVarP(&versionedStacks, "stack-version", "s", nil, "Stack version to operate on as '<stack>:<version>'; may repeat argument for multiple stacks")
}

func takesInputStacks(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&unversionedInputStacks, "input-stack", "I", nil, "Name of unversioned stack to provide state from as input; may repeat for multiple input stacks")
	cmd.Flags().StringArrayVarP(&versionedInputStacks, "input-stack-version", "i", nil, "Stack version (as <stack>:<version>[:<alias>]) to provide state from as input; may repeat for multiple input stacks")
}

func passThroughCommand(cmd *cobra.Command, action string, args []string) {
	stack := parseSingleStack(cmd)
	inputStacks := parseStackArgs(cmd, unversionedInputStacks, versionedInputStacks)
	err := stack.RunAction(action, inputStacks, args...)
	exitIf(err)
}

var exitIf = canarrors.ExitIf
var exitWith = canarrors.ExitWith
