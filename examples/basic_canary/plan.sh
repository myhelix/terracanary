#!/usr/bin/env bash
set -e

. ../plan_helpers.sh

NEXT_VERSION=$(terracanary next)

CURRENT_MAIN="main:$(terracanary output -S routing current_stack_version)" || CURRENT_MAIN=""
if [[ $CURRENT_MAIN ]]; then
    echo "Showing changes to existing main stack; note that deploy.sh will actually apply these changes by creating a new stack version."
    begin main
    run terracanary plan -s $CURRENT_MAIN $MAIN_INPUTS
    end main
else
    echo "No currently deployed main stack; will build a new one."
    begin main
    run terracanary plan -s main:$NEXT_VERSION $MAIN_INPUTS
    end main
fi

ROUTING_INPUTS="-i $CURRENT_MAIN:current -i $CURRENT_MAIN:next"
if [[ $CURRENT_MAIN ]]; then
    echo "Showing changes to routing assuming no new main stack version or canary deployment; actual deployment may use different versions."
    begin routing
    run terracanary plan -S routing $ROUTING_INPUTS
    end routing
else
    echo "Can't plan routing stack until there is a main stack."
fi

