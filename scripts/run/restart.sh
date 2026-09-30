#!/usr/bin/env sh
set -eu
. "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/common.sh"
restart_one "${1:-}"
