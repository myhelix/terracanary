function deploy_check {
    if [[ "$RUN_DEPLOY_CHECK" ]]; then
        url=$2://$1/deployability
        (
            # Too spammy to show setting errs
            set +x
            # You tell me three times; doesn't cost much to be sure
            for i in 1 2 3; do
                # -f means exit with failure code if request failed
                # -k means don't validate SSL certs (needed when hitting LB directly by LB name)
                # -v outputs headers and stuff to stderr, which we swap with stdout to capture in a variable
                if ! errs=`curl -fkv $url 3>&2 2>&1 1>&3`; then
                    echo "$errs"
                    echo "Failed deploy check."
                    exit 1
                fi
            done
            echo
            echo "Deploy check passed."
        )
    fi
}

function _run_task {
    sleep 30
    region=`terracanary output -s $MAIN region` && \
    cluster=`terracanary output -s $MAIN cluster` && \
    task=`terracanary output -s $CODE task_revision_arn` && \
    terracanary util aws ecs run --region $region --cluster $cluster --task-def $task -- "$@"
}

function run_migrations {
    if [[ "$RUN_MIGRATIONS" ]]; then
        _run_task goose --env $TF_VAR_environment up
    fi
}

function deploy_check_task {
    if [[ "$RUN_DEPLOY_CHECK" ]]; then
        sleep 30
        _run_task scripts/check_deployability.sh
    fi
}

function wait_for_tasks {
    read -r region cluster service <<< $(terracanary output -s $MAIN region cluster service_arn) && \
    terracanary util aws ecs wait --region $region --cluster $cluster --service $service
}

function wait_for_instances {
    read -r region cluster instances <<< $(terracanary output -s $MAIN region cluster expected_instances) && \
    terracanary util aws ecs wait --region $region --cluster $cluster --instances $instances
}

