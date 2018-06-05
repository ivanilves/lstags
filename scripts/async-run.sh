#!/usr/bin/env bash
#
if [[ ${#} -lt 2 ]]; then
  echo "Usage: ${0} NAME COMMAND [OPTIONS]"
  exit 1
fi

declare -r NAME=${1}; shift
declare -r EXEC=${@}

if [[ -f ${NAME}.pid ]]; then
  echo "FATAL: ${NAME}.pid file already exists!"
  exit 1
fi

rm -f ${NAME}.success

bash -c "${EXEC} && touch ${NAME}.success" &>${NAME}.log &
echo ${!} >${NAME}.pid
