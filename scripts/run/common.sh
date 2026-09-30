#!/usr/bin/env sh
# 安装包里的启动与停止。脚本在 scripts/run，程序在安装包根目录。

root_dir() {
  CDPATH= cd -- "$(dirname "$0")/../.." && pwd
}

ROOT="$(root_dir)"

usage() {
  echo "用法: $0 server|host|app" >&2
  exit 1
}

check_name() {
  case "$1" in
    server|host|app) ;;
    *) usage ;;
  esac
}

binary_name() {
  case "$1" in
    server) echo flux-guard-server ;;
    host) echo flux-guard-agent-host ;;
    app) echo flux-guard-agent-app ;;
  esac
}

present() {
  base="$(binary_name "$1")"
  [ -f "${ROOT}/${base}" ] || [ -f "${ROOT}/${base}.exe" ]
}

detect_name() {
  found=""
  for name in server host app; do
    if present "${name}"; then
      if [ -n "${found}" ]; then
        echo "此目录有多个程序，请指定 server、host 或 app" >&2
        return 1
      fi
      found="${name}"
    fi
  done
  if [ -z "${found}" ]; then
    echo "此目录没有可启动的程序。中央服务在安装包根目录，代理在 agents/host 或 agents/app。" >&2
    return 1
  fi
  printf '%s\n' "${found}"
}

resolve_bin() {
  base="$(binary_name "$1")"
  if [ -f "${ROOT}/${base}.exe" ]; then
    echo "${ROOT}/${base}.exe"
    return 0
  fi
  if [ -f "${ROOT}/${base}" ]; then
    echo "${ROOT}/${base}"
    return 0
  fi
  echo "找不到 ${base}。请在该程序所在目录执行：中央服务在安装包根目录，代理在 agents/host 或 agents/app。" >&2
  return 1
}

ensure_config() {
  name="$1"
  dst="${ROOT}/configs/${name}.yaml"
  src="${ROOT}/configs/${name}.yaml.example"
  if [ -f "${dst}" ]; then
    return 0
  fi
  if [ ! -f "${src}" ]; then
    echo "缺少配置示例 ${src}" >&2
    return 1
  fi
  cp "${src}" "${dst}"
  echo "已生成 configs/${name}.yaml。当前仍是示例值，上线前请改中央地址和上报令牌。"
}

pid_file() {
  echo "${ROOT}/data/${1}.pid"
}

is_running() {
  file="$(pid_file "$1")"
  if [ ! -f "${file}" ]; then
    return 1
  fi
  pid="$(cat "${file}" 2>/dev/null)" || return 1
  [ -n "${pid}" ] && kill -0 "${pid}" 2>/dev/null
}

start_one() {
  name="${1:-}"
  if [ -z "${name}" ]; then
    name="$(detect_name)" || return 1
  fi
  check_name "${name}"
  set -- "${name}"
  if is_running "$1"; then
    echo "$1 已在运行 (pid $(cat "$(pid_file "$1")"))"
    return 1
  fi
  ensure_config "$1"
  bin="$(resolve_bin "$1")"
  mkdir -p "${ROOT}/data" "${ROOT}/logs"
  cd "${ROOT}"
  nohup "${bin}" >> "${ROOT}/logs/${1}.stdout.log" 2>&1 &
  echo "$!" > "$(pid_file "$1")"
  echo "$1 已启动 (pid $(cat "$(pid_file "$1")"))"
}

stop_one() {
  name="${1:-}"
  if [ -z "${name}" ]; then
    name="$(detect_name)" || return 1
  fi
  check_name "${name}"
  set -- "${name}"
  file="$(pid_file "$1")"
  if ! is_running "$1"; then
    rm -f "${file}"
    echo "$1 未在运行"
    return 0
  fi
  pid="$(cat "${file}")"
  echo "正在停止 $1 (pid ${pid})"
  kill "${pid}" 2>/dev/null || true
  i=0
  while kill -0 "${pid}" 2>/dev/null; do
    i=$((i + 1))
    if [ "${i}" -ge 30 ]; then
      kill -9 "${pid}" 2>/dev/null || true
      break
    fi
    sleep 1
  done
  rm -f "${file}"
  echo "$1 已停止"
}
