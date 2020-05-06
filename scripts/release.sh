#!/bin/bash
set -x

if [[ "$CIRCLE_PR_NUMBER" != "" ]]; then
  echo Skipping release of a PR build
  circleci-agent step halt
  exit 0
fi

make ci-release IMG=flanksource/platform-operator TAG=$(date +%Y%m%d%M%H%M%S)
