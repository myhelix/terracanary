package aws

import (
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS-related utilities",
}
