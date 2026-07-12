param(
  [string[]]$Services = @(),
  [ValidateSet("minimal", "commercial", "all")]
  [string]$Profile = "commercial",
  [int]$MallPort = 0,
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
  "mall-service" = 9115
  "api-gateway" = 18080
}

if ($MallPort -gt 0) {
  $ServicePorts["mall-service"] = $MallPort
}

$ServiceProfiles = [ordered]@{
  minimal = @("admin-service", "api-gateway")
  commercial = @(
    "user-service",
    "content-service",
    "comment-service",
    "reaction-service",
    "search-service",
    "credit-service",
    "notification-service",
    "feed-service",
    "admin-service",
    "mall-service",
    "api-gateway"
  )
  all = @($ServicePorts.Keys)
}

if ($Services.Count -gt 0) {
  $Services = @($Services | ForEach-Object { $_ -split "," } | ForEach-Object { $_.Trim() } | Where-Object { $_ })
} elseif ($All) {
  $Services = @($ServiceProfiles.all)
} else {
  $Services = @($ServiceProfiles[$Profile])
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
    Where-Object { $_.ExecutablePath -eq $expectedExe })
  $listeners = @(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue)
  $processIdValues = @($processes | ForEach-Object { [int]$_.ProcessId })
  $listenerIdValues = @($listeners | ForEach-Object { [int]$_.OwningProcess } | Sort-Object -Unique)
  $serviceListenerIdValues = @($listenerIdValues | Where-Object { $processIdValues -contains $_ })
  $conflictListenerIdValues = @($listenerIdValues | Where-Object { $processIdValues -notcontains $_ })
  $processIds = ($processIdValues | Sort-Object -Unique) -join ","
  $listenerIds = ($serviceListenerIdValues | Sort-Object -Unique) -join ","
  $conflictListenerIds = ($conflictListenerIdValues | Sort-Object -Unique) -join ","

  [pscustomobject]@{
    Service = $serviceName
    Port = $port
    ProcessIds = $processIds
    ListeningPids = $listenerIds
    ConflictPids = $conflictListenerIds
    Ready = ($processes.Count -gt 0 -and $serviceListenerIdValues.Count -gt 0)
  }
}

$rows | Format-Table -AutoSize

if ($Strict -and (($rows | Where-Object { -not $_.Ready }).Count -gt 0)) {
  exit 1
}
