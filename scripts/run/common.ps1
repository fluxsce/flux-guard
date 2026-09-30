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

function Get-BootTaskName([string]$Name) {
    Get-BinaryName $Name
}

function Test-BootManaged([string]$Name) {
    return $null -ne (Get-ScheduledTask -TaskName (Get-BootTaskName $Name) -ErrorAction SilentlyContinue)
}

function Test-BootActive([string]$Name) {
    $task = Get-ScheduledTask -TaskName (Get-BootTaskName $Name) -ErrorAction SilentlyContinue
    return $null -ne $task -and $task.State -eq 'Running'
}

function Assert-Admin() {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "请用管理员 PowerShell 执行"
    }
}

function Install-Boot([string]$Name) {
    $Name = Resolve-Name $Name
    Assert-Admin
    Ensure-Config $Name
    if (Test-Running $Name) { Stop-One $Name }
    if (Test-BootManaged $Name) { Stop-ScheduledTask -TaskName (Get-BootTaskName $Name) -ErrorAction SilentlyContinue }
    $bin = Resolve-Binary $Name
    $action = New-ScheduledTaskAction -Execute $bin -WorkingDirectory $script:Root
    $trigger = New-ScheduledTaskTrigger -AtStartup
    $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 10 -RestartInterval (New-TimeSpan -Minutes 1)
    $settings.ExecutionTimeLimit = 'PT0S'
    Register-ScheduledTask -TaskName (Get-BootTaskName $Name) -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
    Start-ScheduledTask -TaskName (Get-BootTaskName $Name)
    Write-Output "$Name 已登记开机启动：$script:Root"
}

function Uninstall-Boot([string]$Name) {
    $Name = Resolve-Name $Name
    Assert-Admin
    $taskName = Get-BootTaskName $Name
    if (Test-BootManaged $Name) {
        Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }
    Write-Output "$Name 已取消开机启动"
}

function Start-One([string]$Name) {
    $Name = Resolve-Name $Name
    if (Test-BootManaged $Name) {
        if (Test-BootActive $Name) { throw "$Name 已在运行" }
        Ensure-Config $Name
        Start-ScheduledTask -TaskName (Get-BootTaskName $Name)
        Write-Output "$Name 已由开机服务启动"
        return
    }
    if (Test-Running $Name) {
        $procId = (Get-Content -LiteralPath (Get-PidFile $Name) -Raw).Trim()
        throw "$Name 已在运行 (pid $procId)"
    }
    Ensure-Config $Name
    $bin = Resolve-Binary $Name
    $logDir = Join-Path $script:Root 'logs'
    New-Item -ItemType Directory -Force -Path (Join-Path $script:Root 'data'), $logDir | Out-Null
    $outLog = Join-Path $logDir "$Name.stdout.log"
    $errLog = Join-Path $logDir "$Name.stderr.log"
    $proc = Start-Process -FilePath $bin -WorkingDirectory $script:Root -WindowStyle Hidden -RedirectStandardOutput $outLog -RedirectStandardError $errLog -PassThru
    Set-Content -LiteralPath (Get-PidFile $Name) -Value $proc.Id -Encoding ascii
    Start-Sleep -Seconds 1
    if ($proc.HasExited) {
        Remove-Item -LiteralPath (Get-PidFile $Name) -Force -ErrorAction SilentlyContinue
        $tail = ''
        if (Test-Path -LiteralPath $errLog) { $tail = (Get-Content -LiteralPath $errLog -Tail 30) -join "`n" }
        throw "$Name 没有保持运行。最近输出：`n$tail"
    }
    Write-Output "$Name 已启动 (pid $($proc.Id))"
}

function Stop-One([string]$Name) {
    $Name = Resolve-Name $Name
    if (Test-BootManaged $Name) {
        Stop-ScheduledTask -TaskName (Get-BootTaskName $Name) -ErrorAction SilentlyContinue
        Write-Output "$Name 已停止"
        return
    }
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

function Restart-One([string]$Name) {
    $Name = Resolve-Name $Name
    if (Test-BootManaged $Name) {
        Ensure-Config $Name
        $taskName = Get-BootTaskName $Name
        Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
        Start-ScheduledTask -TaskName $taskName
        Write-Output "$Name 已重启"
        return
    }
    Stop-One $Name
    Start-One $Name
}
