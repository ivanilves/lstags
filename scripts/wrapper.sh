#!/usr/bin/env bash
#
[[ "${DEBUG}" == "true" || "${DEBUG}" == "yes" ]] && set -x

set -uo pipefail

declare -r -i CACHE_TTL_MINUTES=2880

declare -r LOCAL_PATH=${HOME}/.lstags; mkdir -p "${LOCAL_PATH}" || exit 2
declare -r LSTAGS=${LOCAL_PATH}/lstags
declare -r UPDATE_MARKER="${LOCAL_PATH}/update"

declare -r LATEST_RELEASE_URL=https://api.github.com/repos/ivanilves/lstags/releases/latest
declare -r DOWNLOAD_URL=https://github.com/ivanilves/lstags/releases/download

CHECK_FOR_UPDATE=false
if [[ ! -x "${LSTAGS}" || ! -f "${UPDATE_MARKER}" ]]; then
  CHECK_FOR_UPDATE=true
elif [[ -n "$(find ${UPDATE_MARKER} -type f -cmin +${CACHE_TTL_MINUTES})" ]]; then
  CHECK_FOR_UPDATE=true
fi

PERFORM_UPDATE=false
if [[ "${CHECK_FOR_UPDATE}" == "true" ]]; then
  declare -r LATEST_VERSION=$(curl --connect-timeout 5 -m10 -s -f -H "Content-Type:application/json" "${LATEST_RELEASE_URL}" | jq -r '.name')
  declare -r LATEST_TAG=${LATEST_VERSION/-*/}
  declare -r LATEST_VERSION_URL=${DOWNLOAD_URL}/${LATEST_TAG}/lstags-$(uname -s | tr [A-Z] [a-z])-${LATEST_VERSION}.tar.gz

  if [[ -x "${LSTAGS}" && -n "${LATEST_VERSION}" ]]; then
    LOCAL_VERSION=$(${LSTAGS} --version | cut -d' ' -f2)
    if [[ "${LOCAL_VERSION}" != "${LATEST_VERSION}" ]]; then
      echo "Local version: ${LOCAL_VERSION} / Will download new one: ${LATEST_VERSION}"
      PERFORM_UPDATE=true
    else
      echo "You already have the latest version: ${LOCAL_VERSION}"
    fi
  elif [[ -n "${LATEST_VERSION}" ]]; then
    echo "No binary found, will download latest version: ${LATEST_VERSION}"
    PERFORM_UPDATE=true
  else
    echo "Failed to check for update!" >>/dev/stderr
  fi

  echo '-'
fi

if [[ "${PERFORM_UPDATE}" == "true" ]]; then
  curl --connect-timeout 5 -m30 -s -f "${LATEST_VERSION_URL}" -L | tar -C "${LOCAL_PATH}" -xz
fi

touch "${UPDATE_MARKER}"

exec ${LSTAGS} ${@}
