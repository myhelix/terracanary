#!/usr/bin/env bash
set -e

. ../plan_helpers.sh

begin shared
run terracanary plan -S shared
end shared

if ! terracanary list | grep -q shared; then
    echo "Can't plan anything except shared stack until shared stack is built."
    exit
fi

NEXT_VERSION=$(terracanary next)

begin code
run terracanary plan -s code:$NEXT_VERSION -I shared
end code

CODE=$(terracanary list | grep -E '^code:[0-9]+$' | tail -n 1)
if [[ ! $CODE ]]; then
    echo "Can't plan main stack until a code stack is built."
    exit
fi

CURRENT_MAIN="main:$(terracanary output -S routing main_stack_version)" || CURRENT_MAIN=""
MAIN_INPUTS="-i $CODE -I shared"

if [[ $CURRENT_MAIN ]]; then
    echo "Showing changes to existing main stack; note that deploy.sh may actually apply these changes by creating a new stack version."
    begin main
    run terracanary plan -s $CURRENT_MAIN $MAIN_INPUTS
    end main
else
    echo "No currently deployed main stack; will build a new one."
    begin main
    run terracanary plan -s main:$NEXT_VERSION $MAIN_INPUTS
    end main
fi

if [[ $CURRENT_MAIN ]]; then
    echo "Showing changes to routing assuming no new main stack version; actual deployment may change main stack version."
    begin routing
    run terracanary plan -S routing -i $CURRENT_MAIN
    end routing
else
    echo "Can't plan routing stack until there is a main stack."
fi

