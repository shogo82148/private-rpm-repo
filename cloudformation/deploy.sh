#!/bin/bash

ROOT=$(cd "$(dirname "$0")" && pwd)

set -xue

sam deploy \
    --region ap-northeast-1 \
    --stack-name "rpm-repository" \
    --template-file "${ROOT}/template.yaml"
