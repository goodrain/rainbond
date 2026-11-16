#!/usr/bin/env bash
# Bound registry operations, including buildx's child process, on Linux runners.
set -euo pipefail

for attempt in 1 2 3; do
  status=0
  timeout --kill-after=10s "${RETRY_TIMEOUT:-5m}" "$@" || status=$?
  if (( status == 0 )); then
    exit 0
  fi
  echo "::warning::Registry command failed (exit ${status}, attempt ${attempt}/3)" >&2
  if (( attempt == 3 )); then
    exit "$status"
  fi
  sleep "${RETRY_DELAY:-10}"
done
