param(
  [string[]]$Services = @("admin-service", "api-gateway"),
  [switch]$All,
  [switch]$Strict
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$ServicesRoot = Join-Path $RepoRoot "backend\services"

$ServicePorts = [ordered]@{
  "user-service" = 9102
  "content-service" = 9103
  "comment-service" = 9104
  "reaction-service" = 9105
  "search-service" = 9106
  "credit-service" = 9107
  "notification-service" = 9108
  "feed-service" = 9113
  "admin-service" = 9114
  "api-gateway" = 18080
}

if ($All) {
  $Services = @($ServicePorts.Keys)
} else {
  $Services = @($Services | ForEach-Object { $_ -split "," } | ForEach-Object { $_.Trim() } | Where-Object { $_ })
}

foreach ($serviceName in $Services) {
  if (-not $ServicePorts.Contains($serviceName)) {
    throw "Unknown service '$serviceName'. Available: $($ServicePorts.Keys -join ', ')"
  }
}

$rows = foreach ($serviceName in $Services) {
  $port = $ServicePorts[$serviceName]
  $serviceDir = Join-Path $ServicesRoot $serviceName
  $expectedExe = Join-Path $serviceDir "bin\$serviceName.exe"
  $processes = @(Get-CimInstance Win32_Process -Filter "name='$serviceName.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.ExecutablePath -eq $expectedExe -or $_.CommandLine -like "*$serviceName.exe*" })
  $listeners = @(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue)
  $processIds = ($processes | ForEach-Object { $_.ProcessId }) -join ","
  $listenerIds = ($listeners | ForEach-Object { $_.OwningProcess } | Sort-Object -Unique) -join ","

  [pscustomobject]@{
    Service = $serviceName
    Port = $port
    ProcessIds = $processIds
    ListeningPids = $listenerIds
    Ready = ($processes.Count -gt 0 -and $listeners.Count -gt 0)
  }
}

$rows | Format-Table -AutoSize

if ($Strict -and (($rows | Where-Object { -not $_.Ready }).Count -gt 0)) {
  exit 1
}
