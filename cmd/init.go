package cmd

import (
	"github.com/myhelix/terraform-experimental/terracanary/config"
	"github.com/spf13/cobra"
)

func init() {
	var bucket, key, region string

	initCmd := &cobra.Command{
		Use: "init <flags>... [-- <terraform-flags>...]",
		DisableFlagsInUseLine: true,
		Short: "Set args that will be passed to 'terraform init'",
		Long: `This persistently configures terracanary to pass a set of arguments through to terraform when running 'terraform init' for any stack. The supplied arguments MUST include the state file bucket, key, and region; those values are intercepted by terracanary and used to manage the various statefiles for the different stacks and stack versions (all of which will used the supplied key as a common prefix). Any additional arguments are passed through directly to 'terraform init'. This must be run before any other terracanary commands, and will generate a fresh '.terracanary' config file in the working directory.

Running the following terracanary commands:

	terracanary init                         \
		--bucket=my-state-bucket             \
		--key=my-state-path/filename         \
		--region=us-east-1                   \
		--                                   \
		-get-plugins=false

	terracanary args -- -var-file=somepath/input.tfvars -var name=foo -var environment=development

	terracanary plan -S shared
	terracanary apply -S shared

Is the equivalent of running:

	cd shared

	terraform init                                          \
		-backend-config="bucket=my-state-bucket"            \
		-backend-config="key=my-state-path/filename-shared" \
		-backend-config="region=us-east-1"                  \
		-get-plugins=false

	terraform plan -var-file=somepath/input.tfvars -var name=foo -var environment=development
	terraform apply -var-file=somepath/input.tfvars -var name=foo -var environment=development`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Override root; don't try to read config
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Clear out and start with defaults
			config.Initialize()
			config.Global.StateFileBucket = bucket
			config.Global.StateFileBase = key
			config.Global.AWSRegion = region
			config.Global.InitArgs = args

			exitIf(config.Write())
		},
	}
	initCmd.Flags().StringVar(&bucket, "bucket", "", "State file bucket (required)")
	initCmd.Flags().StringVar(&key, "key", "", "State file path/name (required)")
	initCmd.Flags().StringVar(&region, "region", "", "Region to access bucket in (required)")
	initCmd.MarkFlagRequired("bucket")
	initCmd.MarkFlagRequired("key")
	initCmd.MarkFlagRequired("region")

	RootCmd.AddCommand(initCmd)
}
