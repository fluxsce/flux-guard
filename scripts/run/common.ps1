# 安装包里的启动与停止。脚本在 scripts/run，程序在安装包根目录。

$script:Root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path

function Assert-Name([string]$Name) {
    if ($Name -notin @('server', 'host', 'app')) {
        throw "用法: server、host 或 app"
    }
}

function Get-BinaryName([string]$Name) {
    switch ($Name) {
        'server' { 'flux-guard-server' }
        'host' { 'flux-guard-agent-host' }
        'app' { 'flux-guard-agent-app' }
    }
}

function Test-BinaryPresent([string]$Name) {
    $base = Get-BinaryName $Name
    $exe = Join-Path $script:Root ($base + '.exe')
    $plain = Join-Path $script:Root $base
    return (Test-Path -LiteralPath $exe) -or (Test-Path -LiteralPath $plain)
}

function Resolve-Name([string]$Name) {
    if ($Name) {
        Assert-Name $Name
        return $Name
    }
    $found = @()
    foreach ($candidate in @('server', 'host', 'app')) {
        if (Test-BinaryPresent $candidate) { $found += $candidate }
    }
    if ($found.Count -eq 1) { return $found[0] }
    if ($found.Count -eq 0) {
        throw "此目录没有可启动的程序。中央服务在安装包根目录，代理在 agents/host 或 agents/app。"
    }
    throw "此目录有多个程序，请指定 server、host 或 app。"
}

function Resolve-Binary([string]$Name) {
    $base = Get-BinaryName $Name
    $exe = Join-Path $script:Root ($base + '.exe')
    $plain = Join-Path $script:Root $base
    if (Test-Path -LiteralPath $exe) { return $exe }
    if (Test-Path -LiteralPath $plain) { return $plain }
    throw "找不到 $base。请在该程序所在目录执行：中央服务在安装包根目录，代理在 agents/host 或 agents/app。"
}

function Ensure-Config([string]$Name) {
    $dst = Join-Path $script:Root "configs\$Name.yaml"
    if (Test-Path -LiteralPath $dst) { return }
    $src = Join-Path $script:Root "configs\$Name.yaml.example"
    if (-not (Test-Path -LiteralPath $src)) {
        throw "缺少配置示例 $src"
    }
    Copy-Item -LiteralPath $src -Destination $dst
    Write-Output "已生成 configs\$Name.yaml。当前仍是示例值，上线前请改中央地址和上报令牌。"
}

function Get-PidFile([string]$Name) {
    Join-Path $script:Root "data\$Name.pid"
}

function Test-Running([string]$Name) {
    $file = Get-PidFile $Name
    if (-not (Test-Path -LiteralPath $file)) { return $false }
    $text = (Get-Content -LiteralPath $file -Raw).Trim()
    if (-not $text) { return $false }
    return $null -ne (Get-Process -Id $text -ErrorAction SilentlyContinue)
}

function Start-One([string]$Name) {
    $Name = Resolve-Name $Name
    if (Test-Running $Name) {
        $procId = (Get-Content -LiteralPath (Get-PidFile $Name) -Raw).Trim()
        throw "$Name 已在运行 (pid $procId)"
    }
    Ensure-Config $Name
    $bin = Resolve-Binary $Name
    New-Item -ItemType Directory -Force -Path (Join-Path $script:Root 'data'), (Join-Path $script:Root 'logs') | Out-Null
    $proc = Start-Process -FilePath $bin -WorkingDirectory $script:Root -WindowStyle Hidden -PassThru
    Set-Content -LiteralPath (Get-PidFile $Name) -Value $proc.Id -Encoding ascii
    Write-Output "$Name 已启动 (pid $($proc.Id))"
}

function Stop-One([string]$Name) {
    $Name = Resolve-Name $Name
    $file = Get-PidFile $Name
    if (-not (Test-Running $Name)) {
        if (Test-Path -LiteralPath $file) { Remove-Item -LiteralPath $file -Force }
        Write-Output "$Name 未在运行"
        return
    }
    $procId = [int](Get-Content -LiteralPath $file -Raw).Trim()
    Write-Output "正在停止 $Name (pid $procId)"
    Stop-Process -Id $procId -ErrorAction SilentlyContinue
    $left = 30
    while ($left -gt 0 -and (Get-Process -Id $procId -ErrorAction SilentlyContinue)) {
        Start-Sleep -Seconds 1
        $left--
    }
    if (Get-Process -Id $procId -ErrorAction SilentlyContinue) {
        Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $file) { Remove-Item -LiteralPath $file -Force }
    Write-Output "$Name 已停止"
}
