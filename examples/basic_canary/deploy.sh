#!/usr/bin/env bash
# This script illustrates a basic canary deploy strategy for a project divided into two stacks:
#   main (versioned):
#       - all resources except DNS
#   routing (not versioned):
#       - DNS records (or any other service discovery mechanism)
#
# The routing stack takes two different versions of the main stack as inputs, using input stack
# aliases. It is statically configured to use a weighted record set so that (for example) 90% of
# traffic goes to the "current" input stack and 10% goes to the "next" input stack.
#
# This script doesn't include doing any testing of the results of the canary deployment; it can
# just be run in two modes by an external system/human, to either send all traffic to a new
# code version or to split traffic between a new code version and whatever was previously running.
#
# The specifics of where to find the new code artifacts to deploy are assumed to have been configured
# earlier in the environment or via "terracanary args".

set -ex

# Get currently deployed stack number
CURRENT="main:$(terracanary output -S routing current_stack_version)" || CURRENT=""

# Get next completely unused version number, and build new stack
NEW="main:$(terracanary next)"
terracanary apply -s $NEW

case $DEPLOY_TYPE in
canary)
    if [[ ! $CURRENT ]]; then
        echo "Can't perform canary deploy without a currently deployed stack."
        exit 1
    fi

    # Apply new stack for testing
    terracanary apply -S routing -i $CURRENT:current -i $NEW:next
    ;;
full)
    # Apply the new stack as current and next (100% combined traffic)
    terracanary apply -S routing -i $NEW:current -i $NEW:next
    ;;
*)
    echo "Unknown DEPLOY_TYPE."
    exit 1
    ;;
esac

# Destroy except for new stack and current stack (if any)
terracanary destroy -a main ${CURRENT:+-e $CURRENT} -e $NEW

echo "Success!"
