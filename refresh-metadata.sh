#!/bin/bash

set -uex

ROOT=$(cd "$(dirname "$0")" && pwd)
IMAGE_NAME="createrepo-$$"

cd "$ROOT"
docker build -t "$IMAGE_NAME" .

mkdir -p "$ROOT/.working"

cd "$ROOT/.working"
aws s3 sync --delete --exclude "*" --include "*.rpm" s3://shogo82148-rpm-repository .

for distribution in */*/*; do
    docker run --rm -it -v "$ROOT/.working/$distribution:/repo" "$IMAGE_NAME" createrepo /repo
done

aws s3 sync --acl public-read --exclude "*.rpm" . s3://shogo82148-rpm-repository
