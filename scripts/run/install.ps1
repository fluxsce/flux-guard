param(
    [Parameter(Position = 0)]
    [string]$Name
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'common.ps1')
Install-Boot $Name
