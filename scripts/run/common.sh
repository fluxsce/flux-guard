#!/usr/bin/env sh
# 安装包里的启动与停止。脚本在 scripts/run，程序在安装包根目录。

root_dir() {
  CDPATH= cd -- "$(dirname "$0")/../.." && pwd
}

ROOT="$(root_dir)"

usage() {
  echo "用法: $0 server|host|goruntime|appside|app" >&2
  exit 1
}

check_name() {
  case "$1" in
    server|host|goruntime|appside|app) ;;
    *) usage ;;
  esac
}

binary_name() {
  case "$1" in
    server) echo flux-guard-server ;;
    host) echo flux-guard-agent-host ;;
    goruntime) echo flux-guard-agent-goruntime ;;
    appside) echo flux-guard-agent-appside ;;
    app) echo flux-guard-agent-app ;;
  esac
}

present() {
  base="$(binary_name "$1")"
  [ -f "${ROOT}/${base}" ] || [ -f "${ROOT}/${base}.exe" ]
}

detect_name() {
  found=""
  for name in server host goruntime appside app; do
    if present "${name}"; then
      if [ -n "${found}" ]; then
        echo "此目录有多个程序，请指定 server、host、goruntime、appside 或 app" >&2
        return 1
      fi
      found="${name}"
    fi
  done
  if [ -z "${found}" ]; then
    echo "此目录没有可启动的程序。中央服务在安装包根目录，代理在 agents/host、agents/goruntime 或 agents/appside。" >&2
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
  echo "找不到 ${base}。请在该程序所在目录执行：中央服务在安装包根目录，代理在 agents/host、agents/goruntime 或 agents/appside。" >&2
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

service_name() {
  binary_name "$1"
}

unit_file() {
  echo "/etc/systemd/system/$(service_name "$1").service"
}

plist_file() {
  echo "/Library/LaunchDaemons/$(service_name "$1").plist"
}

boot_managed() {
  case "$(uname)" in
    Linux) [ -f "$(unit_file "$1")" ] ;;
    Darwin) [ -f "$(plist_file "$1")" ] ;;
    *) return 1 ;;
  esac
}

boot_active() {
  label="$(service_name "$1")"
  case "$(uname)" in
    Linux)
      systemctl is-active --quiet "${label}.service"
      ;;
    Darwin)
      launchctl print "system/${label}" 2>/dev/null | grep -q 'state = running'
      ;;
    *)
      return 1
      ;;
  esac
}

boot_start() {
  label="$(service_name "$1")"
  case "$(uname)" in
    Linux) systemctl start "${label}.service" ;;
    Darwin) launchctl kickstart "system/${label}" ;;
  esac
  echo "$1 已由开机服务启动"
}

boot_stop() {
  label="$(service_name "$1")"
  case "$(uname)" in
    Linux) systemctl stop "${label}.service" ;;
    Darwin) launchctl kill SIGTERM "system/${label}" ;;
  esac
  echo "$1 已停止"
}

boot_restart() {
  label="$(service_name "$1")"
  case "$(uname)" in
    Linux) systemctl restart "${label}.service" ;;
    Darwin) launchctl kickstart -k "system/${label}" ;;
  esac
  echo "$1 已重启"
}

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    echo "请用 root 执行，例如 sudo $0" >&2
    exit 1
  fi
}

write_systemd_unit() {
  bin="$(resolve_bin "$1")"
  label="$(service_name "$1")"
  cat > "$(unit_file "$1")" <<EOF
[Unit]
Description=${label}
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
WorkingDirectory=${ROOT}
ExecStart=${bin}
Restart=on-failure
RestartSec=3
StandardOutput=null
StandardError=null

[Install]
WantedBy=multi-user.target
EOF
}

write_launchd_plist() {
  bin="$(resolve_bin "$1")"
  label="$(service_name "$1")"
  cat > "$(plist_file "$1")" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${label}</string>
  <key>WorkingDirectory</key>
  <string>${ROOT}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${bin}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/dev/null</string>
  <key>StandardErrorPath</key>
  <string>/dev/null</string>
</dict>
</plist>
EOF
}

install_boot() {
  name="${1:-}"
  if [ -z "${name}" ]; then
    name="$(detect_name)" || return 1
  fi
  check_name "${name}"
  require_root
  ensure_config "${name}"
  if is_running "${name}"; then
    stop_one "${name}"
  fi
  label="$(service_name "${name}")"
  case "$(uname)" in
    Linux)
      if ! command -v systemctl >/dev/null 2>&1; then
        echo "未找到 systemctl，无法登记开机启动" >&2
        return 1
      fi
      write_systemd_unit "${name}"
      systemctl daemon-reload
      systemctl enable "${label}.service"
      systemctl restart "${label}.service"
      ;;
    Darwin)
      write_launchd_plist "${name}"
      launchctl bootout "system/${label}" >/dev/null 2>&1 || true
      launchctl bootstrap system "$(plist_file "${name}")"
      ;;
    *)
      echo "当前系统请使用 scripts/run/install.ps1" >&2
      return 1
      ;;
  esac
  echo "${name} 已登记开机启动：${ROOT}"
}

uninstall_boot() {
  name="${1:-}"
  if [ -z "${name}" ]; then
    name="$(detect_name)" || return 1
  fi
  check_name "${name}"
  require_root
  label="$(service_name "${name}")"
  case "$(uname)" in
    Linux)
      systemctl disable --now "${label}.service" >/dev/null 2>&1 || true
      rm -f "$(unit_file "${name}")"
      systemctl daemon-reload
      ;;
    Darwin)
      launchctl bootout "system/${label}" >/dev/null 2>&1 || true
      rm -f "$(plist_file "${name}")"
      ;;
    *)
      echo "当前系统请使用 scripts/run/uninstall.ps1" >&2
      return 1
      ;;
  esac
  echo "${name} 已取消开机启动"
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
  if boot_managed "$1"; then
    if boot_active "$1"; then
      echo "$1 已在运行"
      return 1
    fi
    ensure_config "$1"
    boot_start "$1"
    return 0
  fi
  if is_running "$1"; then
    echo "$1 已在运行 (pid $(cat "$(pid_file "$1")"))"
    return 1
  fi
  ensure_config "$1"
  bin="$(resolve_bin "$1")"
  mkdir -p "${ROOT}/data" "${ROOT}/logs"
  cd "${ROOT}"
  log="${ROOT}/logs/${1}.stdout.log"
  nohup "${bin}" >> "${log}" 2>&1 &
  pid="$!"
  echo "${pid}" > "$(pid_file "$1")"
  sleep 1
  if ! kill -0 "${pid}" 2>/dev/null; then
    rm -f "$(pid_file "$1")"
    echo "$1 没有保持运行。最近输出：" >&2
    tail -n 30 "${log}" >&2 || true
    return 1
  fi
  echo "$1 已启动 (pid ${pid})"
}

stop_one() {
  name="${1:-}"
  if [ -z "${name}" ]; then
    name="$(detect_name)" || return 1
  fi
  check_name "${name}"
  set -- "${name}"
  if boot_managed "$1"; then
    boot_stop "$1"
    return 0
  fi
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

restart_one() {
  name="${1:-}"
  if [ -z "${name}" ]; then
    name="$(detect_name)" || return 1
  fi
  check_name "${name}"
  if boot_managed "${name}"; then
    ensure_config "${name}"
    boot_restart "${name}"
    return 0
  fi
  stop_one "${name}"
  start_one "${name}"
}
