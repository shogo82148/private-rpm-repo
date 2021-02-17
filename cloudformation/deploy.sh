#!/bin/bash

ROOT=$(cd "$(dirname "$0")" && pwd)

set -xue

sam deploy \
    --region ap-northeast-1 \
    --stack-name "rpm-repository-ecr" \
    --no-fail-on-empty-changeset \
    --template-file "${ROOT}/template.yaml"

sam deploy \
    --region ap-northeast-1 \
    --stack-name "rpm-repository-users" \
    --no-fail-on-empty-changeset \
    --capabilities CAPABILITY_IAM \
    --template-file "${ROOT}/users-template.yaml"
