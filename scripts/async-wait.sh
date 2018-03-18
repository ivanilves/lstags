#!/usr/bin/env bash
#
if [[ ${#} -ne 2 ]]; then
  echo "Usage: ${0} NAME TIME"
  exit 1
fi

declare -r NAME=${1}
declare -r TIME=${2}

if [[ ! -f ${NAME}.pid ]]; then
  echo 'FATAL: nothing to wait for!'
  exit 1
fi

declare -r W_PID=$(cat ${NAME}.pid)
RUN_TIME=0
while [[ -e /proc/${W_PID} ]]; do
  if [[ ${RUN_TIME} -gt ${TIME} ]]; then
    echo; cat ${NAME}.log
    echo "Timeout reached: ${TIME} seconds"
    exit 124
  fi

  sleep 1

  echo -n .
  ((RUN_TIME++))
done

cat ${NAME}.log

rm -f ${NAME}.pid
