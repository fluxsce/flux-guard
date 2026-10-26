param(
    [Parameter(Position = 0)]
    [string]$Name
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'common.ps1')
. (Join-Path $PSScriptRoot 'server-companion.ps1')
if (-not $Name) { $Name = Resolve-Name $Name }
Uninstall-Boot $Name
if ($Name -eq 'server') { Uninstall-ServerCompanion }
