$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

# PATH 上的 java 可能是 JRE。jlink 和 jpackage 只在 JDK 里，优先用 JAVA_HOME。
$bindir = $null
if ($env:JAVA_HOME) {
    $bindir = Join-Path $env:JAVA_HOME "bin"
}
if (-not $env:JAVA_HOME) {
    $java = Get-Command java -ErrorAction SilentlyContinue
    if ($java) {
        $bindir = Split-Path $java.Source
        $env:JAVA_HOME = Split-Path $bindir
    }
}
if (-not $bindir) {
    Write-Error "JAVA_HOME 需要指向带 jlink 的 JDK"
}
$jlink = Join-Path $bindir "jlink.exe"
if (-not (Test-Path $jlink)) {
    Write-Error "JAVA_HOME 需要指向带 jlink 的 JDK"
}

mvn -q -DskipTests clean package
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
if (Test-Path "$root\dist") {
    Remove-Item -Recurse -Force "$root\dist"
}
$jpackage = Join-Path $bindir "jpackage.exe"
$runtime = Join-Path $root "target\runtime"
if (Test-Path $runtime) {
    Remove-Item -Recurse -Force $runtime
}
# 只链探针会加载的模块。命名管道不碰 JNA 的窗口类，所以不带 java.desktop。
$modules = "java.base,java.management,java.naming,java.security.jgss,java.security.sasl,jdk.crypto.ec,jdk.unsupported"
& $jlink --add-modules $modules --output $runtime --strip-debug --no-man-pages --no-header-files --compress=2
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& $jpackage --type app-image --name zk-probe --runtime-image $runtime --input "$root\target\image-input" --main-jar zk-probe.jar --main-class fluxguard.zk.Main --dest "$root\dist" --win-console
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
