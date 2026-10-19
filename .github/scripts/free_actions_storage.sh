#!/usr/bin/env bash
# 新版本发布开始时清掉已结束运行留下的 Actions 产物和缓存，并打开本次插件中转用的 draft release。
# 插件包不进 Actions artifacts：删除后配额要 6–12 小时才重算，同一次运行里 upload-artifact 仍会失败。
set -euo pipefail

: "${GITHUB_REPOSITORY:?}"
: "${GITHUB_RUN_ID:?}"
: "${GITHUB_SHA:?}"
: "${GH_TOKEN:?}"

handoff="build-${GITHUB_RUN_ID}"

delete_by_url() {
  local kind="$1"
  local id="$2"
  local err
  err=$(mktemp)
  if gh api -X DELETE "repos/${GITHUB_REPOSITORY}/${kind}/${id}" >"${err}" 2>&1; then
    rm -f "${err}"
    return 0
  fi
  if grep -Eqi '404|not found' "${err}"; then
    rm -f "${err}"
    return 0
  fi
  cat "${err}" >&2
  rm -f "${err}"
  return 1
}

run_status() {
  local run_id="$1"
  local err status
  err=$(mktemp)
  if status=$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs/${run_id}" --jq '.status' 2>"${err}"); then
    rm -f "${err}"
    printf '%s\n' "${status}"
    return 0
  fi
  if grep -Eqi '404|not found' "${err}"; then
    rm -f "${err}"
    printf '%s\n' missing
    return 0
  fi
  cat "${err}" >&2
  rm -f "${err}"
  return 1
}

bytes=0
add_bytes() {
  local size="$1"
  case "${size}" in
    ''|*[!0-9]*) size=0 ;;
  esac
  bytes=$((bytes + size))
}

artifacts=$(mktemp)
statuses=$(mktemp)
seen=$(mktemp)
caches=$(mktemp)
releases=$(mktemp)
trap 'rm -f "${artifacts}" "${statuses}" "${seen}" "${caches}" "${releases}"' EXIT

echo "listing artifacts"
gh api --paginate "repos/${GITHUB_REPOSITORY}/actions/artifacts?per_page=100" \
  --jq '.artifacts[] | [.id, .workflow_run.id, .name, .size_in_bytes] | @tsv' >"${artifacts}"

while IFS=$'\t' read -r _id run_id _name _size; do
  [ -n "${run_id}" ] || continue
  [ "${run_id}" = "${GITHUB_RUN_ID}" ] && continue
  if grep -qx "${run_id}" "${seen}"; then
    continue
  fi
  echo "${run_id}" >>"${seen}"
  status=$(run_status "${run_id}") || exit 1
  echo "${run_id} ${status}" >>"${statuses}"
done <"${artifacts}"

while IFS=$'\t' read -r id run_id name size; do
  [ -n "${id}" ] || continue
  if [ "${run_id}" = "${GITHUB_RUN_ID}" ]; then
    status="this-run"
    drop=1
  else
    status=$(awk -v id="${run_id}" '$1 == id { print $2; exit }' "${statuses}")
    if [ "${status}" = "completed" ] || [ "${status}" = "missing" ]; then
      drop=1
    else
      drop=0
    fi
  fi
  if [ "${drop}" -eq 1 ]; then
    echo "delete artifact ${name} (${size:-0} bytes) from run ${run_id} (${status})"
    delete_by_url "actions/artifacts" "${id}" || exit 1
    add_bytes "${size}"
  else
    echo "keep artifact ${name} from in-progress run ${run_id} (${status})"
  fi
done <"${artifacts}"

echo "listing caches"
gh api --paginate "repos/${GITHUB_REPOSITORY}/actions/caches?per_page=100" \
  --jq '.actions_caches[] | [.id, .size_in_bytes, .key] | @tsv' >"${caches}"
while IFS=$'\t' read -r id size key; do
  [ -n "${id}" ] || continue
  echo "delete cache ${id} (${size:-0} bytes) ${key}"
  delete_by_url "actions/caches" "${id}" || exit 1
  add_bytes "${size}"
done <"${caches}"

echo "listing temporary plugin releases"
gh api --paginate "repos/${GITHUB_REPOSITORY}/releases?per_page=100" \
  --jq '.[] | select(.draft == true) | .tag_name' >"${releases}"
while read -r tag; do
  [ -n "${tag}" ] || continue
  case "${tag}" in
    build-[0-9]*) ;;
    *) continue ;;
  esac
  old_run=${tag#build-}
  [ "${old_run}" = "${GITHUB_RUN_ID}" ] && continue
  status=$(run_status "${old_run}") || exit 1
  if [ "${status}" = "completed" ] || [ "${status}" = "missing" ]; then
    echo "delete temporary release ${tag} (${status})"
    gh release delete "${tag}" --yes --cleanup-tag
  else
    echo "keep temporary release ${tag} (${status})"
  fi
done <"${releases}"

if gh release view "${handoff}" >/dev/null 2>&1; then
  echo "replace temporary release ${handoff}"
  gh release delete "${handoff}" --yes --cleanup-tag
fi
gh release create "${handoff}" \
  --draft \
  --target "${GITHUB_SHA}" \
  --title "${handoff}" \
  --notes "Temporary plugin handoff for workflow run ${GITHUB_RUN_ID}. Removed when publish finishes."

echo "deleted about ${bytes} bytes of Actions artifacts and caches"
echo "plugin handoff ${handoff} is ready"
