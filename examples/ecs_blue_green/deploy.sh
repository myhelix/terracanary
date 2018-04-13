#!/usr/bin/env bash
# This script illustrates a full deployment strategy for complex ECS projects. The stack layout
# is as follows:
#
#   shared (not versioned):
#       - log groups
#       - s3 buckets
#       - RDS databases
#       - any other persistent resources
#   code (versioned):
#       - task definitions
#   main (versioned):
#       - ECS cluster
#       - ECS service(s)
#       - ASG and launch config for ECS container instances (if not using Fargate)
#       - ELB/ALB
#   routing (not versioned):
#       - Route53 entries
#
# The script can deploy in two modes; if there are not significant changes to the main stack, i.e.
# we're just deploying a new code rev, it will create a new code stack, run test/migration tasks,
# and then update the existing service to the new task definition revision.
#
# If there ARE changes to the main stack definitions, the script will build a new main stack, test
# through its new load balancer that it's working properly, then rerouting DNS to the new stack,
# and finally clean up the old one.

set -ex

. ../deploy_helpers.sh

# Update shared stack; contains resources needed by both main and code stacks
terracanary apply -S shared

# Get next completely unused version number, and build new code stack
NEW_VERSION=$(terracanary next)
CODE="code:$NEW_VERSION"
CODE_INPUTS="-I shared"
terracanary apply -s $CODE $CODE_INPUTS

# Find current main stack (if any)
MAIN="main:$(terracanary output -S routing main_stack_version)" || MAIN=""
MAIN_INPUTS="-i $CODE -I shared"

# Is this a code-only update? Test if we can do a shortcut deploy.
if [[ $MAIN ]]; then
    ret=0
    # Allow updates to ECS service (for code changes)
    terracanary test -s $MAIN $MAIN_INPUTS -u aws_ecs_service.default || ret=$?
else
    ret=15
fi

if [[ $ret = 0 ]]; then
    # Shortcut deploy is safe

    # Run DB migrations
    run_migrations
    # Check that new code will run properly
    deploy_check_task

    # Upgrade stack to new code revision
    terracanary apply -s $MAIN $MAIN_INPUTS

    # Wait for task transition to complete
    wait_for_tasks

    # Apply any updates to routing stack, even though we're not routing anywhere new
    terracanary apply -S routing -i $MAIN

elif [[ $ret = 15 ]]; then
    # 15 means the plan worked, but wasn't safe to apply; proceed to build a new main stack
    MAIN="main:$NEW_VERSION"
    terracanary apply -s $MAIN $MAIN_INPUTS

    # Wait for all instances to join cluster, to ensure there's space for our migration task
    wait_for_instances
    # Run DB migrations
    run_migrations
    # Check that new code will run properly
    deploy_check_task

    # Wait for all tasks to start up
    wait_for_tasks

    # Check through load balancer
    target=$(terracanary output -s $MAIN load_balancer_dns_name)
    proto=$(terracanary output -s $MAIN lb_protocol)
    deploy_check $target $proto

    # Route to new stack
    terracanary apply -S routing -i $MAIN
    # Wait for routing to stabilize
    sleep 90

else
    echo "Plan failed."
    exit 1
fi

# Shared final check/cleanup
target=$(terracanary output -S routing external_fqdn)
proto=$(terracanary output -s $MAIN lb_protocol)
deploy_check $target $proto

terracanary destroy -a main $MAIN_INPUTS -e $MAIN
# Inactivating task definitions is more of a pain than a help, so avoid "destroy"ing them
terracanary destroy -a code $CODE_INPUTS -e $CODE -l module.task_definition.aws_ecs_task_definition.default

# Clean up any legacy stack; for projects migrating into terracanary
# Assumes persistent resources aside from DNS weren't previously included in project terraform
terracanary destroy --legacy --skip-confirmation -f main/providers.tf \
                                -l module.task_definition.aws_ecs_task_definition.default \
			                    -l aws_route53_record.default

echo "Success!"

