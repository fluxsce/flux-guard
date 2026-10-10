#!/bin/sh
# 在当前系统打出 Kafka 插件镜像。清单在 plugins/manifests，这里不写。
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
modules="java.base,java.management,java.security.jgss,java.security.sasl,java.sql,jdk.crypto.ec,jdk.unsupported"
"$JAVA_HOME/bin/jlink" --add-modules "$modules" --output "$root/target/runtime" \
  --strip-debug --no-man-pages --no-header-files --compress=2
"$JAVA_HOME/bin/jpackage" --type app-image --name kafka-probe \
  --runtime-image "$root/target/runtime" \
  --input "$root/target/image-input" \
  --main-jar kafka-probe.jar \
  --main-class fluxguard.kafka.Main \
  --dest "$root/dist"
if [ "$(uname)" = "Darwin" ]; then
  mv "$root/dist/kafka-probe.app" "$root/target/kafka-probe.app"
  mkdir -p "$root/dist/kafka-probe"
  mv "$root/target/kafka-probe.app" "$root/dist/kafka-probe/kafka-probe.app"
fi
cat > "$root/dist/kafka-probe/kafka-probe" << 'EOF'
#!/bin/sh
here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
if [ -x "$here/bin/kafka-probe" ]; then
  exec "$here/bin/kafka-probe" "$@"
fi
exec "$here/kafka-probe.app/Contents/MacOS/kafka-probe" "$@"
EOF
chmod 755 "$root/dist/kafka-probe/kafka-probe"
