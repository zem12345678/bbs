<#
.SYNOPSIS
  Explicit opt-in end-to-end verification for the two-node chat cluster.

.DESCRIPTION
  With -Run, this script starts only the four processes that it owns:
  chat-service on 9116 and 19116 by default, and API Gateway on 18080 and
  18081 by default. The four ports can be overridden for a host shared with
  other projects.
  It never starts, resets, restarts, or stops local infrastructure or an
  existing service. Ports and existing chat-service registrations must be
  free before it starts. Unless -KeepRunning is supplied, only the child PIDs
  started by this invocation are stopped when the verification finishes.

  Required already-running dependencies are PostgreSQL, Redis, Kafka, etcd,
  Nacos, and bbs-user-service registered in etcd. The script intentionally
  uses the normal checked-in Nacos/Wire/IoC configuration paths.

.EXAMPLE
  .\backend\scripts\chat-cluster-e2e.ps1 -Preflight

.EXAMPLE
  .\backend\scripts\chat-cluster-e2e.ps1 -Run -Build

.EXAMPLE
  .\backend\scripts\chat-cluster-e2e.ps1 -Preflight -PrimaryChatPort 19117 -SecondaryChatPort 19118 -PrimaryGatewayPort 18082 -SecondaryGatewayPort 18083

.NOTES
  The test creates two timestamped users and one room; it does not remove
  product data. -Run is required for process creation or API writes.
#>

[CmdletBinding()]
param(
  [switch]$Run,
  [switch]$Preflight,
  [switch]$Build,
  [switch]$KeepRunning,
  [string]$EnvironmentFile = "",
  [ValidateRange(10, 300)]
  [int]$WaitSeconds = 45,
  [ValidateRange(5, 120)]
  [int]$EventWaitSeconds = 20,
  [ValidateRange(1, 65535)]
  [int]$PrimaryChatPort = 9116,
  [ValidateRange(1, 65535)]
  [int]$SecondaryChatPort = 19116,
  [ValidateRange(1, 65535)]
  [int]$PrimaryGatewayPort = 18080,
  [ValidateRange(1, 65535)]
  [int]$SecondaryGatewayPort = 18081,
  [ValidateRange(0, 1023)]
  [int]$PrimaryChatWorkerID = 16,
  [ValidateRange(0, 1023)]
  [int]$SecondaryChatWorkerID = 17
)

$ErrorActionPreference = "Stop"

if (-not $Run -and -not $Preflight) {
  Write-Host "chat-cluster-e2e is opt-in; no processes or API writes were performed."
  Write-Host "Run -Preflight to check prerequisites, or -Run [-Build] to execute the two-chat/two-gateway verification."
  return
}

$RepoRoot = [string](Resolve-Path (Join-Path $PSScriptRoot "..\.."))
$ServicesRoot = Join-Path $RepoRoot "backend\services"
$ChatServiceDir = Join-Path $ServicesRoot "chat-service"
$GatewayServiceDir = Join-Path $ServicesRoot "api-gateway"
$ChatExecutable = Join-Path $ChatServiceDir "bin\chat-service.exe"
$GatewayExecutable = Join-Path $GatewayServiceDir "bin\api-gateway.exe"
$ChatServiceName = "bbs-chat-service"
$UserServiceName = "bbs-user-service"
$ChatKafkaTopic = "chat.events"
$ChatRealtimeGroupID = "bbs-chat-realtime"

$clusterPorts = @($PrimaryChatPort, $SecondaryChatPort, $PrimaryGatewayPort, $SecondaryGatewayPort)
if (@($clusterPorts | Select-Object -Unique).Count -ne $clusterPorts.Count) {
  throw "Primary/secondary chat and Gateway ports must all be distinct."
}
if ($PrimaryChatWorkerID -eq $SecondaryChatWorkerID) {
  throw "Primary and secondary chat Snowflake worker IDs must be distinct."
}

function Restore-ProcessEnvironment {
  param([hashtable]$Previous)

  if ($null -eq $Previous) {
    return
  }

  foreach ($entry in $Previous.GetEnumerator()) {
    if ($null -eq $entry.Value) {
      [Environment]::SetEnvironmentVariable([string]$entry.Key, $null, "Process")
    } else {
      [Environment]::SetEnvironmentVariable([string]$entry.Key, [string]$entry.Value, "Process")
    }
  }
}

function Import-ProcessEnvironmentFile {
  param([string]$Path)

  if ([string]::IsNullOrWhiteSpace($Path)) {
    return @{}
  }
  if (-not (Test-Path -LiteralPath $Path)) {
    throw "Environment file not found: $Path"
  }

  # Parse the whole file before changing the caller's process environment so a
  # malformed later entry cannot leave earlier values behind.
  $entries = [ordered]@{}
  $loaded = 0
  foreach ($line in Get-Content -LiteralPath $Path) {
    $value = $line.Trim()
    if ($value.Length -eq 0 -or $value.StartsWith("#")) {
      continue
    }
    $separator = $value.IndexOf("=")
    if ($separator -lt 1) {
      throw "Invalid environment entry in ${Path}: $line"
    }
    $name = $value.Substring(0, $separator).Trim()
    $content = $value.Substring($separator + 1).Trim()
    if ([string]::IsNullOrWhiteSpace($name)) {
      throw "Invalid environment entry in ${Path}: $line"
    }
    if ($content.Length -ge 2 -and (($content.StartsWith('"') -and $content.EndsWith('"')) -or ($content.StartsWith("'") -and $content.EndsWith("'")))) {
      $content = $content.Substring(1, $content.Length - 2)
    }
    $entries[$name] = $content
    $loaded++
  }

  $previous = @{}
  foreach ($entry in $entries.GetEnumerator()) {
    $previous[[string]$entry.Key] = [Environment]::GetEnvironmentVariable([string]$entry.Key, "Process")
  }
  try {
    foreach ($entry in $entries.GetEnumerator()) {
      [Environment]::SetEnvironmentVariable([string]$entry.Key, [string]$entry.Value, "Process")
    }
  } catch {
    Restore-ProcessEnvironment $previous
    throw
  }
  Write-Host "Loaded $loaded process environment values from $Path."
  return $previous
}

function Get-FirstEnvironmentValue {
  param(
    [string[]]$Names,
    [string]$Fallback
  )

  foreach ($name in $Names) {
    $value = [Environment]::GetEnvironmentVariable($name, "Process")
    if (-not [string]::IsNullOrWhiteSpace($value)) {
      return $value.Trim()
    }
  }
  return $Fallback
}

function Get-FirstCommaSeparatedValue {
  param([string]$Value)

  foreach ($part in $Value.Split(",")) {
    $trimmed = $part.Trim()
    if (-not [string]::IsNullOrWhiteSpace($trimmed)) {
      return $trimmed
    }
  }
  return ""
}

function Get-EndpointParts {
  param(
    [string]$Endpoint,
    [int]$DefaultPort,
    [string]$Description
  )

  $value = Get-FirstCommaSeparatedValue $Endpoint
  if ([string]::IsNullOrWhiteSpace($value)) {
    throw "$Description endpoint is empty"
  }
  if ($value -notmatch "://") {
    $value = "tcp://$value"
  }
  try {
    $uri = [Uri]$value
  } catch {
    throw "Invalid $Description endpoint '$Endpoint': $($_.Exception.Message)"
  }
  if ([string]::IsNullOrWhiteSpace($uri.Host)) {
    throw "Invalid $Description endpoint '$Endpoint'"
  }
  $port = $uri.Port
  if ($port -le 0) {
    $port = $DefaultPort
  }
  $address = if ($uri.Host.Contains(":")) { "[$($uri.Host)]:$port" } else { "$($uri.Host):$port" }
  return [pscustomobject]@{
    Host = $uri.Host
    Port = $port
    Address = $address
  }
}

