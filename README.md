## terracanary

Deployment orchestration using terraform

### Synopsis

Terracanary provides a wrapper for terraform that manages multiple versions of terraform stacks and facilitates sharing data between multiple related stacks. This allows you to easily construct complex deployment procedures.

### Examples

```
# Apply database infrastructure updates
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
terracanary destroy --all main --except main:$NEW_VERSION
```

#### Directory Layout

Terracanary must be run from a working directory which has subdirectories containing the terraform definitions for each stack. Terracanary will create a ".terracanary" config file when you run "terracanary init" to set up the working directory. So for the above example, the layout would look like:

```
.
|-- database
|   \-- *.tf
|
|-- main
|   \-- *.tf
|
|-- routing
|   \-- *.tf
|
\-- .terracanary
```

### Options

```
  -h, --help   help for terracanary
```

### SEE ALSO

* [terracanary apply](docs/terracanary_apply.md)	 - Apply changes to a stack
* [terracanary args](docs/terracanary_args.md)	 - Set args that will be passed to terraform for plan/apply/destroy
* [terracanary destroy](docs/terracanary_destroy.md)	 - Destroys one or more stacks
* [terracanary init](docs/terracanary_init.md)	 - Set args that will be passed to 'terraform init'
* [terracanary list](docs/terracanary_list.md)	 - List all stacks
* [terracanary next](docs/terracanary_next.md)	 - Output next unused version number (across all stacks)
* [terracanary output](docs/terracanary_output.md)	 - Retrieve terraform outputs from specified stack
* [terracanary plan](docs/terracanary_plan.md)	 - Plan changes to a stack
* [terracanary test](docs/terracanary_test.md)	 - Check if there are any changes to a stack
* [terracanary util](docs/terracanary_util.md)	 - General utilities to help deployment scripts

###### Auto generated by spf13/cobra on 6-Apr-2018
