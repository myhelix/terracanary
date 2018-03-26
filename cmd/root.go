package cmd

import (
	"fmt"
	"os"

	"github.com/myhelix/terracanary/config"
	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "terracanary",
	Short: "Deployment orchestration using terraform",
	Long:  `Terracanary provides a wrapper for terraform that manages multiple versions of terraform stacks and facilitates sharing data between multiple related stacks. This allows you to easily construct complex deployment procedures.`,
	Example: `# Apply database infrastructure updates
terracanary apply --stack database
# Run database migrations
...
# Build new copy of infrastructure, using shared database
NEW_VERSION=$(terracanary next)
terracanary apply --stack-version main:$NEW_VERSION --input-stack database
# Run some automated tests on new main stack
HOSTNAME_TO_TEST=$(terracanary output --stack-version main:$NEW_VERSION hostname)
...
# Start sending traffic to new main stack
terracanary apply --stack routing --input-stack-version main:$NEW_VERSION
# Clean up old stack(s)
terracanary destroy --all main --except main:$NEW_VERSION`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return config.Read()
	},
}

func init() {
	RootCmd.SetHelpTemplate(`Description:

{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}
{{end}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