function Test-TcpEndpoint {
  param(
    [string]$TargetHost,
    [int]$Port,
    [int]$TimeoutMilliseconds = 2000
  )

  $client = New-Object System.Net.Sockets.TcpClient
  try {
    $async = $client.BeginConnect($TargetHost, $Port, $null, $null)
    if (-not $async.AsyncWaitHandle.WaitOne($TimeoutMilliseconds)) {
      return $false
    }
    $client.EndConnect($async)
    return $true
  } catch {
    return $false
  } finally {
    $client.Close()
  }
}

function Assert-TcpEndpoint {
  param(
    [pscustomobject]$Endpoint,
    [string]$Description
  )

  if (-not (Test-TcpEndpoint -TargetHost $Endpoint.Host -Port $Endpoint.Port)) {
    throw "$Description is not reachable at $($Endpoint.Address). Start or repair it before running this cluster test."
  }
  Write-Host "Prerequisite reachable: $Description at $($Endpoint.Address)."
}

function Get-ProcessSummary {
  param([int]$ProcessId)

  $process = Get-CimInstance Win32_Process -Filter "ProcessId=$ProcessId" -ErrorAction SilentlyContinue
  if ($null -eq $process) {
    return "pid=$ProcessId"
  }
  $path = [string]$process.ExecutablePath
  if ([string]::IsNullOrWhiteSpace($path)) {
    $path = [string]$process.Name
  }
  return "pid=$ProcessId path=$path"
}

function Assert-PortIsFree {
  param(
    [int]$Port,
    [string]$Purpose
  )

  $listeners = @(
    Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
      ForEach-Object { [int]$_.OwningProcess } |
      Sort-Object -Unique
  )
  if ($listeners.Count -eq 0) {
    return
  }
  $details = @($listeners | ForEach-Object { Get-ProcessSummary $_ }) -join "; "
  throw "$Purpose requires local port $Port, but it is occupied by $details. This script will not stop that process."
}

function Assert-NoManagedProcess {
  param(
    [string]$ExecutablePath,
    [string]$ServiceName
  )

  $exeName = [System.IO.Path]::GetFileName($ExecutablePath)
  $processes = @(
    Get-CimInstance Win32_Process -Filter "name='$exeName'" -ErrorAction SilentlyContinue |
      Where-Object { $_.ExecutablePath -and $_.ExecutablePath.Equals($ExecutablePath, [System.StringComparison]::OrdinalIgnoreCase) }
  )
  if ($processes.Count -eq 0) {
    return
  }
  $details = @($processes | ForEach-Object { "pid=$($_.ProcessId)" }) -join ", "
  throw "$ServiceName is already running from $ExecutablePath ($details). This script does not attach to or restart existing service processes."
}

function Get-EtcdServiceAddresses {
  param(
    [pscustomobject]$EtcdEndpoint,
    [string]$ServiceName
  )

  $prefix = "/$ServiceName/"
  $lastCharacter = [char]$prefix[$prefix.Length - 1]
  $rangeEnd = $prefix.Substring(0, $prefix.Length - 1) + [char]([int][char]$lastCharacter + 1)
  $request = @{
    key = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($prefix))
    range_end = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($rangeEnd))
  } | ConvertTo-Json -Compress
  $uri = "http://$($EtcdEndpoint.Address)/v3/kv/range"
  try {
    $response = Invoke-RestMethod -Uri $uri -Method Post -ContentType "application/json" -Body $request -TimeoutSec 5
  } catch {
    throw "Unable to inspect etcd registrations for $ServiceName at ${uri}: $($_.Exception.Message)"
  }

  return @(
    @($response.kvs | Where-Object { $null -ne $_ }) |
      ForEach-Object {
        try {
          $raw = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String([string]$_.value))
          $registration = $raw | ConvertFrom-Json
          [string]$registration.addr
        } catch {
          throw "Invalid etcd registration for ${ServiceName}: $($_.Exception.Message)"
        }
      } |
      Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
      Sort-Object -Unique
  )
}

