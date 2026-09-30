#!/usr/bin/env sh
set -eu
. "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/common.sh"
stop_one "${1:-}"
