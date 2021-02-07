#!/bin/bash

set -uex

ROOT=$(cd "$(dirname "$0")" && pwd)

cd "$ROOT"
createrepo packages/amazonlinux/2/x86_64
aws s3 sync --acl=public-read packages/ s3://shogo82148-rpm-repository/
