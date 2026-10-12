#!/bin/sh
# 在当前系统打出 ZooKeeper 插件镜像。清单在 plugins/manifests，这里不写。
# Linux 启动器在 bin/，macOS 在 .app 里，外面再放一个同名脚本，清单里的路径不用按系统分叉。
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
cd "$root"
if [ -z "${JAVA_HOME:-}" ] || [ ! -x "$JAVA_HOME/bin/jlink" ]; then
  echo "JAVA_HOME 需要指向带 jlink 的 JDK" >&2
  exit 1
fi
mvn -q -DskipTests clean package
rm -rf "$root/dist" "$root/target/runtime"
modules="java.base,java.management,java.naming,java.security.jgss,java.security.sasl,jdk.crypto.ec,jdk.unsupported"
"$JAVA_HOME/bin/jlink" --add-modules "$modules" --output "$root/target/runtime" \
  --strip-debug --no-man-pages --no-header-files --compress=2
"$JAVA_HOME/bin/jpackage" --type app-image --name zk-probe \
  --runtime-image "$root/target/runtime" \
  --input "$root/target/image-input" \
  --main-jar zk-probe.jar \
  --main-class fluxguard.zk.Main \
  --dest "$root/dist"
if [ "$(uname)" = "Darwin" ]; then
  mv "$root/dist/zk-probe.app" "$root/target/zk-probe.app"
  mkdir -p "$root/dist/zk-probe"
  mv "$root/target/zk-probe.app" "$root/dist/zk-probe/zk-probe.app"
fi
cat > "$root/dist/zk-probe/zk-probe" << 'EOF'
#!/bin/sh
here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
if [ -x "$here/bin/zk-probe" ]; then
  exec "$here/bin/zk-probe" "$@"
fi
exec "$here/zk-probe.app/Contents/MacOS/zk-probe" "$@"
EOF
chmod 755 "$root/dist/zk-probe/zk-probe"
