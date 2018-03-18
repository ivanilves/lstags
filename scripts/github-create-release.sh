#!/usr/bin/env bash
#
declare -r API_URL=https://api.github.com/repos/ivanilves/lstags/releases

if [[ ${#} -ne 1 ]]; then
  echo "Create release on GitHub using existing generated artifacts"
  echo "Usage: ${0} RELEASE_PATH" >>/dev/stderr
  exit 1
fi

if [[ -z "${GITHUB_TOKEN}" ]]; then
  echo "GITHUB_TOKEN not set" >>/dev/stderr
  exit 1
fi

set -euo pipefail

declare -r RELEASE_PATH="${1}"

pushd "${RELEASE_PATH}"
  declare -r TAG="$(cat TAG)"
  declare -r NAME="$(cat NAME)"
  declare -r BODY="$(sed 's/$/\\n/' CHANGELOG.md | tr -d '\n' | sed 's/\"/\\"/g')"
  declare -r DATA="{\"tag_name\":\"${TAG}\", \"target_commitish\": \"master\",\"name\": \"${NAME}\", \"body\": \"${BODY}\", \"draft\": false, \"prerelease\": false, \"prerelease\": true}"

  curl -f -X POST -H "Content-Type:application/json" \
    -H "Authorization: Token ${GITHUB_TOKEN}" "${API_URL}" \
    -d "${DATA}"
popd
