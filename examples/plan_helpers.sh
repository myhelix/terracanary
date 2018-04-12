# Because terracanary deploys may be multi-step processes, there isn't a single simple meaning to
# "show me the plan"; often subsequent steps can't be planned until prior steps are applied.
# These helpers provide some basic formatting for multi-step plan output.

function sep {
    echo
    echo "#################### $1 ####################"
    echo
}

function begin {
    sep "BEGIN: plan for '$1' stack"
}

function end {
    sep "END: plan for '$1' stack"
}

function run {
    echo "$@"
    "$@"
}


