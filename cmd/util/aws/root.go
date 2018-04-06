package aws

import (
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS-related utilities",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Override root; don't try to read config
	},
}
