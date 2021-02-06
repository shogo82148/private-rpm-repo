#!/bin/bash

ROOT=$(cd "$(dirname "$0")" && pwd)

aws cloudformation deploy \
    --region ap-northeast-1 \
    --stack-name "rpm-repository" \
    --template-file "${ROOT}/template.yaml" \
    --parameter-overrides "Environment=${APP_ENV}"
