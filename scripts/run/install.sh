#!/usr/bin/env sh
set -eu
. "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/common.sh"
install_boot "${1:-}"
