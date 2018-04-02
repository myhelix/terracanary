package aws

import (
	"github.com/spf13/cobra"
)

var ecsCmd = &cobra.Command{
	Use:   "ecs",
	Short: "Utilities related to Elastic Container Service",
}

func init() {
	RootCmd.AddCommand(ecsCmd)
}
