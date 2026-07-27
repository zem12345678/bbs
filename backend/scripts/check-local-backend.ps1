param(
  [string[]]$Services = @(),
  [ValidateSet("minimal", "commercial", "all")]
  [string]$Profile = "commercial",
  [int]$UserPort = 0,
  [int]$MallPort = 0,
  [int]$SearchPort = 0,
  [int]$ChatPort = 0,
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
  "chat-service" = 9116
  "file-service" = 9111
  "api-gateway" = 18080
}

function Get-ServicePortEnvironmentNames {
  param([string]$ServiceName)

  if ($ServiceName -eq "api-gateway") {
    return @("BBS_GATEWAY_SERVICE_HTTP_PORT")
  }
  if ($ServiceName -eq "user-service") {
    return @(
      "BBS_USER_GRPC_SERVER_PORT",
      "BBS_USER_GRPC_PORT",
      "BBS_USER_SERVICE_GRPC_PORT"
    )
  }
  if ($ServiceName -eq "chat-service") {
    return @(
      "BBS_CHAT_GRPC_SERVER_PORT",
      "BBS_CHAT_GRPC_PORT",
      "BBS_CHAT_SERVICE_GRPC_PORT"
    )
  }

  $prefix = ($ServiceName -replace "-service$", "" -replace "-", "_").ToUpperInvariant()
  return @(
    "BBS_${prefix}_GRPC_SERVER_PORT",
    "BBS_${prefix}_SERVICE_GRPC_PORT"
  )
}

function Resolve-ServicePortOverride {
  param(
    [string]$ServiceName,
    [int]$ExplicitPort = 0
  )

  if ($ExplicitPort -ne 0) {
    if ($ExplicitPort -lt 1 -or $ExplicitPort -gt 65535) {
      throw "Invalid explicit port for $ServiceName`: $ExplicitPort"
    }
    return $ExplicitPort
  }

  foreach ($name in Get-ServicePortEnvironmentNames $ServiceName) {
    $value = [Environment]::GetEnvironmentVariable($name, "Process")
    if ([string]::IsNullOrWhiteSpace($value)) {
      continue
    }
    $parsed = 0
    if (-not [int]::TryParse($value, [ref]$parsed) -or $parsed -lt 1 -or $parsed -gt 65535) {
      throw "Invalid port in $name`: $value"
    }
    return $parsed
  }

  return 0
}

$explicitPorts = @{
  "user-service" = $UserPort
  "mall-service" = $MallPort
  "search-service" = $SearchPort
  "chat-service" = $ChatPort
  "api-gateway" = $GatewayPort
}
foreach ($serviceName in $ServicePorts.Keys) {
  $explicitPort = if ($explicitPorts.ContainsKey($serviceName)) { $explicitPorts[$serviceName] } else { 0 }
  $resolvedPort = Resolve-ServicePortOverride -ServiceName $serviceName -ExplicitPort $explicitPort
  if ($resolvedPort -gt 0) {
    $ServicePorts[$serviceName] = $resolvedPort
  }
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
    "chat-service",
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
  $discoveryReady = -not $discoveryRequired -or (@($discoveryAddresses | Where-Object { ([string]$_).EndsWith(":$port", [System.StringComparison]::Ordinal) }).Count -gt 0)

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
