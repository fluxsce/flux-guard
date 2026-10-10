$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

mvn -q -DskipTests clean package
if (Test-Path "$root\dist") {
    Remove-Item -Recurse -Force "$root\dist"
}
$bindir = Split-Path (Get-Command java).Source
$jlink = Join-Path $bindir "jlink.exe"
$jpackage = Join-Path $bindir "jpackage.exe"
$runtime = Join-Path $root "target\runtime"
if (Test-Path $runtime) {
    Remove-Item -Recurse -Force $runtime
}
# 只链探针会加载的模块。命名管道不碰 JNA 的窗口类，所以不带 java.desktop。
# jdk.crypto.ec 给 TLS 的椭圆曲线，静态分析看不到。
$modules = "java.base,java.management,java.security.jgss,java.security.sasl,java.sql,jdk.crypto.ec,jdk.unsupported"
& $jlink --add-modules $modules --output $runtime --strip-debug --no-man-pages --no-header-files --compress=2
& $jpackage --type app-image --name kafka-probe --runtime-image $runtime --input "$root\target\image-input" --main-jar kafka-probe.jar --main-class fluxguard.kafka.Main --dest "$root\dist" --win-console
