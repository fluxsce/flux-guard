# 只检查本机主机代理的地址和配置改写，不登记系统服务。
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'server-companion.ps1')

function Assert-Same([string]$Got, [string]$Want, [string]$Label) {
    if ($Got -ne $Want) { throw "$Label got=$Got want=$Want" }
}

Assert-Same (Get-AgentServerUrl ':8080' 'false') 'http://127.0.0.1:8080' 'any'
Assert-Same (Get-AgentServerUrl '0.0.0.0:8080' 'false') 'http://127.0.0.1:8080' 'zero'
Assert-Same (Get-AgentServerUrl '[::]:8080' 'false') 'http://127.0.0.1:8080' 'ipv6'
Assert-Same (Get-AgentServerUrl '127.0.0.1:9090' 'false') 'http://127.0.0.1:9090' 'loop'
Assert-Same (Get-AgentServerUrl '10.0.0.5:9090' 'true') 'https://10.0.0.5:9090' 'bind'
Assert-Same (Get-AgentServerUrl '' 'false') 'http://127.0.0.1:8080' 'empty'

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("fg-companion-" + [guid]::NewGuid().ToString('n'))
New-Item -ItemType Directory -Force -Path (Join-Path $tmp 'agent\configs') | Out-Null
try {
    $server = Join-Path $tmp 'server.yaml'
    $utf8 = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($server, "listen: `":8080`" # 本程序监听的地址`nenable_https: false`n", $utf8)
    Assert-Same (Get-YamlScalar $server 'listen') ':8080' 'listen'
    Assert-Same (Get-YamlScalar $server 'enable_https') 'false' 'https'

    $example = Join-Path $PSScriptRoot '..\..\configs\host.yaml.example'
    $hostYaml = Join-Path $tmp 'agent\configs\host.yaml'
    Copy-Item -LiteralPath $example -Destination $hostYaml
    Set-CompanionConfig (Join-Path $tmp 'agent') 'http://127.0.0.1:9090' $true
    Assert-Same (Get-YamlScalar $hostYaml 'server_url') 'http://127.0.0.1:9090' 'create-url'
    Assert-Same (Get-YamlScalar $hostYaml 'report_name') '中央本机' 'create-name'
    Assert-Same (Get-YamlScalar $hostYaml 'agent_token') 'agt_dev_local' 'create-token'

    Set-YamlKey $hostYaml 'agent_token' 'agt_rotated' ''
    Set-CompanionConfig (Join-Path $tmp 'agent') 'https://10.0.0.8:8443' $false
    Assert-Same (Get-YamlScalar $hostYaml 'server_url') 'https://10.0.0.8:8443' 'update-url'
    Assert-Same (Get-YamlScalar $hostYaml 'report_name') '中央本机' 'keep-name'
    Assert-Same (Get-YamlScalar $hostYaml 'agent_token') 'agt_rotated' 'keep-token'
} finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
Write-Output 'server-companion ok'
