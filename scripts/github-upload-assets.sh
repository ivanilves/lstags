#!/usr/bin/env bash
#
declare -r API_URL=https://api.github.com/repos/ivanilves/lstags/releases
declare -r UPLOAD_URL=https://uploads.github.com/repos/ivanilves/lstags/releases

if [[ ${#} -ne 2 ]]; then
  echo "Compress and upload built assets to the GitHub"
  echo "Usage: ${0} TAG ASSETS_PATH" >>/dev/stderr
  exit 1
fi

if [[ -z "${GITHUB_TOKEN}" ]]; then
  echo "GITHUB_TOKEN not set" >>/dev/stderr
  exit 1
fi

set -euo pipefail

declare -r TAG="${1}"
declare -r ASSETS_PATH="${2}"

ID=$(curl -s -H "Authorization: Token ${GITHUB_TOKEN}" "${API_URL}/tags/${TAG}" | jq ".id")

pushd "${ASSETS_PATH}"
  for DIR in $(find -mindepth 1 -maxdepth 1 -type d); do
    tar -C "${DIR}" -zc . -f "${DIR}-$(cat ../release/NAME).tar.gz"
  done

  for FILE in $(find -mindepth 1 -maxdepth 1 -type f -name "*.tar.gz"); do
    curl -H "Content-Type: $(file -b --mime-type ${FILE})" \
      -H "Authorization: Token ${GITHUB_TOKEN}" \
      --data-binary @${FILE} \
      "${UPLOAD_URL}/${ID}/assets?name=$(basename ${FILE})"
    echo
  done
popd