function Wait-ChatClusterRegistration {
  param(
    [pscustomobject]$EtcdEndpoint,
    [int]$TimeoutSeconds
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  $lastAddresses = @()
  $lastError = ""
  while ((Get-Date) -lt $deadline) {
    try {
      $addresses = @(Get-EtcdServiceAddresses -EtcdEndpoint $EtcdEndpoint -ServiceName $ChatServiceName)
      $lastAddresses = $addresses
      $hasPrimary = @($addresses | Where-Object { $_.EndsWith(":$PrimaryChatPort", [System.StringComparison]::Ordinal) }).Count -eq 1
      $hasSecondary = @($addresses | Where-Object { $_.EndsWith(":$SecondaryChatPort", [System.StringComparison]::Ordinal) }).Count -eq 1
      if ($addresses.Count -eq 2 -and $hasPrimary -and $hasSecondary) {
        return $addresses
      }
    } catch {
      $lastError = $_.Exception.Message
    }
    Start-Sleep -Milliseconds 300
  }
  $observed = if ($lastAddresses.Count -gt 0) { $lastAddresses -join ", " } else { "none" }
  $suffix = if ([string]::IsNullOrWhiteSpace($lastError)) { "" } else { " Last etcd error: $lastError" }
  throw "Expected exactly two $ChatServiceName registrations on ports $PrimaryChatPort and $SecondaryChatPort; observed: $observed.$suffix"
}

function Wait-ChatClusterDeregistration {
  param(
    [pscustomobject]$EtcdEndpoint,
    [int]$TimeoutSeconds = 20
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  $targetSuffixes = @(
    ":$PrimaryChatPort",
    ":$SecondaryChatPort"
  )
  $lastAddresses = @()
  $lastError = ""
  while ((Get-Date) -lt $deadline) {
    try {
      $lastAddresses = @(Get-EtcdServiceAddresses -EtcdEndpoint $EtcdEndpoint -ServiceName $ChatServiceName)
      $remaining = @($lastAddresses | Where-Object {
        $address = [string]$_
        @($targetSuffixes | Where-Object { $address.EndsWith($_, [System.StringComparison]::Ordinal) }).Count -gt 0
      })
      if ($remaining.Count -eq 0) {
        return $true
      }
    } catch {
      $lastError = $_.Exception.Message
    }
    Start-Sleep -Milliseconds 300
  }
  $remaining = @($lastAddresses | Where-Object {
    $address = [string]$_
    @($targetSuffixes | Where-Object { $address.EndsWith($_, [System.StringComparison]::Ordinal) }).Count -gt 0
  })
  $detail = if ($remaining.Count -gt 0) { $remaining -join ', ' } elseif (-not [string]::IsNullOrWhiteSpace($lastError)) { "last etcd error: $lastError" } else { "none" }
  Write-Warning "Owned chat-service processes stopped, but test registrations remain unverified: $detail"
  return $false
}

function Invoke-ClusterBuild {
  param(
    [string]$ServiceDirectory,
    [string]$OutputPath,
    [string]$ServiceName
  )

  $go = Get-Command go -ErrorAction SilentlyContinue
  if ($null -eq $go) {
    throw "go is required for -Build but was not found on PATH"
  }
  Push-Location $ServiceDirectory
  try {
    Write-Host "Building $ServiceName..."
    & $go.Source build -o $OutputPath ".\cmd"
    if ($LASTEXITCODE -ne 0) {
      throw "go build failed for $ServiceName with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }
}

function Assert-ExecutableExists {
  param(
    [string]$Path,
    [string]$ServiceName
  )

  if (-not (Test-Path -LiteralPath $Path)) {
    throw "$ServiceName binary not found: $Path. Run this script with -Build."
  }
}

function Start-OwnedService {
  param(
    [string]$Name,
    [string]$ExecutablePath,
    [string]$WorkingDirectory,
    [string[]]$Arguments,
    [hashtable]$Environment,
    [string]$StandardOutputPath,
    [string]$StandardErrorPath
  )

  $previous = @{}
  foreach ($entry in $Environment.GetEnumerator()) {
    $previous[$entry.Key] = [Environment]::GetEnvironmentVariable([string]$entry.Key, "Process")
  }
  try {
    foreach ($entry in $Environment.GetEnumerator()) {
      [Environment]::SetEnvironmentVariable([string]$entry.Key, [string]$entry.Value, "Process")
    }
    $process = Start-Process -FilePath $ExecutablePath -ArgumentList $Arguments -WorkingDirectory $WorkingDirectory `
      -PassThru -WindowStyle Hidden -RedirectStandardOutput $StandardOutputPath -RedirectStandardError $StandardErrorPath
  } finally {
    foreach ($entry in $previous.GetEnumerator()) {
      if ($null -eq $entry.Value) {
        [Environment]::SetEnvironmentVariable([string]$entry.Key, $null, "Process")
      } else {
        [Environment]::SetEnvironmentVariable([string]$entry.Key, [string]$entry.Value, "Process")
      }
    }
  }
  Write-Host "Started $Name (pid $($process.Id))."
  return $process
}

function Get-ProcessLogTail {
  param(
    [string]$Path,
    [int]$Lines = 40
  )

  if (-not (Test-Path -LiteralPath $Path)) {
    return ""
  }
  try {
    return (Get-Content -LiteralPath $Path -Tail $Lines -ErrorAction Stop) -join [Environment]::NewLine
  } catch {
    return ""
  }
}

function Assert-ProcessAlive {
  param(
    [System.Diagnostics.Process]$Process,
    [string]$Name,
    [string]$StandardOutputPath,
    [string]$StandardErrorPath
  )

  if (-not $Process.HasExited) {
    return
  }
  $stdout = Get-ProcessLogTail $StandardOutputPath
  $stderr = Get-ProcessLogTail $StandardErrorPath
  throw "$Name exited before becoming ready (exit code $($Process.ExitCode)). stdout:`n$stdout`nstderr:`n$stderr"
}

function Wait-ProcessPort {
  param(
    [System.Diagnostics.Process]$Process,
    [int]$Port,
    [string]$Name,
    [string]$StandardOutputPath,
    [string]$StandardErrorPath,
    [int]$TimeoutSeconds
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    Assert-ProcessAlive -Process $Process -Name $Name -StandardOutputPath $StandardOutputPath -StandardErrorPath $StandardErrorPath
    $listener = @(
      Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Where-Object { [int]$_.OwningProcess -eq [int]$Process.Id }
    )
    if ($listener.Count -gt 0) {
      return
    }
    Start-Sleep -Milliseconds 250
  }
  throw "$Name (pid $($Process.Id)) did not listen on port $Port within $TimeoutSeconds seconds. stdout:`n$(Get-ProcessLogTail $StandardOutputPath)`nstderr:`n$(Get-ProcessLogTail $StandardErrorPath)"
}

function Wait-GatewayReady {
  param(
    [System.Diagnostics.Process]$Process,
    [string]$BaseUrl,
    [string]$Name,
    [string]$StandardOutputPath,
    [string]$StandardErrorPath,
    [int]$TimeoutSeconds
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    Assert-ProcessAlive -Process $Process -Name $Name -StandardOutputPath $StandardOutputPath -StandardErrorPath $StandardErrorPath
    try {
      $response = Invoke-WebRequest -Uri "$BaseUrl/metrics" -UseBasicParsing -TimeoutSec 3
      if ($response.StatusCode -eq 200) {
        return
      }
    } catch {
    }
    Start-Sleep -Milliseconds 300
  }
  throw "$Name did not serve /metrics within $TimeoutSeconds seconds. stdout:`n$(Get-ProcessLogTail $StandardOutputPath)`nstderr:`n$(Get-ProcessLogTail $StandardErrorPath)"
}

function Invoke-BbsApi {
  param(
    [string]$Uri,
    [string]$Method = "GET",
    [hashtable]$Headers,
    [string]$Body = ""
  )

  $params = @{ Uri = $Uri; Method = $Method; TimeoutSec = 15; ErrorAction = "Stop" }
  if ($null -ne $Headers) {
    $params.Headers = $Headers
  }
  if (-not [string]::IsNullOrWhiteSpace($Body)) {
    $params.ContentType = "application/json"
    $params.Body = $Body
  }
  $response = Invoke-RestMethod @params
  if ($null -ne $response -and $null -ne $response.PSObject.Properties["code"] -and [int64]$response.code -ne 0) {
    throw "API error $($response.code): $($response.message) from $Uri"
  }
  if ($null -ne $response -and $null -ne $response.PSObject.Properties["data"]) {
    return $response.data
  }
  return $response
}

function Register-ClusterUser {
  param(
    [string]$BaseUrl,
    [string]$Prefix,
    [string]$Stamp
  )

  $username = "$Prefix$Stamp"
  $registration = Invoke-BbsApi -Uri "$BaseUrl/api/v1/auth/register" -Method Post -Body (@{
    username = $username
    email = "$username@example.com"
    password = "Password123!"
    nickname = "Chat Cluster $Prefix"
  } | ConvertTo-Json -Compress)
  if ([string]::IsNullOrWhiteSpace([string]$registration.access_token) -or [string]::IsNullOrWhiteSpace([string]$registration.user.id)) {
    throw "Registration through $BaseUrl did not return an access token and user ID for $username"
  }
  return [pscustomobject]@{
    Username = $username
    ID = [string]$registration.user.id
    Token = [string]$registration.access_token
    Headers = @{ Authorization = "Bearer $($registration.access_token)" }
  }
}

function Open-ChatWebSocket {
  param(
    [string]$BaseUrl,
    [string]$Ticket
  )

  $webSocketBase = $BaseUrl -replace "^http://", "ws://" -replace "^https://", "wss://"
  $uri = [Uri]("$webSocketBase/api/v1/chat/ws?ticket=$([Uri]::EscapeDataString($Ticket))")
  $socket = New-Object System.Net.WebSockets.ClientWebSocket
  $cancellation = New-Object System.Threading.CancellationTokenSource
  try {
    $cancellation.CancelAfter(10000)
    $connectTask = $socket.ConnectAsync($uri, $cancellation.Token)
    [void]$connectTask.GetAwaiter().GetResult()
    return $socket
  } catch {
    $socket.Dispose()
    throw "WebSocket connect to $uri failed: $($_.Exception.Message)"
  } finally {
    $cancellation.Dispose()
  }
}

function Close-ChatWebSocket {
  param([System.Net.WebSockets.ClientWebSocket]$Socket)

  if ($null -eq $Socket) {
    return
  }
  try {
    $Socket.Abort()
  } catch {
  }
  try {
    $Socket.Dispose()
  } catch {
  }
}

function Send-WebSocketJson {
  param(
    [System.Net.WebSockets.ClientWebSocket]$Socket,
    [hashtable]$Envelope
  )

  $raw = $Envelope | ConvertTo-Json -Depth 8 -Compress
  $bytes = [System.Text.Encoding]::UTF8.GetBytes($raw)
  $segment = [System.ArraySegment[byte]]::new($bytes, 0, $bytes.Length)
  $sendTask = $Socket.SendAsync($segment, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, [System.Threading.CancellationToken]::None)
  [void]$sendTask.GetAwaiter().GetResult()
}

function Receive-WebSocketJson {
  param(
    [System.Net.WebSockets.ClientWebSocket]$Socket,
    [int]$TimeoutMilliseconds
  )

  $buffer = New-Object byte[] 8192
  $stream = New-Object System.IO.MemoryStream
  $cancellation = New-Object System.Threading.CancellationTokenSource
  try {
    $cancellation.CancelAfter($TimeoutMilliseconds)
    do {
      $segment = [System.ArraySegment[byte]]::new($buffer, 0, $buffer.Length)
      $receiveTask = $Socket.ReceiveAsync($segment, $cancellation.Token)
      $result = $receiveTask.GetAwaiter().GetResult()
      if ($result.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
        throw "WebSocket closed by the server"
      }
      if ($result.MessageType -ne [System.Net.WebSockets.WebSocketMessageType]::Text) {
        throw "Unexpected WebSocket frame type $($result.MessageType)"
      }
      $stream.Write($buffer, 0, [int]$result.Count)
    } while (-not $result.EndOfMessage)
    $raw = [System.Text.Encoding]::UTF8.GetString($stream.ToArray())
    $event = $raw | ConvertFrom-Json
    $event | Add-Member -NotePropertyName "_raw" -NotePropertyValue $raw -Force
    return $event
  } finally {
    $cancellation.Dispose()
    $stream.Dispose()
  }
}

function Receive-WebSocketEventUntil {
  param(
    [System.Net.WebSockets.ClientWebSocket]$Socket,
    [string]$ExpectedType,
    [string]$RequestID = "",
    [int]$TimeoutSeconds
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  $observed = New-Object System.Collections.Generic.List[string]
  while ((Get-Date) -lt $deadline) {
    $remainingMilliseconds = [Math]::Max(100, [int](($deadline - (Get-Date)).TotalMilliseconds))
    try {
      $event = Receive-WebSocketJson -Socket $Socket -TimeoutMilliseconds $remainingMilliseconds
    } catch [System.OperationCanceledException] {
      continue
    }
    [void]$observed.Add([string]$event._raw)
    if ([string]$event.type -eq "error") {
      throw "WebSocket returned error while waiting for ${ExpectedType}: $($event._raw)"
    }
    $matchesType = [string]$event.type -eq $ExpectedType
    $matchesRequest = [string]::IsNullOrWhiteSpace($RequestID) -or [string]$event.request_id -eq $RequestID
    if ($matchesType -and $matchesRequest) {
      return $event
    }
  }
  $seen = if ($observed.Count -gt 0) { $observed -join [Environment]::NewLine } else { "none" }
  throw "Timed out waiting for WebSocket event '$ExpectedType' (request '$RequestID'). Observed:`n$seen"
}

function Get-ChatSidebarRoom {
  param(
    [string]$BaseUrl,
    [hashtable]$Headers,
    [string]$RoomNo
  )

  $sidebar = Invoke-BbsApi -Uri "$BaseUrl/api/v1/chat/sidebar" -Headers $Headers
  $matches = @($sidebar.rooms | Where-Object { [string]$_.room.room_no -eq $RoomNo })
  if ($matches.Count -ne 1) {
    throw "Expected exactly one sidebar entry for room $RoomNo through $BaseUrl, found $($matches.Count)"
  }
  return $matches[0]
}

function Stop-OwnedService {
  param([pscustomobject]$OwnedService)

  if ($null -eq $OwnedService -or $null -eq $OwnedService.Process -or [string]::IsNullOrWhiteSpace([string]$OwnedService.ExecutablePath)) {
    return $false
  }
  try {
    if ($OwnedService.Process.HasExited) {
      return $true
    }
    $expectedPath = [System.IO.Path]::GetFullPath([string]$OwnedService.ExecutablePath)
    $current = Get-CimInstance Win32_Process -Filter "ProcessId=$($OwnedService.Process.Id)" -ErrorAction Stop
    $actualPath = if ($null -eq $current) { "" } else { [string]$current.ExecutablePath }
    if ([string]::IsNullOrWhiteSpace($actualPath) -or -not [System.IO.Path]::GetFullPath($actualPath).Equals($expectedPath, [System.StringComparison]::OrdinalIgnoreCase)) {
      Write-Warning "Refusing to stop owned $($OwnedService.Name) process $($OwnedService.Process.Id): executable path no longer matches $expectedPath."
      return $false
    }
    # Kill on the Process object returned by Start-Process. Its process handle
    # remains bound to this invocation's child, unlike a new PID-only lookup.
    $OwnedService.Process.Kill()
    if (-not $OwnedService.Process.WaitForExit(5000)) {
      Write-Warning "Owned $($OwnedService.Name) process $($OwnedService.Process.Id) did not exit within 5 seconds."
      return $false
    }
    Write-Host "Stopped owned $($OwnedService.Name) process $($OwnedService.Process.Id)."
    return $true
  } catch {
    Write-Warning "Could not stop owned $($OwnedService.Name) process $($OwnedService.Process.Id): $($_.Exception.Message)"
    return $false
  }
}

function Write-OwnedServiceDiagnostics {
  param([object[]]$OwnedServices)

  foreach ($owned in $OwnedServices) {
    $stdout = Get-ProcessLogTail $owned.StandardOutputPath
    $stderr = Get-ProcessLogTail $owned.StandardErrorPath
    if (-not [string]::IsNullOrWhiteSpace($stdout) -or -not [string]::IsNullOrWhiteSpace($stderr)) {
      Write-Host "--- $($owned.Name) stdout tail ---"
      Write-Host $stdout
      Write-Host "--- $($owned.Name) stderr tail ---"
      Write-Host $stderr
    }
  }
}

$environmentSnapshot = Import-ProcessEnvironmentFile $EnvironmentFile

try {

if ($env:BBS_CHAT_SKIP_NACOS -match "^(?i:1|true|yes)$") {
  throw "BBS_CHAT_SKIP_NACOS is enabled. This cluster check requires the normal Nacos configuration path; unset it before running."
}

$etcdValue = Get-FirstEnvironmentValue -Names @("BBS_CHAT_GRPC_SERVER_ETCD_ADDR", "BBS_CHAT_ETCD_ADDR", "BBS_GATEWAY_GRPC_CLIENT_ETCD_ADDR") -Fallback "127.0.0.1:2379"
$etcdEndpoint = Get-EndpointParts -Endpoint $etcdValue -DefaultPort 2379 -Description "etcd"
$redisValue = Get-FirstEnvironmentValue -Names @("BBS_CHAT_REDIS_URL", "BBS_CHAT_REDIS_ADDR") -Fallback "127.0.0.1:6379"
$redisEndpoint = Get-EndpointParts -Endpoint $redisValue -DefaultPort 6379 -Description "Redis"
$kafkaValue = Get-FirstEnvironmentValue -Names @("BBS_CHAT_KAFKA_BROKERS") -Fallback "127.0.0.1:9092"
$kafkaEndpoint = Get-EndpointParts -Endpoint $kafkaValue -DefaultPort 9092 -Description "Kafka"
$postgresValue = Get-FirstEnvironmentValue -Names @("BBS_CHAT_POSTGRES_DSN") -Fallback "postgres://bbs_chat_app:local_chat_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_chat"
$postgresEndpoint = Get-EndpointParts -Endpoint $postgresValue -DefaultPort 5432 -Description "PostgreSQL"
$chatNacosHost = Get-FirstEnvironmentValue -Names @("BBS_CHAT_NACOS_ADDR") -Fallback "127.0.0.1"
$chatNacosPortValue = Get-FirstEnvironmentValue -Names @("BBS_CHAT_NACOS_PORT") -Fallback "8848"
$nacosPort = 0
if (-not [int]::TryParse($chatNacosPortValue, [ref]$nacosPort) -or $nacosPort -le 0) {
  throw "Invalid BBS_CHAT_NACOS_PORT value '$chatNacosPortValue'"
}
$chatNacosEndpoint = Get-EndpointParts -Endpoint "${chatNacosHost}:$nacosPort" -DefaultPort 8848 -Description "chat Nacos"
# api-gateway currently reads this endpoint from its checked-in local config
# before its runtime environment overrides apply, so verify that endpoint too.
$gatewayNacosEndpoint = Get-EndpointParts -Endpoint "127.0.0.1:8848" -DefaultPort 8848 -Description "Gateway Nacos"

Assert-TcpEndpoint -Endpoint $etcdEndpoint -Description "etcd"
Assert-TcpEndpoint -Endpoint $redisEndpoint -Description "Redis"
Assert-TcpEndpoint -Endpoint $kafkaEndpoint -Description "Kafka"
Assert-TcpEndpoint -Endpoint $postgresEndpoint -Description "PostgreSQL"
Assert-TcpEndpoint -Endpoint $chatNacosEndpoint -Description "chat Nacos"
if ($gatewayNacosEndpoint.Address -ne $chatNacosEndpoint.Address) {
  Assert-TcpEndpoint -Endpoint $gatewayNacosEndpoint -Description "Gateway Nacos"
}

$preflightFailures = New-Object 'System.Collections.Generic.List[string]'

try {
  $userServiceAddresses = @(Get-EtcdServiceAddresses -EtcdEndpoint $etcdEndpoint -ServiceName $UserServiceName)
  if ($userServiceAddresses.Count -eq 0) {
    [void]$preflightFailures.Add("$UserServiceName has no etcd registration. It is required for registration and chat user hydration.")
  } else {
    Write-Host "Prerequisite registered: $UserServiceName at $($userServiceAddresses -join ', ')."
  }
} catch {
  [void]$preflightFailures.Add($_.Exception.Message)
}

try {
  $existingChatAddresses = @(Get-EtcdServiceAddresses -EtcdEndpoint $etcdEndpoint -ServiceName $ChatServiceName)
  if ($existingChatAddresses.Count -gt 0) {
    [void]$preflightFailures.Add("$ChatServiceName is already registered in etcd at $($existingChatAddresses -join ', '). This script requires an isolated two-node chat topology and will not replace existing nodes.")
  }
} catch {
  [void]$preflightFailures.Add($_.Exception.Message)
}

$portChecks = @(
  [pscustomobject]@{ Port = $PrimaryChatPort; Purpose = "Primary chat-service" },
  [pscustomobject]@{ Port = $SecondaryChatPort; Purpose = "Secondary chat-service" },
  [pscustomobject]@{ Port = $PrimaryGatewayPort; Purpose = "Primary API Gateway" },
  [pscustomobject]@{ Port = $SecondaryGatewayPort; Purpose = "Secondary API Gateway" }
)
foreach ($portCheck in $portChecks) {
  try {
    Assert-PortIsFree -Port $portCheck.Port -Purpose $portCheck.Purpose
  } catch {
    [void]$preflightFailures.Add($_.Exception.Message)
  }
}

$managedProcessChecks = @(
  [pscustomobject]@{ ExecutablePath = $ChatExecutable; ServiceName = "chat-service" },
  [pscustomobject]@{ ExecutablePath = $GatewayExecutable; ServiceName = "api-gateway" }
)
foreach ($managedProcessCheck in $managedProcessChecks) {
  try {
    Assert-NoManagedProcess -ExecutablePath $managedProcessCheck.ExecutablePath -ServiceName $managedProcessCheck.ServiceName
  } catch {
    [void]$preflightFailures.Add($_.Exception.Message)
  }
}

if ($preflightFailures.Count -gt 0) {
  $details = @($preflightFailures | ForEach-Object { " - $_" }) -join [Environment]::NewLine
  throw "Chat cluster preflight failed:$([Environment]::NewLine)$details"
}

if (-not $Run) {
  Write-Host "Chat cluster preflight passed. No process, infrastructure, or product data was changed."
  return
}

if ($Build) {
  Invoke-ClusterBuild -ServiceDirectory $ChatServiceDir -OutputPath $ChatExecutable -ServiceName "chat-service"
  Invoke-ClusterBuild -ServiceDirectory $GatewayServiceDir -OutputPath $GatewayExecutable -ServiceName "api-gateway"
}
Assert-ExecutableExists -Path $ChatExecutable -ServiceName "chat-service"
Assert-ExecutableExists -Path $GatewayExecutable -ServiceName "api-gateway"

$stamp = Get-Date -Format "yyyyMMddHHmmssfff"
$logDirectory = Join-Path $RepoRoot "tmp-service-logs\chat-cluster-e2e\$stamp"
[void](New-Item -ItemType Directory -Path $logDirectory -Force)
$gatewayAUrl = "http://127.0.0.1:$PrimaryGatewayPort"
$gatewayBUrl = "http://127.0.0.1:$SecondaryGatewayPort"
$ownedServices = @()
$senderSocket = $null
$receiverSocket = $null
$runError = $null

$chatCommonEnvironment = @{
  BBS_CHAT_SERVICE_NAME = $ChatServiceName
  BBS_CHAT_APP_NAME = $ChatServiceName
  BBS_CHAT_GRPC_SERVER_SERVICE_NAME = $ChatServiceName
  BBS_CHAT_GRPC_SERVER_ETCD_ADDR = $etcdEndpoint.Address
  BBS_CHAT_GRPC_CLIENT_ETCD_ADDR = $etcdEndpoint.Address
  BBS_CHAT_KAFKA_TOPIC = $ChatKafkaTopic
  BBS_CHAT_KAFKA_REALTIME_GROUP_ID = $ChatRealtimeGroupID
  BBS_CHAT_KAFKA_PRODUCER_TOPIC = $ChatKafkaTopic
  BBS_CHAT_KAFKA_CONSUMER_TOPICS = $ChatKafkaTopic
  BBS_CHAT_KAFKA_CONSUMER_GROUP_ID = $ChatRealtimeGroupID
}

try {
  $primaryChatEnvironment = @{}
  foreach ($entry in $chatCommonEnvironment.GetEnumerator()) { $primaryChatEnvironment[$entry.Key] = $entry.Value }
  $primaryChatEnvironment["BBS_CHAT_SERVICE_GRPC_PORT"] = [string]$PrimaryChatPort
  $primaryChatEnvironment["BBS_CHAT_GRPC_SERVER_PORT"] = [string]$PrimaryChatPort
  $primaryChatEnvironment["BBS_CHAT_SNOWFLAKE_WORKER_ID"] = [string]$PrimaryChatWorkerID
  $primaryChatEnvironment["BBS_CHAT_LOG_FILENAME"] = (Join-Path $logDirectory "chat-primary.log")
  $primaryChatOutput = Join-Path $logDirectory "chat-primary.stdout.log"
  $primaryChatError = Join-Path $logDirectory "chat-primary.stderr.log"
  $primaryChatProcess = Start-OwnedService -Name "chat-service primary (worker $PrimaryChatWorkerID)" -ExecutablePath $ChatExecutable `
    -WorkingDirectory $ChatServiceDir -Arguments @("server", "-c", "configs/config.yaml") -Environment $primaryChatEnvironment `
    -StandardOutputPath $primaryChatOutput -StandardErrorPath $primaryChatError
  $ownedServices += [pscustomobject]@{ Name = "chat-service primary"; Process = $primaryChatProcess; ExecutablePath = $ChatExecutable; StandardOutputPath = $primaryChatOutput; StandardErrorPath = $primaryChatError }

  $secondaryChatEnvironment = @{}
  foreach ($entry in $chatCommonEnvironment.GetEnumerator()) { $secondaryChatEnvironment[$entry.Key] = $entry.Value }
  $secondaryChatEnvironment["BBS_CHAT_SERVICE_GRPC_PORT"] = [string]$SecondaryChatPort
  $secondaryChatEnvironment["BBS_CHAT_GRPC_SERVER_PORT"] = [string]$SecondaryChatPort
  $secondaryChatEnvironment["BBS_CHAT_SNOWFLAKE_WORKER_ID"] = [string]$SecondaryChatWorkerID
  $secondaryChatEnvironment["BBS_CHAT_LOG_FILENAME"] = (Join-Path $logDirectory "chat-secondary.log")
  $secondaryChatOutput = Join-Path $logDirectory "chat-secondary.stdout.log"
  $secondaryChatError = Join-Path $logDirectory "chat-secondary.stderr.log"
  $secondaryChatProcess = Start-OwnedService -Name "chat-service secondary (worker $SecondaryChatWorkerID)" -ExecutablePath $ChatExecutable `
    -WorkingDirectory $ChatServiceDir -Arguments @("server", "-c", "configs/config.yaml") -Environment $secondaryChatEnvironment `
    -StandardOutputPath $secondaryChatOutput -StandardErrorPath $secondaryChatError
  $ownedServices += [pscustomobject]@{ Name = "chat-service secondary"; Process = $secondaryChatProcess; ExecutablePath = $ChatExecutable; StandardOutputPath = $secondaryChatOutput; StandardErrorPath = $secondaryChatError }

  Wait-ProcessPort -Process $primaryChatProcess -Port $PrimaryChatPort -Name "chat-service primary" -StandardOutputPath $primaryChatOutput -StandardErrorPath $primaryChatError -TimeoutSeconds $WaitSeconds
  Wait-ProcessPort -Process $secondaryChatProcess -Port $SecondaryChatPort -Name "chat-service secondary" -StandardOutputPath $secondaryChatOutput -StandardErrorPath $secondaryChatError -TimeoutSeconds $WaitSeconds
  $chatEndpoints = @(Wait-ChatClusterRegistration -EtcdEndpoint $etcdEndpoint -TimeoutSeconds $WaitSeconds)
  Write-Host "Verified etcd chat cluster endpoints: $($chatEndpoints -join ', ')."

  $gatewayAOutput = Join-Path $logDirectory "gateway-primary.stdout.log"
  $gatewayAError = Join-Path $logDirectory "gateway-primary.stderr.log"
  # The scenario creates timestamped accounts. Keep its isolated Gateways from
  # consuming the normal local registration window across repeated test runs.
  $gatewayAEnvironment = @{
    BBS_GATEWAY_SERVICE_HTTP_PORT = [string]$PrimaryGatewayPort
    BBS_GATEWAY_GRPC_CLIENT_ETCD_ADDR = $etcdEndpoint.Address
    BBS_GATEWAY_UPSTREAMS_CHAT = $ChatServiceName
    BBS_GATEWAY_AUTH_RATE_LIMIT_REGISTER_RATE = "1000"
    BBS_GATEWAY_LOG_FILENAME = (Join-Path $logDirectory "gateway-primary.log")
    BBS_GATEWAY_TRACE_SERVICE_NAME = "bbs-api-gateway-chat-cluster-primary"
  }
  $gatewayAProcess = Start-OwnedService -Name "API Gateway primary" -ExecutablePath $GatewayExecutable -WorkingDirectory $GatewayServiceDir `
    -Arguments @("server", "-c", "configs/config.local.yaml") -Environment $gatewayAEnvironment -StandardOutputPath $gatewayAOutput -StandardErrorPath $gatewayAError
  $ownedServices += [pscustomobject]@{ Name = "API Gateway primary"; Process = $gatewayAProcess; ExecutablePath = $GatewayExecutable; StandardOutputPath = $gatewayAOutput; StandardErrorPath = $gatewayAError }

  $gatewayBOutput = Join-Path $logDirectory "gateway-secondary.stdout.log"
  $gatewayBError = Join-Path $logDirectory "gateway-secondary.stderr.log"
  $gatewayBEnvironment = @{
    BBS_GATEWAY_SERVICE_HTTP_PORT = [string]$SecondaryGatewayPort
    BBS_GATEWAY_GRPC_CLIENT_ETCD_ADDR = $etcdEndpoint.Address
    BBS_GATEWAY_UPSTREAMS_CHAT = $ChatServiceName
    BBS_GATEWAY_AUTH_RATE_LIMIT_REGISTER_RATE = "1000"
    BBS_GATEWAY_LOG_FILENAME = (Join-Path $logDirectory "gateway-secondary.log")
    BBS_GATEWAY_TRACE_SERVICE_NAME = "bbs-api-gateway-chat-cluster-secondary"
  }
  $gatewayBProcess = Start-OwnedService -Name "API Gateway secondary" -ExecutablePath $GatewayExecutable -WorkingDirectory $GatewayServiceDir `
    -Arguments @("server", "-c", "configs/config.local.yaml") -Environment $gatewayBEnvironment -StandardOutputPath $gatewayBOutput -StandardErrorPath $gatewayBError
  $ownedServices += [pscustomobject]@{ Name = "API Gateway secondary"; Process = $gatewayBProcess; ExecutablePath = $GatewayExecutable; StandardOutputPath = $gatewayBOutput; StandardErrorPath = $gatewayBError }

  Wait-ProcessPort -Process $gatewayAProcess -Port $PrimaryGatewayPort -Name "API Gateway primary" -StandardOutputPath $gatewayAOutput -StandardErrorPath $gatewayAError -TimeoutSeconds $WaitSeconds
  Wait-ProcessPort -Process $gatewayBProcess -Port $SecondaryGatewayPort -Name "API Gateway secondary" -StandardOutputPath $gatewayBOutput -StandardErrorPath $gatewayBError -TimeoutSeconds $WaitSeconds
  Wait-GatewayReady -Process $gatewayAProcess -BaseUrl $gatewayAUrl -Name "API Gateway primary" -StandardOutputPath $gatewayAOutput -StandardErrorPath $gatewayAError -TimeoutSeconds $WaitSeconds
  Wait-GatewayReady -Process $gatewayBProcess -BaseUrl $gatewayBUrl -Name "API Gateway secondary" -StandardOutputPath $gatewayBOutput -StandardErrorPath $gatewayBError -TimeoutSeconds $WaitSeconds

  $sender = Register-ClusterUser -BaseUrl $gatewayAUrl -Prefix "chatclustera" -Stamp $stamp
  $receiver = Register-ClusterUser -BaseUrl $gatewayBUrl -Prefix "chatclusterb" -Stamp $stamp
  $room = Invoke-BbsApi -Uri "$gatewayAUrl/api/v1/chat/rooms" -Method Post -Headers $sender.Headers -Body (@{
    name = "Chat Cluster $stamp"
  } | ConvertTo-Json -Compress)
  $roomNo = [string]$room.details.room.room_no
  if ([string]::IsNullOrWhiteSpace($roomNo)) {
    throw "Chat room creation through $gatewayAUrl did not return room_no"
  }
  $joined = Invoke-BbsApi -Uri "$gatewayBUrl/api/v1/chat/rooms/$roomNo/join" -Method Post -Headers $receiver.Headers
  if ([string]$joined.details.membership.user_id -ne $receiver.ID) {
    throw "Receiver join through $gatewayBUrl did not return its membership"
  }

  # Tickets are deliberately issued by the opposite Gateway from the one that
  # upgrades each socket, proving shared Redis ticket consumption across nodes.
  $senderTicket = Invoke-BbsApi -Uri "$gatewayBUrl/api/v1/chat/ws-tickets" -Method Post -Headers $sender.Headers
  $receiverTicket = Invoke-BbsApi -Uri "$gatewayAUrl/api/v1/chat/ws-tickets" -Method Post -Headers $receiver.Headers
  if ([string]::IsNullOrWhiteSpace([string]$senderTicket.ticket) -or [string]::IsNullOrWhiteSpace([string]$receiverTicket.ticket)) {
    throw "Cross-Gateway WebSocket ticket issuance did not return both tickets"
  }
  $senderSocket = Open-ChatWebSocket -BaseUrl $gatewayAUrl -Ticket ([string]$senderTicket.ticket)
  $receiverSocket = Open-ChatWebSocket -BaseUrl $gatewayBUrl -Ticket ([string]$receiverTicket.ticket)
  [void](Receive-WebSocketEventUntil -Socket $senderSocket -ExpectedType "session.ready" -TimeoutSeconds $EventWaitSeconds)
  [void](Receive-WebSocketEventUntil -Socket $receiverSocket -ExpectedType "session.ready" -TimeoutSeconds $EventWaitSeconds)

  $senderSubscriptionRequestID = "sender-sub-$stamp"
  $receiverSubscriptionRequestID = "receiver-sub-$stamp"
  Send-WebSocketJson -Socket $senderSocket -Envelope @{
    type = "room.subscribe"
    request_id = $senderSubscriptionRequestID
    payload = @{ room_numbers = @($roomNo) }
  }
  Send-WebSocketJson -Socket $receiverSocket -Envelope @{
    type = "room.subscribe"
    request_id = $receiverSubscriptionRequestID
    payload = @{ room_numbers = @($roomNo) }
  }
  [void](Receive-WebSocketEventUntil -Socket $senderSocket -ExpectedType "room.subscribed" -RequestID $senderSubscriptionRequestID -TimeoutSeconds $EventWaitSeconds)
  [void](Receive-WebSocketEventUntil -Socket $receiverSocket -ExpectedType "room.subscribed" -RequestID $receiverSubscriptionRequestID -TimeoutSeconds $EventWaitSeconds)

  $clientMessageID = [guid]::NewGuid().ToString()
  $messageBody = "chat cluster websocket $stamp"
  $sendRequestID = "send-$stamp"
  Send-WebSocketJson -Socket $senderSocket -Envelope @{
    type = "message.send"
    request_id = $sendRequestID
    payload = @{
      room_no = $roomNo
      client_message_id = $clientMessageID
      body = $messageBody
    }
  }
  $ack = Receive-WebSocketEventUntil -Socket $senderSocket -ExpectedType "message.ack" -RequestID $sendRequestID -TimeoutSeconds $EventWaitSeconds
  if ([string]$ack.payload.message.client_message_id -ne $clientMessageID -or [string]$ack.payload.message.body -ne $messageBody -or [int64]$ack.payload.message.seq -le 0) {
    throw "Sender Gateway did not return the expected message.ack: $($ack._raw)"
  }
  $messageID = [string]$ack.payload.message.id
  if ([string]::IsNullOrWhiteSpace($messageID)) {
    throw "Sender Gateway did not return a message id: $($ack._raw)"
  }
  $messageSequence = [int64]$ack.payload.message.seq

  $created = Receive-WebSocketEventUntil -Socket $receiverSocket -ExpectedType "message.created" -TimeoutSeconds $EventWaitSeconds
  if ([string]$created.payload.payload.client_message_id -ne $clientMessageID -or [string]$created.payload.payload.body -ne $messageBody -or [string]$created.payload.payload.room_no -ne $roomNo) {
    throw "Receiver Gateway did not receive the expected cross-node message.created event: $($created._raw)"
  }

  # Retry the same client message through the other Gateway's HTTP fallback.
  # The persisted message must be returned rather than allocating a new room
  # sequence or emitting a second durable event.
  $idempotentRetry = Invoke-BbsApi -Uri "$gatewayBUrl/api/v1/chat/rooms/$roomNo/messages" -Method Post -Headers $sender.Headers -Body (@{
    client_message_id = $clientMessageID
    body = $messageBody
  } | ConvertTo-Json -Compress)
  if ([string]$idempotentRetry.message.client_message_id -ne $clientMessageID -or [string]$idempotentRetry.message.body -ne $messageBody -or [int64]$idempotentRetry.message.seq -ne $messageSequence) {
    throw "Cross-Gateway idempotent message retry did not return the original message sequence $messageSequence"
  }

  $history = Invoke-BbsApi -Uri "$gatewayBUrl/api/v1/chat/rooms/$roomNo/messages?after_seq=0&limit=100" -Headers $receiver.Headers
  $persisted = @($history.messages | Where-Object { [string]$_.client_message_id -eq $clientMessageID })
  if ($persisted.Count -ne 1 -or [string]$persisted[0].body -ne $messageBody -or [int64]$persisted[0].seq -ne $messageSequence) {
    throw "Chat history through $gatewayBUrl did not contain exactly one persisted message for client_message_id $clientMessageID"
  }

  $deleted = Invoke-BbsApi -Uri "$gatewayBUrl/api/v1/chat/rooms/$roomNo/messages/$messageID" -Method Delete -Headers $sender.Headers
  if ([string]$deleted.message.id -ne $messageID -or [int]$deleted.message.status -ne 2 -or -not [string]::IsNullOrEmpty([string]$deleted.message.body)) {
    throw "Cross-Gateway message deletion did not return the expected tombstone"
  }
  $deletedEvent = Receive-WebSocketEventUntil -Socket $receiverSocket -ExpectedType "message.deleted" -TimeoutSeconds $EventWaitSeconds
  if ([string]$deletedEvent.payload.payload.message_id -ne $messageID -or [string]$deletedEvent.payload.payload.room_no -ne $roomNo -or [int64]$deletedEvent.payload.payload.seq -ne $messageSequence -or [int]$deletedEvent.payload.payload.status -ne 2) {
    throw "Receiver Gateway did not receive the expected cross-node message.deleted event: $($deletedEvent._raw)"
  }
  $deletedHistory = Invoke-BbsApi -Uri "$gatewayBUrl/api/v1/chat/rooms/$roomNo/messages?after_seq=0&limit=100" -Headers $receiver.Headers
  $deletedPersisted = @($deletedHistory.messages | Where-Object { [string]$_.id -eq $messageID })
  if ($deletedPersisted.Count -ne 1 -or [int]$deletedPersisted[0].status -ne 2 -or -not [string]::IsNullOrEmpty([string]$deletedPersisted[0].body) -or [int64]$deletedPersisted[0].seq -ne $messageSequence) {
    throw "Chat history did not retain the deleted message tombstone for $messageID"
  }

  $announcementText = "chat announcement $stamp"
  $announcement = Invoke-BbsApi -Uri "$gatewayAUrl/api/v1/chat/rooms/$roomNo/announcement" -Method Patch -Headers $sender.Headers -Body (@{
    announcement = $announcementText
  } | ConvertTo-Json -Compress)
  $announcementVersion = [int64]$announcement.room.announcement_version
  if ([string]$announcement.room.announcement -ne $announcementText -or $announcementVersion -le 0) {
    throw "Owner announcement update through $gatewayAUrl did not return the saved room state"
  }
  $announcementEvent = Receive-WebSocketEventUntil -Socket $receiverSocket -ExpectedType "announcement.updated" -TimeoutSeconds $EventWaitSeconds
  if ([string]$announcementEvent.payload.payload.room_no -ne $roomNo -or [string]$announcementEvent.payload.payload.announcement -ne $announcementText -or [int64]$announcementEvent.payload.payload.announcement_version -ne $announcementVersion) {
    throw "Receiver Gateway did not receive the expected cross-node announcement.updated event: $($announcementEvent._raw)"
  }
  $announcementSeen = Invoke-BbsApi -Uri "$gatewayBUrl/api/v1/chat/rooms/$roomNo/announcement-seen" -Method Put -Headers $receiver.Headers -Body (@{
    announcement_version = $announcementVersion
  } | ConvertTo-Json -Compress)
  if ([int64]$announcementSeen.membership.last_seen_announcement_version -ne $announcementVersion) {
    throw "Receiver announcement seen state did not advance to version $announcementVersion"
  }

  $receiverSidebarBeforeRead = Get-ChatSidebarRoom -BaseUrl $gatewayBUrl -Headers $receiver.Headers -RoomNo $roomNo
  $unreadBeforeRead = [int64]$receiverSidebarBeforeRead.unread_count
  if ($unreadBeforeRead -lt 1) {
    throw "Receiver unread count before mark-read was $unreadBeforeRead, expected at least one"
  }
  $read = Invoke-BbsApi -Uri "$gatewayBUrl/api/v1/chat/rooms/$roomNo/read" -Method Put -Headers $receiver.Headers -Body (@{
    read_seq = $messageSequence
  } | ConvertTo-Json -Compress)
  if ([int64]$read.unread_count -ne 0 -or [int64]$read.membership.last_read_seq -ne $messageSequence) {
    throw "Receiver mark-read did not advance to sequence $messageSequence"
  }
  $receiverSidebarAfterRead = Get-ChatSidebarRoom -BaseUrl $gatewayBUrl -Headers $receiver.Headers -RoomNo $roomNo
  if ([int64]$receiverSidebarAfterRead.unread_count -ne 0) {
    throw "Receiver sidebar unread count remained $($receiverSidebarAfterRead.unread_count) after mark-read"
  }

  [pscustomobject]@{
    chat_service_name = $ChatServiceName
    chat_endpoints = $chatEndpoints
    kafka_topic = $ChatKafkaTopic
    kafka_realtime_group = $ChatRealtimeGroupID
    chat_workers = @($PrimaryChatWorkerID, $SecondaryChatWorkerID)
    gateways = @($gatewayAUrl, $gatewayBUrl)
    sender_user_id = $sender.ID
    receiver_user_id = $receiver.ID
    room_no = $roomNo
    message_client_id = $clientMessageID
    message_id = $messageID
    message_seq = $messageSequence
    history_matches = $persisted.Count
    deleted_history_matches = $deletedPersisted.Count
    cross_gateway_message_deleted = $true
    idempotent_retry_gateway = $gatewayBUrl
    idempotent_retry_seq = [int64]$idempotentRetry.message.seq
    idempotent_retry_matches = $true
    announcement_version = $announcementVersion
    announcement_cross_gateway = $true
    announcement_seen_version = [int64]$announcementSeen.membership.last_seen_announcement_version
    unread_before_read = $unreadBeforeRead
    unread_after_read = [int64]$receiverSidebarAfterRead.unread_count
    cross_gateway_ticket_consumed = $true
    cross_gateway_message_created = $true
    logs = $logDirectory
  } | ConvertTo-Json -Depth 4
} catch {
	$runError = $_
  Write-OwnedServiceDiagnostics -OwnedServices $ownedServices
  throw
} finally {
	$cleanupFailures = New-Object 'System.Collections.Generic.List[string]'
  if ($KeepRunning) {
    if ($ownedServices.Count -gt 0) {
      Write-Host "-KeepRunning was supplied; owned service processes remain running. Logs: $logDirectory"
    }
  } else {
    $ownedServicesToStop = @($ownedServices)
    [Array]::Reverse($ownedServicesToStop)
    foreach ($owned in $ownedServicesToStop) {
      if (-not (Stop-OwnedService $owned)) {
        [void]$cleanupFailures.Add("could not stop owned $($owned.Name) process")
      }
    }
    $ownedChatServices = @($ownedServices | Where-Object { [string]$_.Name -like "chat-service *" })
    if ($ownedChatServices.Count -gt 0 -and -not (Wait-ChatClusterDeregistration -EtcdEndpoint $etcdEndpoint)) {
      [void]$cleanupFailures.Add("chat-service registrations did not clear from etcd")
    }
  }
  Close-ChatWebSocket $senderSocket
  Close-ChatWebSocket $receiverSocket
  if ($cleanupFailures.Count -gt 0) {
    $message = "Chat cluster cleanup failed: $($cleanupFailures -join '; '). Logs: $logDirectory"
    if ($null -eq $runError) {
      throw $message
    }
    Write-Warning $message
  }
}
} finally {
  Restore-ProcessEnvironment $environmentSnapshot
}
