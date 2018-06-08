#!/usr/bin/env bash
#
declare MODES="silent|verbose"

if [[ ${#} -ne 3 ]]; then
  echo "Usage: ${0} NAME TIME ${MODES}"
  exit 1
fi

declare -r NAME=${1}
declare -r TIME=${2}
declare -r MODE=${3}

if [[ ! ${MODE} =~ ${MODES} ]]; then
  echo "FATAL: Invalid mode: ${MODE} (could be \"${MODES}\")"
  exit 1
fi

if [[ ! -f ${NAME}.pid ]]; then
  echo 'FATAL: nothing to wait for!'
  exit 1
fi

declare -r W_PID=$(cat ${NAME}.pid)
RUN_TIME=0
while [[ -e /proc/${W_PID} ]]; do
  if [[ ${RUN_TIME} -gt ${TIME} ]]; then
    echo; cat ${NAME}.log
    echo "ERROR: Timeout reached: ${TIME} seconds"
    exit 124
  fi

  sleep 1

  echo -n .
  ((RUN_TIME++))
done

if [[ ${MODE} == "verbose" || ! -f ${NAME}.success ]]; then
  echo; cat ${NAME}.log
fi

STATUS=0; [[ -f ${NAME}.success ]] || STATUS=1

rm -f ${NAME}.pid ${NAME}.log ${NAME}.success

exit ${STATUS}
