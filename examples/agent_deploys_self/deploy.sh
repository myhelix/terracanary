#!/usr/bin/env bash
# Not every use of terracanary involves multiple stacks; in this case we use multiple versions of
# a single stack to allow a build agent to deploy itself. In the "deploy" phase, a new set of build
# agents is deployed by the existing agent. Once those register with the CI/CD server, one of the
# new agents runs the following "cleanup" stage in the pipeline, tearing down the old agents.
#
# If the cleanup job gets randomly picked up by an old agent instead of a new agent, the way this
# server works the job would just hang at the point where the old agent shut itself down, and then
# the cleanup stage would get re-run by a new agent and do any remaining cleanup.

set -ex

. ../deploy_helpers.sh

case $PHASE in
deploy)
    # Get next completely unused version number, and build new main stack
    NEW_VERSION=$(terracanary next)
    MAIN="main:$NEW_VERSION"
    terracanary apply -s $MAIN

    # Wait for tasks to start up
    wait_tasks

    # Now our new agents are running, but we're running on an old agent! So don't clean up yet,
    # wait for a subsequent pipeline stage to do cleanup (running on a new agent).
    ;;
cleanup)
    MAIN=$(terracanary list | grep 'main:' | tail -n 1)
    if [[ ! $MAIN ]]; then
        echo "Can't cleanup, no main stack."
        exit 1
    fi

    terracanary destroy -a main -e $MAIN
    ;;
*)
    echo "Unknown PHASE."
    exit 1
    ;;
esac

echo "Success!"
