param(
  [string[]]$Services = @(),
  [ValidateSet("minimal", "commercial", "all")]
  [string]$Profile = "commercial",
  [int]$MallPort = 0,
  [int]$SearchPort = 0,
  [int]$GatewayPort = 0,
  [switch]$All,
  [switch]$RequireDiscovery,
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
  "file-service" = 9111
  "api-gateway" = 18080
}

function Resolve-MallPortOverride {
  param([int]$ExplicitPort)

  if ($ExplicitPort -gt 0) {
    return $ExplicitPort
  }

  foreach ($name in @("BBS_MALL_GRPC_SERVER_PORT", "BBS_MALL_SERVICE_GRPC_PORT")) {
    $value = [Environment]::GetEnvironmentVariable($name, "Process")
    $parsed = 0
    if (-not [string]::IsNullOrWhiteSpace($value) -and [int]::TryParse($value, [ref]$parsed) -and $parsed -gt 0) {
      return $parsed
    }
  }

  return 0
}

function Resolve-SearchPortOverride {
  param([int]$ExplicitPort)

  if ($ExplicitPort -gt 0) {
    return $ExplicitPort
  }

  foreach ($name in @("BBS_SEARCH_GRPC_SERVER_PORT", "BBS_SEARCH_SERVICE_GRPC_PORT")) {
    $value = [Environment]::GetEnvironmentVariable($name, "Process")
    $parsed = 0
    if (-not [string]::IsNullOrWhiteSpace($value) -and [int]::TryParse($value, [ref]$parsed) -and $parsed -gt 0) {
      return $parsed
    }
  }

  return 0
}

$resolvedMallPort = Resolve-MallPortOverride $MallPort
if ($resolvedMallPort -gt 0) {
  $ServicePorts["mall-service"] = $resolvedMallPort
}
$resolvedSearchPort = Resolve-SearchPortOverride $SearchPort
if ($resolvedSearchPort -gt 0) {
  $ServicePorts["search-service"] = $resolvedSearchPort
}
if ($GatewayPort -gt 0) {
  $ServicePorts["api-gateway"] = $GatewayPort
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
    "file-service",
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

function Get-ServiceDiscoveryAddresses {
  param([string]$ServiceName)

  $discoveryName = "bbs-$ServiceName"
  $prefix = "/$discoveryName/"
  $rangeEnd = $prefix.Substring(0, $prefix.Length - 1) + [char]([int][char]$prefix[$prefix.Length - 1] + 1)
  $request = @{
    key = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($prefix))
    range_end = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($rangeEnd))
  } | ConvertTo-Json -Compress

  try {
    $response = Invoke-RestMethod -Uri "http://127.0.0.1:2379/v3/kv/range" -Method Post -ContentType "application/json" -Body $request -TimeoutSec 5
  } catch {
    throw "Unable to inspect etcd registrations for ${discoveryName}: $($_.Exception.Message)"
  }

  return @(
    @($response.kvs | Where-Object { $null -ne $_ }) |
      ForEach-Object {
        try {
          $value = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String([string]$_.value)) | ConvertFrom-Json
          [string]$value.addr
        } catch {
          throw "Invalid etcd registration for ${discoveryName}: $($_.Exception.Message)"
        }
      } |
      Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
      Sort-Object -Unique
  )
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
  $discoveryRequired = $RequireDiscovery -and $serviceName -ne "api-gateway"
  [string[]]$discoveryAddresses = if ($discoveryRequired) { @(Get-ServiceDiscoveryAddresses $serviceName) } else { @() }
  $discoveryReady = -not $discoveryRequired -or ($discoveryAddresses.Count -eq 1 -and $discoveryAddresses[0].EndsWith(":$port", [System.StringComparison]::Ordinal))

  [pscustomobject]@{
    Service = $serviceName
    Port = $port
    ProcessIds = $processIds
    ListeningPids = $listenerIds
    ConflictPids = $conflictListenerIds
    EtcdAddresses = $discoveryAddresses -join ","
    DiscoveryReady = $discoveryReady
    Ready = ($processes.Count -gt 0 -and $serviceListenerIdValues.Count -gt 0 -and $discoveryReady)
  }
}

$rows | Format-Table -AutoSize

$notReady = @($rows | Where-Object { $_.Ready -ne $true })
if ($Strict -and $notReady.Count -gt 0) {
  throw "Backend service health check failed: $($notReady.Service -join ', ')"
}
