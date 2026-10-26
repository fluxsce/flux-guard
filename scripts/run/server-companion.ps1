# 中央安装包附带的本机主机代理。
# 只在安装或卸载中央服务时调用：解开 agents/flux-guard-agent-host.tar.gz，
# 按中央 listen 写好 server_url，再执行该目录自己的安装或卸载脚本。
# 这里不采集，也不在中央进程里启动代理。

function Get-YamlScalar([string]$File, [string]$Key) {
    if (-not (Test-Path -LiteralPath $File)) { return '' }
    $utf8 = New-Object System.Text.UTF8Encoding $false
    $text = [System.IO.File]::ReadAllText($File, $utf8)
    foreach ($line in ($text -split "`n")) {
        $line = $line.TrimEnd("`r")
        if ($line -match "^${Key}:\s*(.*)$") {
            $value = $Matches[1] -replace '\s+#.*$', ''
            return $value.Trim().Trim('"').Trim("'")
        }
    }
    return ''
}

# Get-AgentServerUrl 把中央 listen 收成同一台机器上主机代理要访问的地址。
# 监听所有网卡时用 127.0.0.1。写了具体地址时用那个地址，进程只绑在那里。
function Get-AgentServerUrl([string]$Listen, [string]$Https) {
    $scheme = 'http'
    if ($Https -in @('true', 'TRUE', 'True')) { $scheme = 'https' }
    $port = '8080'
    if ($Listen -match ':(\d+)$') { $port = $Matches[1] }
    $hostName = ''
    if ($Listen) { $hostName = $Listen -replace ':\d+$', '' }
    if ($hostName -in @('', '0.0.0.0', '::', '[::]', '*', '[::0]')) {
        $hostName = '127.0.0.1'
    }
    return "${scheme}://${hostName}:${port}"
}

function Set-YamlKey([string]$File, [string]$Key, [string]$Value, [string]$Comment) {
    $utf8 = New-Object System.Text.UTF8Encoding $false
    $text = [System.IO.File]::ReadAllText($File, $utf8)
    $newline = if ($text.Contains("`r`n")) { "`r`n" } else { "`n" }
    $replacement = if ($Comment) { "${Key}: ${Value} # ${Comment}" } else { "${Key}: ${Value}" }
    $lines = New-Object System.Collections.Generic.List[string]
    $found = $false
    foreach ($line in ($text -split "`r?`n", -1)) {
        if ($line.StartsWith("${Key}:")) {
            $lines.Add($replacement) | Out-Null
            $found = $true
        } else {
            $lines.Add($line) | Out-Null
        }
    }
    if (-not $found) { $lines.Add($replacement) | Out-Null }
    while ($lines.Count -gt 0 -and $lines[$lines.Count - 1] -eq '') { $lines.RemoveAt($lines.Count - 1) }
    $out = ($lines -join $newline) + $newline
    [System.IO.File]::WriteAllText($File, $out, $utf8)
}

function Get-ServerCompanionUrl() {
    $cfg = Join-Path $script:Root 'configs\server.yaml'
    if (-not (Test-Path -LiteralPath $cfg)) { return 'http://127.0.0.1:8080' }
    return Get-AgentServerUrl (Get-YamlScalar $cfg 'listen') (Get-YamlScalar $cfg 'enable_https')
}

function Set-CompanionConfig([string]$Dir, [string]$Url, [bool]$Created) {
    $file = Join-Path $Dir 'configs\host.yaml'
    if ($Created) {
        Set-YamlKey $file 'report_name' '中央本机' '中央安装时写入；控制台改名后列表名不再被这份配置覆盖'
        Set-YamlKey $file 'description' '中央服务所在机器的主机代理' '非空时每次上报覆盖控制台备注'
    }
    Set-YamlKey $file 'server_url' $Url '中央地址，随 configs/server.yaml 的监听地址'
}

function Invoke-CompanionScript([string]$Script, [string]$Arg) {
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $Script $Arg
    if ($LASTEXITCODE -ne 0) {
        throw "执行失败: $Script"
    }
}

function Install-ServerCompanion() {
    if (-not (Test-BinaryPresent 'server')) {
        Write-Output '当前目录不是中央安装包，跳过本机主机代理'
        return
    }
    $archive = Join-Path $script:Root 'agents\flux-guard-agent-host.tar.gz'
    $dir = Join-Path $script:Root 'agents\flux-guard-agent-host'
    if (-not (Test-Path -LiteralPath $archive)) {
        throw "未找到 $archive。中央服务已登记，本机主机代理未安装。"
    }
    $stop = Join-Path $dir 'scripts\run\stop.ps1'
    if (Test-Path -LiteralPath $stop) {
        Invoke-CompanionScript $stop 'host'
    }
    $agents = Join-Path $script:Root 'agents'
    New-Item -ItemType Directory -Force -Path $agents | Out-Null
    & tar.exe -xzf $archive -C $agents
    if ($LASTEXITCODE -ne 0) { throw "解开主机代理包失败" }
    $hostYaml = Join-Path $dir 'configs\host.yaml'
    $created = -not (Test-Path -LiteralPath $hostYaml)
    if ($created) {
        $example = Join-Path $dir 'configs\host.yaml.example'
        if (-not (Test-Path -LiteralPath $example)) {
            throw '主机代理包里没有 configs/host.yaml.example'
        }
        Copy-Item -LiteralPath $example -Destination $hostYaml
    }
    $url = Get-ServerCompanionUrl
    Set-CompanionConfig $dir $url $created
    Invoke-CompanionScript (Join-Path $dir 'scripts\run\install.ps1') 'host'
    Write-Output "本机主机代理已登记：$dir"
    Write-Output "上报地址 $url。令牌用该目录 configs\host.yaml 的 agent_token。在环境设置里更换后，请同步改这一行。"
}

function Uninstall-ServerCompanion() {
    $script = Join-Path $script:Root 'agents\flux-guard-agent-host\scripts\run\uninstall.ps1'
    if (-not (Test-Path -LiteralPath $script)) { return }
    Invoke-CompanionScript $script 'host'
}
