#!/usr/bin/env sh
set -eu
here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
# shellcheck disable=SC1091
. "${here}/common.sh"
# shellcheck disable=SC1091
. "${here}/server-companion.sh"
name="${1:-}"
if [ -z "${name}" ]; then
  name=$(detect_name) || exit 1
else
  check_name "${name}"
fi
uninstall_boot "${name}"
if [ "${name}" = "server" ]; then
  uninstall_server_companion
fi
