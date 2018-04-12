#!/usr/bin/env bash
set -e

. ../../plan_helpers.sh

begin shared
run terracanary plan -S shared
end shared

if ! terracanary list | grep -q shared; then
    echo "Can't plan anything except shared stack until shared stack is built."
    exit
fi

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

plan_main

ROUTING_INPUTS="-i $CURRENT_MAIN"
plan_routing

