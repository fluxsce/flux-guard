# 中央安装包附带的本机主机代理。
# 只在安装或卸载中央服务时调用：解开 agents/flux-guard-agent-host.tar.gz，
# 按中央 listen 写好 server_url，再执行该目录自己的安装或卸载脚本。
# 这里不采集，也不在中央进程里启动代理。

# yaml_scalar 读取顶层标量。去掉行尾注释和引号。
yaml_scalar() {
  file=$1
  key=$2
  sed -n "s/^${key}:[[:space:]]*//p" "$file" | head -n 1 \
    | sed 's/[[:space:]]*#.*$//' \
    | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' \
    | sed 's/^"//;s/"$//' \
    | sed "s/^'//;s/'$//"
}

# agent_server_url 把中央 listen 收成同一台机器上主机代理要访问的地址。
# 监听所有网卡时用 127.0.0.1。写了具体地址时用那个地址，进程只绑在那里。
agent_server_url() {
  listen=$1
  https=$2
  scheme=http
  case $https in
    true|TRUE|True) scheme=https ;;
  esac
  port=$(printf '%s\n' "$listen" | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p')
  if [ -z "$port" ]; then
    port=8080
  fi
  host=$(printf '%s\n' "$listen" | sed 's/:[0-9][0-9]*$//')
  case $host in
    ""|0.0.0.0|"::"|"[::]"|"*"|"[::0]") host=127.0.0.1 ;;
  esac
  printf '%s://%s:%s\n' "$scheme" "$host" "$port"
}

# set_yaml_key 替换或追加一个顶层键，保留文件原来的编码。
set_yaml_key() {
  file=$1
  key=$2
  value=$3
  comment=$4
  keyfile=$(mktemp)
  awk -v key="$key" -v value="$value" -v comment="$comment" '
    index($0, key ":") == 1 {
      line = key ": " value
      if (comment != "") line = line " # " comment
      print line
      found = 1
      next
    }
    { print }
    END {
      if (!found) {
        line = key ": " value
        if (comment != "") line = line " # " comment
        print line
      }
    }
  ' "$file" > "$keyfile"
  mv "$keyfile" "$file"
}

server_companion_url() {
  cfg="${ROOT}/configs/server.yaml"
  if [ ! -f "$cfg" ]; then
    printf '%s\n' "http://127.0.0.1:8080"
    return 0
  fi
  listen=$(yaml_scalar "$cfg" listen)
  https=$(yaml_scalar "$cfg" enable_https)
  agent_server_url "$listen" "$https"
}

prepare_companion_config() {
  dir=$1
  url=$2
  created=$3
  file="${dir}/configs/host.yaml"
  if [ "$created" = "1" ]; then
    set_yaml_key "$file" report_name "中央本机" "中央安装时写入；控制台改名后列表名不再被这份配置覆盖"
    set_yaml_key "$file" description "中央服务所在机器的主机代理" "非空时每次上报覆盖控制台备注"
  fi
  set_yaml_key "$file" server_url "$url" "中央地址，随 configs/server.yaml 的监听地址"
}

install_server_companion() {
  if ! present server; then
    echo "当前目录不是中央安装包，跳过本机主机代理" >&2
    return 0
  fi
  archive="${ROOT}/agents/flux-guard-agent-host.tar.gz"
  dir="${ROOT}/agents/flux-guard-agent-host"
  if [ ! -f "$archive" ]; then
    echo "未找到 ${archive}。中央服务已登记，本机主机代理未安装。" >&2
    return 1
  fi
  if [ -f "${dir}/scripts/run/stop.sh" ]; then
    sh "${dir}/scripts/run/stop.sh" host
  fi
  mkdir -p "${ROOT}/agents"
  tar -xzf "$archive" -C "${ROOT}/agents"
  created=0
  if [ ! -f "${dir}/configs/host.yaml" ]; then
    if [ ! -f "${dir}/configs/host.yaml.example" ]; then
      echo "主机代理包里没有 configs/host.yaml.example" >&2
      return 1
    fi
    cp "${dir}/configs/host.yaml.example" "${dir}/configs/host.yaml"
    created=1
  fi
  url=$(server_companion_url)
  prepare_companion_config "$dir" "$url" "$created"
  sh "${dir}/scripts/run/install.sh" host
  echo "本机主机代理已登记：${dir}"
  echo "上报地址 ${url}。令牌用该目录 configs/host.yaml 的 agent_token。在环境设置里更换后，请同步改这一行。"
}

uninstall_server_companion() {
  dir="${ROOT}/agents/flux-guard-agent-host"
  script="${dir}/scripts/run/uninstall.sh"
  if [ ! -f "$script" ]; then
    return 0
  fi
  sh "$script" host
}
