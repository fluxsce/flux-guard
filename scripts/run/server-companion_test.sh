#!/bin/sh
# 只检查本机主机代理的地址和配置改写，不登记系统服务。
set -eu
here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
# shellcheck disable=SC1091
. "${here}/server-companion.sh"

fail=0
check_url() {
  got=$(agent_server_url "$1" "$2")
  if [ "$got" != "$3" ]; then
    echo "listen=$1 https=$2 got=$got want=$3" >&2
    fail=1
  fi
}

check_url ":8080" false "http://127.0.0.1:8080"
check_url "0.0.0.0:8080" false "http://127.0.0.1:8080"
check_url "[::]:8080" false "http://127.0.0.1:8080"
check_url "127.0.0.1:9090" false "http://127.0.0.1:9090"
check_url "10.0.0.5:9090" true "https://10.0.0.5:9090"
check_url "" false "http://127.0.0.1:8080"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cat > "${tmp}/server.yaml" <<'EOF'
listen: ":8080" # 本程序监听的地址
enable_https: false
EOF
got=$(yaml_scalar "${tmp}/server.yaml" listen)
if [ "$got" != ":8080" ]; then
  echo "yaml listen got=$got" >&2
  fail=1
fi
got=$(yaml_scalar "${tmp}/server.yaml" enable_https)
if [ "$got" != "false" ]; then
  echo "yaml https got=$got" >&2
  fail=1
fi

mkdir -p "${tmp}/agent/configs"
cp "${here}/../../configs/host.yaml.example" "${tmp}/agent/configs/host.yaml"
prepare_companion_config "${tmp}/agent" "http://127.0.0.1:9090" 1
url=$(yaml_scalar "${tmp}/agent/configs/host.yaml" server_url)
name=$(yaml_scalar "${tmp}/agent/configs/host.yaml" report_name)
token=$(yaml_scalar "${tmp}/agent/configs/host.yaml" agent_token)
if [ "$url" != "http://127.0.0.1:9090" ] || [ "$name" != "中央本机" ] || [ "$token" != "agt_dev_local" ]; then
  echo "create url=$url name=$name token=$token" >&2
  fail=1
fi

set_yaml_key "${tmp}/agent/configs/host.yaml" agent_token "agt_rotated" ""
prepare_companion_config "${tmp}/agent" "https://10.0.0.8:8443" 0
url=$(yaml_scalar "${tmp}/agent/configs/host.yaml" server_url)
name=$(yaml_scalar "${tmp}/agent/configs/host.yaml" report_name)
token=$(yaml_scalar "${tmp}/agent/configs/host.yaml" agent_token)
if [ "$url" != "https://10.0.0.8:8443" ] || [ "$name" != "中央本机" ] || [ "$token" != "agt_rotated" ]; then
  echo "update url=$url name=$name token=$token" >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "server-companion ok"
