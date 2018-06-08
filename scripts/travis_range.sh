#!/usr/bin/env bash
#
if [[ "${TRAVIS}" != "true" ]]; then
  echo "FATAL: This should NOT be run outside Travis!"
  exit 1
fi

if [[ "${TRAVIS_PULL_REQUEST}" == "true" ]]; then
  echo ${TRAVIS_BRANCH}...HEAD
else
  echo ${TRAVIS_COMMIT_RANGE}
fi
