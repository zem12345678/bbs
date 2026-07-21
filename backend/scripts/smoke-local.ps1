param(
  [int]$GatewayPort = 18080,
  [int]$MallPort = 0,
  [int]$SearchPort = 0,
  [switch]$SkipBuild,
  [switch]$KeepRunning,
  [switch]$RefreshRunningServices,
  [int]$ProjectionRetries = 60
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$ServicesRoot = Join-Path $RepoRoot "backend\services"
$Services = @(
  @{ Name = "user-service"; Port = 9102 },
  @{ Name = "content-service"; Port = 9103 },
  @{ Name = "comment-service"; Port = 9104 },
  @{ Name = "reaction-service"; Port = 9105 },
  @{ Name = "admin-service"; Port = 9114 },
  @{ Name = "search-service"; Port = 9106 },
  @{ Name = "credit-service"; Port = 9107 },
  @{ Name = "notification-service"; Port = 9108 },
  @{ Name = "feed-service"; Port = 9113 },
  @{ Name = "mall-service"; Port = 9115 },
  @{ Name = "file-service"; Port = 9111 }
)

$Started = New-Object System.Collections.Generic.List[System.Diagnostics.Process]

if ($ProjectionRetries -lt 1) {
  throw "ProjectionRetries must be greater than 0"
}
if ($MallPort -lt 0) {
  throw "MallPort must be greater than or equal to 0"
}
if ($SearchPort -lt 0) {
  throw "SearchPort must be greater than or equal to 0"
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

$MallPort = Resolve-MallPortOverride $MallPort
if ($MallPort -gt 0) {
  foreach ($service in $Services) {
    if ($service.Name -eq "mall-service") {
      $service.Port = $MallPort
      break
    }
  }
}

$SearchPort = Resolve-SearchPortOverride $SearchPort
if ($SearchPort -gt 0) {
  foreach ($service in $Services) {
    if ($service.Name -eq "search-service") {
      $service.Port = $SearchPort
      break
    }
  }
}

function Get-LocalProbeHosts {
  $hosts = New-Object System.Collections.Generic.List[string]
  $hosts.Add("127.0.0.1")
  try {
    $interfaces = [System.Net.NetworkInformation.NetworkInterface]::GetAllNetworkInterfaces()
    foreach ($interface in $interfaces) {
      foreach ($addressInfo in $interface.GetIPProperties().UnicastAddresses) {
        if ($addressInfo.Address.AddressFamily -eq [System.Net.Sockets.AddressFamily]::InterNetwork) {
          $address = $addressInfo.Address.ToString()
          if ($address -ne "0.0.0.0" -and -not $hosts.Contains($address)) {
            $hosts.Add($address)
          }
        }
      }
    }
  } catch {
  }
  return @($hosts)
}

function Test-TcpEndpoint {
  param(
    [string]$HostName,
    [int]$Port,
    [int]$TimeoutMilliseconds = 300
  )

  $client = New-Object System.Net.Sockets.TcpClient
  try {
    $async = $client.BeginConnect($HostName, $Port, $null, $null)
    if ($async.AsyncWaitHandle.WaitOne($TimeoutMilliseconds)) {
      $client.EndConnect($async)
      return $true
    }
    return $false
  } catch {
    return $false
  } finally {
    $client.Close()
  }
}

function Test-PortListening {
  param([int]$Port)

  foreach ($probeHost in Get-LocalProbeHosts) {
    if (Test-TcpEndpoint $probeHost $Port 300) {
      return $true
    }
  }
  return $false
}

function Get-ExpectedServiceExecutablePath {
  param([string]$ServiceName)

  return [System.IO.Path]::GetFullPath((Join-Path (Join-Path $ServicesRoot $ServiceName) "bin\$ServiceName.exe"))
}

function Get-ExpectedServiceProcessIds {
  param([string]$ServiceName)

  $expectedPath = Get-ExpectedServiceExecutablePath $ServiceName
  return @(
    Get-CimInstance Win32_Process -Filter "name='$ServiceName.exe'" -ErrorAction SilentlyContinue |
      Where-Object {
        -not [string]::IsNullOrWhiteSpace([string]$_.ExecutablePath) -and
        [System.StringComparer]::OrdinalIgnoreCase.Equals(
          [System.IO.Path]::GetFullPath([string]$_.ExecutablePath),
          $expectedPath
        )
      } |
      ForEach-Object { [int]$_.ProcessId } |
      Sort-Object -Unique
  )
}

function Get-ListeningProcessIds {
  param([int]$Port)

  return @(
    Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
      ForEach-Object { [int]$_.OwningProcess } |
      Sort-Object -Unique
  )
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

function Assert-PortReusableOrFree {
  param(
    [string]$ServiceName,
    [int]$Port
  )

  $listeningProcessIds = @(Get-ListeningProcessIds $Port)
  if ($listeningProcessIds.Count -eq 0) {
    return $false
  }

  $expectedProcessIds = @(Get-ExpectedServiceProcessIds $ServiceName)
  $expectedListeningProcessIds = @($listeningProcessIds | Where-Object { $expectedProcessIds -contains $_ })
  if ($expectedListeningProcessIds.Count -gt 0) {
    return $true
  }
  $unexpectedProcessIds = @($listeningProcessIds | Where-Object { $expectedProcessIds -notcontains $_ })

  $details = @($unexpectedProcessIds | ForEach-Object { Get-ProcessSummary $_ }) -join "; "
  if ([string]::IsNullOrWhiteSpace($details)) {
    $details = @($listeningProcessIds | ForEach-Object { Get-ProcessSummary $_ }) -join "; "
  }
  throw "$ServiceName cannot use port $Port because it is already held by an unexpected process: $details"
}

function Wait-PortReleased {
  param(
    [string]$ServiceName,
    [int]$Port,
    [int]$TimeoutSeconds = 10
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    if (@(Get-ListeningProcessIds $Port).Count -eq 0) {
      return
    }
    Start-Sleep -Milliseconds 250
  }

  $details = @(Get-ListeningProcessIds $Port | ForEach-Object { Get-ProcessSummary $_ }) -join "; "
  throw "Timed out waiting for $ServiceName to release port ${Port}: $details"
}

function Stop-ExpectedServiceProcesses {
  param([string]$ServiceName)

  foreach ($processId in @(Get-ExpectedServiceProcessIds $ServiceName)) {
    Stop-Process -Id $processId -Force -ErrorAction Stop
    Write-Host "Stopped $ServiceName process $processId for a fresh smoke run."
  }
}

function Get-ServiceDiscoveryRegistrations {
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

  $items = @($response.kvs | Where-Object { $null -ne $_ })
  foreach ($item in $items) {
    try {
      $key = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String([string]$item.key))
      $value = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String([string]$item.value)) | ConvertFrom-Json
    } catch {
      throw "Invalid etcd registration for ${discoveryName}: $($_.Exception.Message)"
    }
    [pscustomobject]@{
      Key = $key
      Address = [string]$value.addr
    }
  }
}

function Wait-ServiceDiscoveryRegistrationsAbsent {
  param(
    [object[]]$ServiceSpecs,
    [int]$TimeoutSeconds = 20
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  $remaining = @()
  while ((Get-Date) -lt $deadline) {
    $remaining = @(
      foreach ($service in $ServiceSpecs) {
        $registrations = @(Get-ServiceDiscoveryRegistrations $service.Name)
        if ($registrations.Count -gt 0) {
          "$($service.Name): $((@($registrations | ForEach-Object { $_.Address } | Sort-Object -Unique) -join ', '))"
        }
      }
    )
    if ($remaining.Count -eq 0) {
      return
    }
    Start-Sleep -Seconds 1
  }

  throw "Timed out waiting for prior etcd registrations to expire: $($remaining -join '; ')"
}

function Wait-ForSingleServiceDiscoveryRegistration {
  param(
    [string]$ServiceName,
    [int]$Port,
    [int]$TimeoutSeconds = 30
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  $addresses = @()
  while ((Get-Date) -lt $deadline) {
    $addresses = @(
      Get-ServiceDiscoveryRegistrations $ServiceName |
        ForEach-Object { [string]$_.Address } |
        Sort-Object -Unique
    )
    if ($addresses.Count -eq 1 -and $addresses[0].EndsWith(":$Port", [System.StringComparison]::Ordinal)) {
      return
    }
    Start-Sleep -Seconds 1
  }

  $found = if ($addresses.Count -gt 0) { $addresses -join ', ' } else { "none" }
  throw "Expected exactly one $ServiceName etcd registration on port $Port, found: $found"
}

function Refresh-RunningServiceProcesses {
  foreach ($service in $Services) {
    [void](Assert-PortReusableOrFree $service.Name $service.Port)
  }
  [void](Assert-PortReusableOrFree "api-gateway" $GatewayPort)

  foreach ($service in $Services) {
    Stop-ExpectedServiceProcesses $service.Name
  }
  Stop-ExpectedServiceProcesses "api-gateway"

  foreach ($service in $Services) {
    Wait-PortReleased $service.Name $service.Port
  }
  Wait-PortReleased "api-gateway" $GatewayPort
  Wait-ServiceDiscoveryRegistrationsAbsent $Services
}

function Invoke-GoBuild {
  param([string]$ServiceName)

  $serviceDir = Join-Path $ServicesRoot $ServiceName
  $buildTarget = ".\cmd"
  Push-Location $serviceDir
  try {
    go build -o "bin\$ServiceName.exe" $buildTarget
  } finally {
    Pop-Location
  }
}

function Wait-Port {
  param(
    [int]$Port,
    [int]$TimeoutSeconds = 45
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    foreach ($probeHost in Get-LocalProbeHosts) {
      if (Test-TcpEndpoint $probeHost $Port 500) {
        return
      }
    }
    Start-Sleep -Milliseconds 300
  }
  throw "Timed out waiting for TCP port $Port"
}

function Wait-Http {
  param(
    [string]$Url,
    [int]$TimeoutSeconds = 45
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    try {
      Invoke-RestMethod -Uri $Url -Method Get -TimeoutSec 2 | Out-Null
      return
    } catch {
      Start-Sleep -Milliseconds 500
    }
  }
  throw "Timed out waiting for $Url"
}

function Invoke-Api {
  $result = Microsoft.PowerShell.Utility\Invoke-RestMethod @args
  if ($null -ne $result) {
    $propertyNames = @($result.PSObject.Properties | ForEach-Object { $_.Name })
    if ($propertyNames -contains "http_code" -and $propertyNames -contains "code" -and $propertyNames -contains "data") {
      if ([int64]$result.code -ne 0) {
        throw "API error $($result.code): $($result.message)"
      }
      return $result.data
    }
  }
  return $result
}

function Get-MailpitActionToken {
  param(
    [string]$MessageId,
    [string]$ActionPath
  )

  $detail = Invoke-RestMethod -Uri "http://127.0.0.1:8025/api/v1/message/$MessageId" -TimeoutSec 5
  $parts = @($detail.Text, $detail.HTML, $detail.Raw) | Where-Object {
    -not [string]::IsNullOrWhiteSpace([string]$_)
  }
  $body = [string]::Join("`n", [string[]]@($parts))
  $pattern = 'https?://[^[:space:]]+' + [regex]::Escape($ActionPath) + '\?token=(?<token>[0-9a-f]{64})'
  $match = [regex]::Match($body, $pattern)
  if (-not $match.Success) {
    return $null
  }
  return $match.Groups['token'].Value
}

function Wait-MailpitActionMessage {
  param(
    [string]$Recipient,
    [string]$ActionPath,
    [int]$TimeoutSeconds = 15
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    $messages = @((Invoke-RestMethod -Uri "http://127.0.0.1:8025/api/v1/messages" -TimeoutSec 5).messages)
    foreach ($message in $messages) {
      $recipients = @($message.To | Where-Object {
        [string]::Equals([string]$_.Address, $Recipient, [System.StringComparison]::OrdinalIgnoreCase)
      })
      if ($recipients.Count -eq 0) {
        continue
      }
      $token = Get-MailpitActionToken -MessageId ([string]$message.ID) -ActionPath $ActionPath
      if (-not [string]::IsNullOrWhiteSpace($token)) {
        return [pscustomobject]@{
          MessageId = [string]$message.ID
          Token = $token
        }
      }
    }
    Start-Sleep -Milliseconds 500
  }

  throw "Mailpit did not receive a valid $ActionPath action email for $Recipient"
}

function Assert-ApiForbidden {
  try {
    Microsoft.PowerShell.Utility\Invoke-RestMethod @args | Out-Null
  } catch {
    $statusCode = Get-ApiStatusCode $_
    if ($statusCode -eq 403) {
      return
    }
    throw
  }
  throw "Expected API request to be forbidden"
}

function Assert-ApiStatus {
  param(
    [int]$ExpectedStatus
  )
  try {
    Microsoft.PowerShell.Utility\Invoke-RestMethod @args | Out-Null
  } catch {
    $statusCode = Get-ApiStatusCode $_
    if ($statusCode -eq $ExpectedStatus) {
      return
    }
    throw "Expected API request to return HTTP $ExpectedStatus, got HTTP $statusCode`: $(Get-ApiErrorBody $_)"
  }
  throw "Expected API request to return HTTP $ExpectedStatus"
}

function Get-ApiErrorBody {
  param([object]$ErrorRecord)

  if ($ErrorRecord.ErrorDetails -and -not [string]::IsNullOrWhiteSpace([string]$ErrorRecord.ErrorDetails.Message)) {
    return [string]$ErrorRecord.ErrorDetails.Message
  }

  $response = $ErrorRecord.Exception.Response
  if ($response -and $response.GetResponseStream) {
    try {
      $reader = New-Object System.IO.StreamReader($response.GetResponseStream())
      try {
        return $reader.ReadToEnd()
      } finally {
        $reader.Dispose()
      }
    } catch {
    }
  }

  return [string]$ErrorRecord.Exception.Message
}

function Get-ApiStatusCode {
  param([object]$ErrorRecord)

  $response = $ErrorRecord.Exception.Response
  if ($response -and $null -ne $response.StatusCode) {
    return [int]$response.StatusCode
  }
  if ($ErrorRecord.ErrorDetails -and -not [string]::IsNullOrWhiteSpace([string]$ErrorRecord.ErrorDetails.Message)) {
    try {
      $payload = $ErrorRecord.ErrorDetails.Message | ConvertFrom-Json
      if ($null -ne $payload.http_code) {
        return [int]$payload.http_code
      }
    } catch {
    }
  }
  return $null
}

function Assert-ApiStatusMessage {
  param(
    [int]$ExpectedStatus,
    [string]$ExpectedMessage
  )
  try {
    Microsoft.PowerShell.Utility\Invoke-RestMethod @args | Out-Null
  } catch {
    $statusCode = Get-ApiStatusCode $_
    $body = Get-ApiErrorBody $_
    if ($statusCode -eq $ExpectedStatus -and $body -like "*$ExpectedMessage*") {
      return
    }
    throw "Expected API request to return HTTP $ExpectedStatus containing '$ExpectedMessage', got HTTP $statusCode`: $body"
  }
  throw "Expected API request to return HTTP $ExpectedStatus containing '$ExpectedMessage'"
}

function Assert-ObjectProperty {
  param(
    [object]$Object,
    [string]$Property,
    [string]$Message
  )

  if ($null -eq $Object) {
    throw $Message
  }
  $propertyNames = @($Object.PSObject.Properties | ForEach-Object { $_.Name })
  if ($propertyNames -notcontains $Property) {
    throw $Message
  }
}

function Expand-TreeNodes {
  param([object[]]$Nodes)

  foreach ($node in @($Nodes)) {
    if ($null -eq $node) {
      continue
    }
    $node
    $propertyNames = @($node.PSObject.Properties | ForEach-Object { $_.Name })
    if ($propertyNames -contains "children" -and $node.children) {
      Expand-TreeNodes -Nodes @($node.children)
    }
  }
}

function Invoke-ServiceMigrate {
  param([string]$ServiceName)

  $serviceDir = Join-Path $ServicesRoot $ServiceName
  Push-Location $serviceDir
  try {
    & (Join-Path $serviceDir "bin\$ServiceName.exe") "migrate" "-c" "configs/config.yaml"
    if ($LASTEXITCODE -ne 0) {
      throw "$ServiceName migrate failed with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }
}

function Invoke-ReactionCacheRebuild {
  $serviceDir = Join-Path $ServicesRoot "reaction-service"
  Push-Location $serviceDir
  try {
    $output = & (Join-Path $serviceDir "bin\reaction-service.exe") "rebuild-cache" "-c" "configs/config.yaml" "--verify"
    if ($LASTEXITCODE -ne 0) {
      throw "reaction-service rebuild-cache failed with exit code $LASTEXITCODE"
    }
    $jsonOutput = @($output | Where-Object { [string]$_ -match "^\s*\{" } | Select-Object -Last 1)
    if ([string]::IsNullOrWhiteSpace([string]$jsonOutput)) {
      throw "reaction-service rebuild-cache did not emit JSON output"
    }
    return ($jsonOutput | ConvertFrom-Json)
  } finally {
    Pop-Location
  }
}

function Start-ServiceProcess {
  param(
    [string]$ServiceName,
    [int]$Port
  )

  if ($Port -gt 0 -and (Assert-PortReusableOrFree $ServiceName $Port)) {
    Write-Host "$ServiceName already listens on port $Port; reusing existing process."
    return
  }

  $serviceDir = Join-Path $ServicesRoot $ServiceName
  $logsDir = Join-Path $serviceDir "logs"
  New-Item -ItemType Directory -Force -Path $logsDir | Out-Null
  $argumentList = @("server", "-c", "configs/config.yaml")
  $previousMallGrpcPort = [Environment]::GetEnvironmentVariable("BBS_MALL_GRPC_SERVER_PORT", "Process")
  $previousMallServiceGrpcPort = [Environment]::GetEnvironmentVariable("BBS_MALL_SERVICE_GRPC_PORT", "Process")
  $previousSearchGrpcPort = [Environment]::GetEnvironmentVariable("BBS_SEARCH_GRPC_SERVER_PORT", "Process")
  $previousSearchServiceGrpcPort = [Environment]::GetEnvironmentVariable("BBS_SEARCH_SERVICE_GRPC_PORT", "Process")
  try {
    if ($ServiceName -eq "mall-service" -and $MallPort -gt 0) {
      [Environment]::SetEnvironmentVariable("BBS_MALL_GRPC_SERVER_PORT", "$MallPort", "Process")
      [Environment]::SetEnvironmentVariable("BBS_MALL_SERVICE_GRPC_PORT", "$MallPort", "Process")
    }
    if ($ServiceName -eq "search-service" -and $SearchPort -gt 0) {
      [Environment]::SetEnvironmentVariable("BBS_SEARCH_GRPC_SERVER_PORT", "$SearchPort", "Process")
      [Environment]::SetEnvironmentVariable("BBS_SEARCH_SERVICE_GRPC_PORT", "$SearchPort", "Process")
    }
    $process = Start-Process `
      -FilePath (Join-Path $serviceDir "bin\$ServiceName.exe") `
      -ArgumentList $argumentList `
      -WorkingDirectory $serviceDir `
      -RedirectStandardOutput (Join-Path $logsDir "smoke-out.log") `
      -RedirectStandardError (Join-Path $logsDir "smoke-err.log") `
      -WindowStyle Hidden `
      -PassThru
  } finally {
    if ($ServiceName -eq "mall-service" -and $MallPort -gt 0) {
      [Environment]::SetEnvironmentVariable("BBS_MALL_GRPC_SERVER_PORT", $previousMallGrpcPort, "Process")
      [Environment]::SetEnvironmentVariable("BBS_MALL_SERVICE_GRPC_PORT", $previousMallServiceGrpcPort, "Process")
    }
    if ($ServiceName -eq "search-service" -and $SearchPort -gt 0) {
      [Environment]::SetEnvironmentVariable("BBS_SEARCH_GRPC_SERVER_PORT", $previousSearchGrpcPort, "Process")
      [Environment]::SetEnvironmentVariable("BBS_SEARCH_SERVICE_GRPC_PORT", $previousSearchServiceGrpcPort, "Process")
    }
  }
  $Started.Add($process)
}

function Stop-StartedProcesses {
  foreach ($process in $Started) {
    try {
      if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit(5000) | Out-Null
      }
    } catch {
    }
  }
}

try {
  if ($RefreshRunningServices) {
    Refresh-RunningServiceProcesses
  }

  $reuseGateway = Assert-PortReusableOrFree "api-gateway" $GatewayPort
  if ($reuseGateway) {
    Write-Host "Gateway port $GatewayPort is in use; reusing the existing gateway for smoke."
  }

  if (-not $SkipBuild) {
    foreach ($service in $Services) {
      Invoke-GoBuild $service.Name
    }
    Invoke-GoBuild "api-gateway"
  }

  Invoke-ServiceMigrate "user-service"
  Invoke-ServiceMigrate "admin-service"
  Invoke-ServiceMigrate "content-service"
  Invoke-ServiceMigrate "reaction-service"
  Invoke-ServiceMigrate "mall-service"
  Invoke-ServiceMigrate "file-service"

  foreach ($service in $Services) {
    Start-ServiceProcess $service.Name $service.Port
  }
  foreach ($service in $Services) {
    Wait-Port $service.Port
  }
  if ($RefreshRunningServices) {
    foreach ($service in $Services) {
      Wait-ForSingleServiceDiscoveryRegistration $service.Name $service.Port
    }
  }

  if (-not $reuseGateway) {
    $gatewayDir = Join-Path $ServicesRoot "api-gateway"
    $gatewayLogsDir = Join-Path $gatewayDir "logs"
    New-Item -ItemType Directory -Force -Path $gatewayLogsDir | Out-Null
    $previousGatewayPort = [Environment]::GetEnvironmentVariable("BBS_GATEWAY_SERVICE_HTTP_PORT", "Process")
    try {
      [Environment]::SetEnvironmentVariable("BBS_GATEWAY_SERVICE_HTTP_PORT", "$GatewayPort", "Process")
      $gateway = Start-Process `
        -FilePath (Join-Path $gatewayDir "bin\api-gateway.exe") `
        -ArgumentList @("server", "-c", "configs\config.yaml") `
        -WorkingDirectory $gatewayDir `
        -RedirectStandardOutput (Join-Path $gatewayLogsDir "smoke-out.log") `
        -RedirectStandardError (Join-Path $gatewayLogsDir "smoke-err.log") `
        -WindowStyle Hidden `
        -PassThru
    } finally {
      [Environment]::SetEnvironmentVariable("BBS_GATEWAY_SERVICE_HTTP_PORT", $previousGatewayPort, "Process")
    }
    $Started.Add($gateway)
  }

  $baseUrl = "http://127.0.0.1:$GatewayPort"
  if (-not (Test-TcpEndpoint "127.0.0.1" 1025 500)) {
    throw "Mailpit SMTP is required for password reset and email verification smoke at 127.0.0.1:1025"
  }
  if (-not (Test-TcpEndpoint "127.0.0.1" 8025 500)) {
    throw "Mailpit HTTP API is required for password reset and email verification smoke at 127.0.0.1:8025"
  }
  Wait-Http "http://127.0.0.1:8025/api/v1/info"
  Wait-Http "$baseUrl/healthz"

  $authConfig = Invoke-Api -Uri "$baseUrl/api/v1/auth/config" -Method Get -TimeoutSec 10
  foreach ($property in @("password_enabled", "register_enabled", "webmaster_enabled", "oauth_callback_hint", "providers")) {
    Assert-ObjectProperty $authConfig $property "Auth config did not include $property"
  }
  $authProviderNames = @($authConfig.providers | ForEach-Object { $_.provider })
  foreach ($providerName in @("github", "qq", "google")) {
    if ($authProviderNames -notcontains $providerName) {
      throw "Auth config did not include $providerName provider"
    }
    $provider = @($authConfig.providers | Where-Object { $_.provider -eq $providerName })[0]
    foreach ($property in @("enabled", "label", "start_url")) {
      Assert-ObjectProperty $provider $property "Auth config provider $providerName did not include $property"
    }
    if ([string]::IsNullOrWhiteSpace([string]$provider.start_url)) {
      throw "Auth config provider $providerName did not include start_url"
    }
  }
  $githubAuthProvider = @($authConfig.providers | Where-Object { $_.provider -eq "github" })[0]
  Assert-ObjectProperty $githubAuthProvider "min_account_years" "Auth config GitHub provider did not include min_account_years"

  $stamp = Get-Date -Format "yyyyMMddHHmmss"
  $username = "smoke$stamp"
  $password = "Password123!"
  $registerBody = @{
    username = $username
    email = "$username@example.com"
    password = $password
    nickname = "Smoke User"
  } | ConvertTo-Json
  $register = Invoke-Api -Uri "$baseUrl/api/v1/auth/register" -Method Post -ContentType "application/json" -Body $registerBody -TimeoutSec 10
  $token = $register.access_token
  if (-not $token) {
    throw "Register response did not include access_token"
  }

  $headers = @{ Authorization = "Bearer $token" }
  $me = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Get -Headers $headers -TimeoutSec 10
  if (-not $me.user.id) {
    throw "Current user response did not include user.id"
  }
  $userBadges = Invoke-Api -Uri "$baseUrl/api/v1/users/$($me.user.id)/badges?limit=20&offset=0" -Method Get -TimeoutSec 10
  $memberBadgeListed = $false
  foreach ($item in @($userBadges.items)) {
    if ($item.id -eq "community-member") {
      $memberBadgeListed = $true
    }
  }
  if (-not $memberBadgeListed) {
    throw "User badge list did not include community-member badge"
  }

  $changedPassword = "Password456!"
  $changePasswordBody = @{
    old_password = $password
    new_password = $changedPassword
  } | ConvertTo-Json
  Invoke-Api -Uri "$baseUrl/api/v1/users/me/password" -Method Post -Headers $headers -ContentType "application/json" -Body $changePasswordBody -TimeoutSec 10 | Out-Null
  $oldPasswordLoginBody = @{
    account = $username
    password = $password
  } | ConvertTo-Json
  Assert-ApiStatus 400 -Uri "$baseUrl/api/v1/auth/login" -Method Post -ContentType "application/json" -Body $oldPasswordLoginBody -TimeoutSec 10
  $changedPasswordLoginBody = @{
    account = $username
    password = $changedPassword
  } | ConvertTo-Json
  $changedPasswordLogin = Invoke-Api -Uri "$baseUrl/api/v1/auth/login" -Method Post -ContentType "application/json" -Body $changedPasswordLoginBody -TimeoutSec 10
  if (-not $changedPasswordLogin.access_token) {
    throw "Changed password login response did not include access_token"
  }
  if ([string]$changedPasswordLogin.user.id -ne [string]$me.user.id) {
    throw "Changed password login returned a different user"
  }
  $token = $changedPasswordLogin.access_token
  $headers = @{ Authorization = "Bearer $token" }

  $resetPassword = "Password789!"
  $passwordResetRequestBody = @{
    email = "$username@example.com"
  } | ConvertTo-Json
  $passwordResetRequest = Invoke-Api -Uri "$baseUrl/api/v1/auth/password/forgot" -Method Post -ContentType "application/json" -Body $passwordResetRequestBody -TimeoutSec 10
  if (-not $passwordResetRequest.accepted -or $null -ne $passwordResetRequest.reset_token -or $null -ne $passwordResetRequest.reset_url) {
    throw "Password reset request did not return a safe accepted response"
  }
  $passwordResetEmail = Wait-MailpitActionMessage -Recipient "$username@example.com" -ActionPath "/user/password/reset"
  $resetPasswordBody = @{
    token = $passwordResetEmail.Token
    new_password = $resetPassword
  } | ConvertTo-Json
  Invoke-Api -Uri "$baseUrl/api/v1/auth/password/reset" -Method Post -ContentType "application/json" -Body $resetPasswordBody -TimeoutSec 10 | Out-Null
  Assert-ApiStatus 400 -Uri "$baseUrl/api/v1/auth/login" -Method Post -ContentType "application/json" -Body $changedPasswordLoginBody -TimeoutSec 10
  $resetPasswordLogin = Invoke-Api -Uri "$baseUrl/api/v1/auth/login" -Method Post -ContentType "application/json" -Body (@{
    account = $username
    password = $resetPassword
  } | ConvertTo-Json) -TimeoutSec 10
  if (-not $resetPasswordLogin.access_token -or [string]$resetPasswordLogin.user.id -ne [string]$me.user.id) {
    throw "Password reset email link did not produce a valid login for the original user"
  }
  $token = $resetPasswordLogin.access_token
  $headers = @{ Authorization = "Bearer $token" }

  $verificationUsername = "verify$stamp"
  $verificationEmail = "$verificationUsername@example.com"
  $verificationPassword = "Verify123!"
  $verificationRegister = Invoke-Api -Uri "$baseUrl/api/v1/auth/register" -Method Post -ContentType "application/json" -Body (@{
    username = $verificationUsername
    email = $verificationEmail
    password = $verificationPassword
    nickname = "Security Verification"
  } | ConvertTo-Json) -TimeoutSec 10
  if (-not $verificationRegister.access_token) {
    throw "Verification smoke user registration did not return an access token"
  }
  $verificationHeaders = @{ Authorization = "Bearer $($verificationRegister.access_token)" }
  $emailVerificationRequest = Invoke-Api -Uri "$baseUrl/api/v1/auth/email/verification" -Method Post -Headers $verificationHeaders -TimeoutSec 10
  if (-not $emailVerificationRequest.accepted -or $null -ne $emailVerificationRequest.verification_token -or $null -ne $emailVerificationRequest.verify_url) {
    throw "Email verification request did not return a safe accepted response"
  }
  $emailVerificationMessage = Wait-MailpitActionMessage -Recipient $verificationEmail -ActionPath "/user/email/verify"
  $verifiedEmail = Invoke-Api -Uri "$baseUrl/api/v1/auth/email/verify" -Method Post -ContentType "application/json" -Body (@{
    token = $emailVerificationMessage.Token
  } | ConvertTo-Json) -TimeoutSec 10
  if (-not $verifiedEmail.user.email_verified) {
    throw "Email verification link did not verify the smoke user"
  }
  $verifiedSmokeUser = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Get -Headers $verificationHeaders -TimeoutSec 10
  if (-not $verifiedSmokeUser.user.email_verified) {
    throw "Verified email state was not returned through the gateway"
  }

  $profileBody = @{
    nickname = "Smoke Updated"
    avatar_url = ""
    bio = "Local smoke profile"
  } | ConvertTo-Json
  $profile = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Put -Headers $headers -ContentType "application/json" -Body $profileBody -TimeoutSec 10
  if ($profile.user.nickname -ne "Smoke Updated") {
    throw "Profile update did not persist nickname"
  }

  $followeeUsername = "target$stamp"
  $followeeBody = @{
    username = $followeeUsername
    email = "$followeeUsername@example.com"
    password = $password
    nickname = "Follow Target"
  } | ConvertTo-Json
  $followee = Invoke-Api -Uri "$baseUrl/api/v1/auth/register" -Method Post -ContentType "application/json" -Body $followeeBody -TimeoutSec 10
  $followeeId = $followee.user.id
  if (-not $followeeId) {
    throw "Follow target response did not include user.id"
  }
  $followeeToken = $followee.access_token
  if (-not $followeeToken) {
    throw "Follow target response did not include access_token"
  }
  $followeeHeaders = @{ Authorization = "Bearer $followeeToken" }

  $userSearch = Invoke-Api -Uri "$baseUrl/api/v1/search/users?q=$([uri]::EscapeDataString($followeeUsername))&page=1&page_size=10" -Method Get -TimeoutSec 10
  $userSearchListed = $false
  foreach ($item in @($userSearch.items)) {
    if ([string]$item.id -eq [string]$followeeId -and [string]$item.username -eq [string]$followeeUsername) {
      $userSearchListed = $true
    }
  }
  if (-not $userSearchListed) {
    throw "User search did not include follow target"
  }

  $adminUsername = "admin"
  $adminDefaultPassword = "Admin123!"
  $adminLoginBody = @{
    account = $adminUsername
    password = $adminDefaultPassword
  } | ConvertTo-Json
  $admin = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/login" -Method Post -ContentType "application/json" -Body $adminLoginBody -TimeoutSec 10
  $adminToken = $admin.access_token
  if (-not $adminToken) {
    throw "Admin login response did not include access_token"
  }
  $adminHeaders = @{ Authorization = "Bearer $adminToken" }
  $adminProfile = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/profile" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminProfile.roles) -notcontains "admin") {
    throw "Admin profile did not include admin role"
  }
  $adminPerms = @($adminProfile.permissions)
  if (
    $adminPerms -notcontains "system:view_dashboard" -or
    $adminPerms -notcontains "governance:list_articles" -or
    $adminPerms -notcontains "governance:archive_article" -or
    $adminPerms -notcontains "governance:list_topics" -or
    $adminPerms -notcontains "governance:hide_topic" -or
    $adminPerms -notcontains "governance:archive_topic" -or
    $adminPerms -notcontains "governance:list_users" -or
    $adminPerms -notcontains "governance:list_comments" -or
    $adminPerms -notcontains "governance:hide_comment" -or
    $adminPerms -notcontains "governance:list_admin_users" -or
    $adminPerms -notcontains "governance:create_admin_user" -or
    $adminPerms -notcontains "governance:list_roles" -or
    $adminPerms -notcontains "governance:assign_roles" -or
    $adminPerms -notcontains "governance:list_badges" -or
    $adminPerms -notcontains "governance:create_badge" -or
    $adminPerms -notcontains "governance:update_badge" -or
    $adminPerms -notcontains "governance:delete_badge" -or
    $adminPerms -notcontains "governance:list_levels" -or
    $adminPerms -notcontains "governance:create_level" -or
    $adminPerms -notcontains "governance:update_level" -or
    $adminPerms -notcontains "governance:delete_level" -or
    $adminPerms -notcontains "governance:list_forbidden_words" -or
    $adminPerms -notcontains "governance:create_forbidden_word" -or
    $adminPerms -notcontains "governance:update_forbidden_word" -or
    $adminPerms -notcontains "governance:delete_forbidden_word" -or
    $adminPerms -notcontains "governance:list_settings" -or
    $adminPerms -notcontains "governance:update_setting" -or
    $adminPerms -notcontains "governance:list_email_logs" -or
    $adminPerms -notcontains "governance:list_links" -or
    $adminPerms -notcontains "governance:create_link" -or
    $adminPerms -notcontains "governance:update_link" -or
    $adminPerms -notcontains "governance:delete_link" -or
    $adminPerms -notcontains "governance:list_tasks" -or
    $adminPerms -notcontains "governance:create_task" -or
    $adminPerms -notcontains "governance:update_task" -or
    $adminPerms -notcontains "governance:delete_task" -or
    $adminPerms -notcontains "mall:list_product_categories" -or
    $adminPerms -notcontains "mall:create_product_category" -or
    $adminPerms -notcontains "mall:update_product_category" -or
    $adminPerms -notcontains "mall:list_products" -or
    $adminPerms -notcontains "mall:create_product" -or
    $adminPerms -notcontains "mall:update_product" -or
    $adminPerms -notcontains "mall:list_product_reviews" -or
    $adminPerms -notcontains "mall:update_product_review" -or
    $adminPerms -notcontains "mall:list_coupons" -or
    $adminPerms -notcontains "mall:list_coupon_usages" -or
    $adminPerms -notcontains "mall:create_coupon" -or
    $adminPerms -notcontains "mall:update_coupon" -or
    $adminPerms -notcontains "mall:list_orders" -or
    $adminPerms -notcontains "mall:close_expired_orders" -or
    $adminPerms -notcontains "mall:recover_paying_orders" -or
    $adminPerms -notcontains "mall:requeue_outbox_events" -or
    $adminPerms -notcontains "mall:update_order_status" -or
    $adminPerms -notcontains "mall:list_order_logs" -or
    $adminPerms -notcontains "mall:list_order_payments" -or
    $adminPerms -notcontains "mall:list_digital_entitlements" -or
    $adminPerms -notcontains "mall:revoke_digital_entitlement" -or
    $adminPerms -notcontains "mall:list_refunds" -or
    $adminPerms -notcontains "mall:review_refunds"
  ) {
    throw "Admin profile did not include expected governance, mall, or dashboard permissions"
  }
  $adminOverview = Invoke-Api -Uri "$baseUrl/api/v1/admin/overview" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminOverview.metrics).Count -lt 4 -or @($adminOverview.daily).Count -lt 14) {
    throw "Admin dashboard overview did not include expected metrics or daily rows"
  }

  $governanceUsers = Invoke-Api -Uri "$baseUrl/api/v1/admin/users?query=$username&status=0&page=1&page_size=20" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $governanceUserListed = $false
  foreach ($item in @($governanceUsers.items)) {
    if ([string]$item.id -eq [string]$me.user.id) {
      $governanceUserListed = $true
    }
  }
  if (-not $governanceUserListed) {
    throw "Admin user governance list did not include registered smoke user"
  }

  $roles = Invoke-Api -Uri "$baseUrl/api/v1/admin/rbac/roles" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $roleKeys = @($roles.items | ForEach-Object { $_.key })
  if ($roleKeys -notcontains "admin" -or $roleKeys -notcontains "moderator") {
    throw "Admin RBAC role list did not include expected roles"
  }
  $adminUsers = Invoke-Api -Uri "$baseUrl/api/v1/admin/rbac/users?limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $bootstrapAdminListed = $false
  foreach ($item in @($adminUsers.items)) {
    if ($item.username -eq $adminUsername) {
      $bootstrapAdminListed = $true
    }
  }
  if (-not $bootstrapAdminListed) {
    throw "Admin RBAC user list did not include bootstrap admin"
  }
  $rbacUsername = "rbac$stamp"
  $rbacPassword = "Admin123!$stamp"
  $createAdminBody = @{
    username = $rbacUsername
    email = "$rbacUsername@admin.local"
    phone = ""
    password = $rbacPassword
    role_keys = @("moderator")
  } | ConvertTo-Json
  $createdAdmin = Invoke-Api -Uri "$baseUrl/api/v1/admin/rbac/users" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $createAdminBody -TimeoutSec 10
  if (-not $createdAdmin.user.id -or @($createdAdmin.user.roles) -notcontains "moderator") {
    throw "Admin RBAC create user did not return moderator admin user"
  }
  $createdAdminLoginBody = @{
    account = $rbacUsername
    password = $rbacPassword
  } | ConvertTo-Json
  $moderatorLogin = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/login" -Method Post -ContentType "application/json" -Body $createdAdminLoginBody -TimeoutSec 10
  if (-not $moderatorLogin.access_token -or @($moderatorLogin.roles) -notcontains "moderator" -or @($moderatorLogin.roles) -contains "admin") {
    throw "Created moderator could not login with exactly the moderator role"
  }
  $moderatorHeaders = @{ Authorization = "Bearer $($moderatorLogin.access_token)" }
  $moderatorProfile = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/profile" -Method Get -Headers $moderatorHeaders -TimeoutSec 10
  $moderatorPerms = @($moderatorProfile.permissions)
  if ($moderatorPerms -contains "system:view_dashboard") {
    throw "Moderator profile unexpectedly included dashboard permission"
  }
  $moderatorDashboardForbidden = $false
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/admin/overview" -Method Get -Headers $moderatorHeaders -TimeoutSec 10
  $moderatorDashboardForbidden = $true

  $systemMenus = Invoke-Api -Uri "$baseUrl/api/v1/admin/system/menus?page_size=800" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $allSystemMenus = @(Expand-TreeNodes -Nodes @($systemMenus.items))
  $expectedMallReadonlyNodeNames = @(
    "mall",
    "mall.orders",
    "mall.orders.query",
    "mall.overview",
    "mall.overview.query"
  )
  $mallReadonlyMenuIds = @(
    $allSystemMenus |
      Where-Object { $expectedMallReadonlyNodeNames -contains $_.name } |
      ForEach-Object { [int64]$_.id }
  )
  if ($mallReadonlyMenuIds.Count -ne $expectedMallReadonlyNodeNames.Count) {
    throw "System menu list did not include all mall readonly smoke menu nodes"
  }
  $mallReadonlyRoleKey = "mall_order_viewer_$stamp"
  $mallReadonlyRoleBody = @{
    key = $mallReadonlyRoleKey
    name = "Mall Order Viewer $stamp"
    remark = "Smoke restricted mall order viewer"
    data_scope = "1"
    sort = 990
    admin = $false
    status = "1"
  } | ConvertTo-Json
  $mallReadonlyRole = Invoke-Api -Uri "$baseUrl/api/v1/admin/system/roles" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $mallReadonlyRoleBody -TimeoutSec 10
  if (-not $mallReadonlyRole.role.id) {
    throw "System role create did not return mall readonly role id"
  }
  $mallReadonlyAssignBody = @{
    menu_ids = $mallReadonlyMenuIds
  } | ConvertTo-Json
  $assignedMallReadonlyRole = Invoke-Api -Uri "$baseUrl/api/v1/admin/system/roles/$($mallReadonlyRole.role.id)/menus" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $mallReadonlyAssignBody -TimeoutSec 10
  $assignedMallReadonlyPerms = @($assignedMallReadonlyRole.role.permissions)
  if ($assignedMallReadonlyPerms.Count -ne 1 -or $assignedMallReadonlyPerms -notcontains "mall:list_orders") {
    throw "Mall readonly role permissions were not limited to mall:list_orders"
  }
  $mallReadonlyUsername = "mallviewer$stamp"
  $mallReadonlyPassword = "Viewer123!$stamp"
  $mallReadonlyUserBody = @{
    username = $mallReadonlyUsername
    nickname = "Mall Order Viewer"
    password = $mallReadonlyPassword
    email = "$mallReadonlyUsername@admin.local"
    phone = ""
    avatar_url = ""
    status = 1
    dept_id = 0
    post_id = 0
    role_ids = [int64[]]@($mallReadonlyRole.role.id)
  } | ConvertTo-Json
  $mallReadonlyUser = Invoke-Api -Uri "$baseUrl/api/v1/admin/system/users" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $mallReadonlyUserBody -TimeoutSec 10
  if (-not $mallReadonlyUser.user.id) {
    throw "System user create did not return mall readonly user id"
  }
  $mallReadonlyLoginBody = @{
    account = $mallReadonlyUsername
    password = $mallReadonlyPassword
  } | ConvertTo-Json
  $mallReadonlyLogin = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/login" -Method Post -ContentType "application/json" -Body $mallReadonlyLoginBody -TimeoutSec 10
  if (-not $mallReadonlyLogin.access_token -or @($mallReadonlyLogin.permissions) -notcontains "mall:list_orders") {
    throw "Mall readonly system user could not login with mall:list_orders"
  }
  $mallReadonlyHeaders = @{ Authorization = "Bearer $($mallReadonlyLogin.access_token)" }
  $mallReadonlyProfile = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/profile" -Method Get -Headers $mallReadonlyHeaders -TimeoutSec 10
  $mallReadonlyPerms = @($mallReadonlyProfile.permissions)
  if ($mallReadonlyPerms.Count -ne 1 -or $mallReadonlyPerms -notcontains "mall:list_orders") {
    throw "Mall readonly profile permissions were not limited to mall:list_orders"
  }
  $mallReadonlyMenus = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/menus" -Method Get -Headers $mallReadonlyHeaders -TimeoutSec 10
  $expectedMallReadonlyRouteNames = @("mall", "mall.overview", "mall.orders")
  $mallReadonlyMenuNames = @(
    Expand-TreeNodes -Nodes @($mallReadonlyMenus.items) |
      Where-Object { [string]$_.type -ne "F" } |
      ForEach-Object { [string]$_.name }
  )
  $missingMallReadonlyRoutes = @($expectedMallReadonlyRouteNames | Where-Object { $mallReadonlyMenuNames -notcontains $_ })
  $unexpectedMallReadonlyRoutes = @($mallReadonlyMenuNames | Where-Object { $expectedMallReadonlyRouteNames -notcontains $_ })
  if ($missingMallReadonlyRoutes.Count -gt 0 -or $unexpectedMallReadonlyRoutes.Count -gt 0) {
    throw "Mall readonly auth menus did not match expected routes"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders?page_size=1" -Method Get -Headers $mallReadonlyHeaders -TimeoutSec 10 | Out-Null
  Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/overview?low_stock_threshold=10" -Method Get -Headers $mallReadonlyHeaders -TimeoutSec 10 | Out-Null
  $mallReadonlyProductsForbidden = $false
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/admin/mall/products?page_size=1" -Method Get -Headers $mallReadonlyHeaders -TimeoutSec 10
  $mallReadonlyProductsForbidden = $true
  $mallReadonlyCloseExpiredForbidden = $false
  $mallReadonlyCloseExpiredBody = @{
    expire_after_seconds = 60
    limit = 1
  } | ConvertTo-Json
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/admin/mall/orders/expire" -Method Post -Headers $mallReadonlyHeaders -ContentType "application/json" -Body $mallReadonlyCloseExpiredBody -TimeoutSec 10
  $mallReadonlyCloseExpiredForbidden = $true
  $mallReadonlyRecoverPayingForbidden = $false
  $mallReadonlyRecoverPayingBody = @{
    stale_after_seconds = 60
    limit = 1
  } | ConvertTo-Json
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/admin/mall/orders/recover-paying" -Method Post -Headers $mallReadonlyHeaders -ContentType "application/json" -Body $mallReadonlyRecoverPayingBody -TimeoutSec 10
  $mallReadonlyRecoverPayingForbidden = $true
  $mallReadonlyOutboxRequeueForbidden = $false
  $mallReadonlyOutboxRequeueBody = @{
    statuses = @("failed", "dead_letter")
    limit = 1
  } | ConvertTo-Json -Depth 4
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/admin/mall/outbox/requeue" -Method Post -Headers $mallReadonlyHeaders -ContentType "application/json" -Body $mallReadonlyOutboxRequeueBody -TimeoutSec 10
  $mallReadonlyOutboxRequeueForbidden = $true

  $assignAdminBody = @{
    role_keys = @("admin")
  } | ConvertTo-Json
  $assignedAdmin = Invoke-Api -Uri "$baseUrl/api/v1/admin/rbac/users/$($createdAdmin.user.id)/roles" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $assignAdminBody -TimeoutSec 10
  if (@($assignedAdmin.user.roles) -notcontains "admin") {
    throw "Admin RBAC assign roles did not return admin role"
  }
  $createdAdminLogin = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/login" -Method Post -ContentType "application/json" -Body $createdAdminLoginBody -TimeoutSec 10
  if (-not $createdAdminLogin.access_token -or @($createdAdminLogin.roles) -notcontains "admin") {
    throw "Created admin could not login with assigned admin role"
  }
  $createdAdminHeaders = @{ Authorization = "Bearer $($createdAdminLogin.access_token)" }
  $createdAdminOverview = Invoke-Api -Uri "$baseUrl/api/v1/admin/overview" -Method Get -Headers $createdAdminHeaders -TimeoutSec 10
  if (@($createdAdminOverview.metrics).Count -lt 4 -or @($createdAdminOverview.daily).Count -lt 14) {
    throw "Created admin dashboard overview did not include expected metrics or daily rows"
  }

  Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/follow" -Method Post -Headers $headers -TimeoutSec 10 | Out-Null
  $followState = Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/following-state" -Method Get -Headers $headers -TimeoutSec 10
  if (-not $followState.following) {
    throw "Follow state was not true after follow"
  }
  $followers = Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/followers?page=1&page_size=10" -Method Get -TimeoutSec 10
  if (@($followers.items).Count -lt 1) {
    throw "Followers list did not include any follower after follow"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/follow" -Method Delete -Headers $headers -TimeoutSec 10 | Out-Null
  $unfollowState = Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/following-state" -Method Get -Headers $headers -TimeoutSec 10
  if ($unfollowState.following) {
    throw "Follow state was still true after unfollow"
  }
  Assert-ApiStatus 401 -Uri "$baseUrl/api/v1/feed?sort=follow&limit=10&offset=0" -Method Get -TimeoutSec 10
  Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/follow" -Method Post -Headers $headers -TimeoutSec 10 | Out-Null
  $refollowState = Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/following-state" -Method Get -Headers $headers -TimeoutSec 10
  if (-not $refollowState.following) {
    throw "Follow state was not true after refollow for following feed"
  }

  $categories = Invoke-Api -Uri "$baseUrl/api/v1/categories?status=2&limit=20&offset=0" -Method Get -TimeoutSec 10
  $defaultCategory = $null
  foreach ($item in @($categories.items)) {
    if ($item.slug -eq "general") {
      $defaultCategory = $item
    }
  }
  if (-not $defaultCategory -or -not $defaultCategory.id) {
    throw "Category list did not include seeded general category"
  }
  $categoryId = [int64]$defaultCategory.id
  $categoryDetail = Invoke-Api -Uri "$baseUrl/api/v1/categories/$categoryId" -Method Get -TimeoutSec 10
  if ($categoryDetail.category.slug -ne "general") {
    throw "Category detail did not return seeded general category"
  }
  $links = Invoke-Api -Uri "$baseUrl/api/v1/links?limit=20&offset=0" -Method Get -TimeoutSec 10
  if (@($links.items).Count -lt 1) {
    throw "Links endpoint did not return persisted links"
  }
  $tasks = Invoke-Api -Uri "$baseUrl/api/v1/tasks?limit=20&offset=0" -Method Get -TimeoutSec 10
  if (@($tasks.items).Count -lt 1) {
    throw "Tasks endpoint did not return persisted tasks"
  }
  $adminCategories = Invoke-Api -Uri "$baseUrl/api/v1/admin/categories?status=0&limit=50&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $adminCategoryListed = $false
  foreach ($item in @($adminCategories.items)) {
    if ($item.slug -eq "general") {
      $adminCategoryListed = $true
    }
  }
  if (-not $adminCategoryListed) {
    throw "Admin category list did not include seeded general category"
  }
  $adminBadges = Invoke-Api -Uri "$baseUrl/api/v1/admin/badges?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminBadges.items).Count -lt 1) {
    throw "Admin badges endpoint did not return persisted badges"
  }
  $badgeBody = @{
    key = "smoke-badge-$stamp"
    name = "Smoke Badge $stamp"
    description = "Created by local smoke test."
    rule_type = "manual"
    rule_value = 0
    status = 2
    sort = 90
  } | ConvertTo-Json
  $createdBadge = Invoke-Api -Uri "$baseUrl/api/v1/admin/badges" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $badgeBody -TimeoutSec 10
  $createdBadgeId = $createdBadge.badge.id
  if (-not $createdBadgeId) {
    throw "Admin badge create did not return badge.id"
  }
  $updateBadgeBody = @{
    name = "Smoke Badge Updated $stamp"
    description = "Updated by local smoke test."
    rule_type = "manual"
    rule_value = 0
    status = 2
    sort = 91
  } | ConvertTo-Json
  $updatedBadge = Invoke-Api -Uri "$baseUrl/api/v1/admin/badges/$createdBadgeId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateBadgeBody -TimeoutSec 10
  if ($updatedBadge.badge.name -ne "Smoke Badge Updated $stamp") {
    throw "Admin badge update did not persist name"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/badges/$createdBadgeId" -Method Delete -Headers $adminHeaders -TimeoutSec 10 | Out-Null

  $adminLevels = Invoke-Api -Uri "$baseUrl/api/v1/admin/levels?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminLevels.items).Count -lt 1) {
    throw "Admin levels endpoint did not return persisted levels"
  }
  $publicLevels = Invoke-Api -Uri "$baseUrl/api/v1/levels?limit=20&offset=0" -Method Get -TimeoutSec 10
  if (@($publicLevels.items).Count -lt 1) {
    throw "Public levels endpoint did not return active levels"
  }
  $levelBody = @{
    key = "smoke-level-$stamp"
    name = "Smoke Level $stamp"
    description = "Created by local smoke test."
    min_score = 777
    max_score = 888
    status = 2
    sort = 90
  } | ConvertTo-Json
  $createdLevel = Invoke-Api -Uri "$baseUrl/api/v1/admin/levels" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $levelBody -TimeoutSec 10
  $createdLevelId = $createdLevel.level.id
  if (-not $createdLevelId) {
    throw "Admin level create did not return level.id"
  }
  $updateLevelBody = @{
    name = "Smoke Level Updated $stamp"
    description = "Updated by local smoke test."
    min_score = 778
    max_score = 889
    status = 2
    sort = 91
  } | ConvertTo-Json
  $updatedLevel = Invoke-Api -Uri "$baseUrl/api/v1/admin/levels/$createdLevelId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateLevelBody -TimeoutSec 10
  if ($updatedLevel.level.min_score -ne 778) {
    throw "Admin level update did not persist min_score"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/levels/$createdLevelId" -Method Delete -Headers $adminHeaders -TimeoutSec 10 | Out-Null

  $adminForbiddenWords = Invoke-Api -Uri "$baseUrl/api/v1/admin/forbidden-words?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminForbiddenWords.items).Count -lt 1) {
    throw "Admin forbidden words endpoint did not return persisted rows"
  }
  $forbiddenWordBody = @{
    word = "smoke-word-$stamp"
    scene = "content"
    action = "review"
    replacement = ""
    description = "Created by local smoke test."
    status = 2
  } | ConvertTo-Json
  $createdForbiddenWord = Invoke-Api -Uri "$baseUrl/api/v1/admin/forbidden-words" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $forbiddenWordBody -TimeoutSec 10
  $createdForbiddenWordId = $createdForbiddenWord.word.id
  if (-not $createdForbiddenWordId) {
    throw "Admin forbidden word create did not return word.id"
  }
  $updateForbiddenWordBody = @{
    word = "smoke-word-updated-$stamp"
    scene = "comment"
    action = "replace"
    replacement = "***"
    description = "Updated by local smoke test."
    status = 2
  } | ConvertTo-Json
  $updatedForbiddenWord = Invoke-Api -Uri "$baseUrl/api/v1/admin/forbidden-words/$createdForbiddenWordId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateForbiddenWordBody -TimeoutSec 10
  if ($updatedForbiddenWord.word.action -ne "replace") {
    throw "Admin forbidden word update did not persist action"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/forbidden-words/$createdForbiddenWordId" -Method Delete -Headers $adminHeaders -TimeoutSec 10 | Out-Null

  $adminSettings = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings?limit=100&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminSettings.items).Count -lt 1 -or -not $adminSettings.settings.site_name) {
    throw "Admin settings endpoint did not return seeded site settings"
  }
  $settingBody = @{
    key = "smoke_setting"
    value = "enabled"
    group = "smoke"
    value_type = "string"
    description = "Created by local smoke test."
    status = 2
  } | ConvertTo-Json
  $updatedSetting = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/smoke_setting" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $settingBody -TimeoutSec 10
  if ($updatedSetting.setting.value -ne "enabled") {
    throw "Admin setting update did not persist value"
  }
  $secretSettingBody = @{
    key = "smoke_secret"
    value = "secret-$stamp"
    group = "smoke"
    value_type = "password"
    description = "Created by local smoke test."
    status = 2
  } | ConvertTo-Json
  $createdSecretSetting = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/smoke_secret" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $secretSettingBody -TimeoutSec 10
  if ([string]::IsNullOrWhiteSpace([string]$createdSecretSetting.setting.value) -or $createdSecretSetting.setting.value -eq "secret-$stamp") {
    throw "Admin secret setting response did not mask the persisted password value"
  }
  $clearSecretSettingBody = @{
    key = "smoke_secret"
    value = ""
    group = "smoke"
    value_type = "password"
    description = "Created by local smoke test."
    status = 2
    clear_value = $true
  } | ConvertTo-Json
  $clearedSecretSetting = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/smoke_secret" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $clearSecretSettingBody -TimeoutSec 10
  if (-not [string]::IsNullOrEmpty([string]$clearedSecretSetting.setting.value)) {
    throw "Admin secret setting clear_value did not clear the password value"
  }

  $adminEmailLogs = Invoke-Api -Uri "$baseUrl/api/v1/admin/email-logs?page=1&page_size=20" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if ($null -eq $adminEmailLogs.items -or $null -eq $adminEmailLogs.list -or $null -eq $adminEmailLogs.total -or [int]$adminEmailLogs.currentPage -ne 1 -or [int]$adminEmailLogs.pageSize -ne 20) {
    throw "Admin email logs endpoint did not return a paginated table response"
  }

  $adminLinks = Invoke-Api -Uri "$baseUrl/api/v1/admin/links?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminLinks.items).Count -lt 1) {
    throw "Admin links endpoint did not return persisted links"
  }
  $linkBody = @{
    key = "smoke-link-$stamp"
    title = "Smoke Link $stamp"
    url = "https://example.com/$stamp"
    description = "Created by local smoke test."
    status = 2
    sort = 90
  } | ConvertTo-Json
  $createdLink = Invoke-Api -Uri "$baseUrl/api/v1/admin/links" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $linkBody -TimeoutSec 10
  $createdLinkId = $createdLink.link.id
  if (-not $createdLinkId) {
    throw "Admin link create did not return link.id"
  }
  $updateLinkBody = @{
    title = "Smoke Link Updated $stamp"
    url = "https://example.com/$stamp/updated"
    description = "Updated by local smoke test."
    status = 2
    sort = 91
  } | ConvertTo-Json
  $updatedLink = Invoke-Api -Uri "$baseUrl/api/v1/admin/links/$createdLinkId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateLinkBody -TimeoutSec 10
  if ($updatedLink.link.title -ne "Smoke Link Updated $stamp") {
    throw "Admin link update did not persist title"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/links/$createdLinkId" -Method Delete -Headers $adminHeaders -TimeoutSec 10 | Out-Null

  $adminTasks = Invoke-Api -Uri "$baseUrl/api/v1/admin/tasks?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminTasks.items).Count -lt 1) {
    throw "Admin tasks endpoint did not return persisted tasks"
  }
  $unsupportedTaskBody = @{
    key = "smoke-task-$stamp"
    title = "Smoke Task $stamp"
    description = "An unclaimable task must be rejected."
    reward_points = 7
    status = 2
    sort = 90
  } | ConvertTo-Json
  Assert-ApiStatus -ExpectedStatus 412 -Uri "$baseUrl/api/v1/admin/tasks" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $unsupportedTaskBody -TimeoutSec 10

  $claimableTask = @($adminTasks.items | Where-Object { [string]$_.key -eq "daily_check_in" } | Select-Object -First 1)
  if ($claimableTask.Count -ne 1 -or [int64]$claimableTask[0].id -le 0) {
    throw "Admin tasks endpoint did not include the claimable daily_check_in task"
  }
  $claimableTask = $claimableTask[0]
  $claimableTaskId = [int64]$claimableTask.id
  $managedTaskCreateBody = @{
    key = [string]$claimableTask.key
    title = "Duplicate claimable task $stamp"
    description = "Built-in task definitions must not be created through the admin API."
    reward_points = 7
    status = 2
    sort = 90
  } | ConvertTo-Json
  Assert-ApiStatus -ExpectedStatus 412 -Uri "$baseUrl/api/v1/admin/tasks" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $managedTaskCreateBody -TimeoutSec 10
  Assert-ApiStatus -ExpectedStatus 412 -Uri "$baseUrl/api/v1/admin/tasks/$claimableTaskId" -Method Delete -Headers $adminHeaders -TimeoutSec 10
  $restoreTaskBody = @{
    key = [string]$claimableTask.key
    title = [string]$claimableTask.title
    description = [string]$claimableTask.description
    reward_points = [int64]$claimableTask.reward_points
    status = [int64]$claimableTask.status
    sort = [int64]$claimableTask.sort
  } | ConvertTo-Json
  $updateTaskBody = @{
    key = [string]$claimableTask.key
    title = "Smoke Task Updated $stamp"
    description = "Updated by local smoke test."
    reward_points = 9
    status = 2
    sort = 91
  } | ConvertTo-Json
  try {
    $updatedTask = Invoke-Api -Uri "$baseUrl/api/v1/admin/tasks/$claimableTaskId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateTaskBody -TimeoutSec 10
    if ($updatedTask.task.reward_points -ne 9 -or $updatedTask.task.key -ne $claimableTask.key) {
      throw "Admin claimable task update did not persist the configured values"
    }
  } finally {
    $restoredTask = Invoke-Api -Uri "$baseUrl/api/v1/admin/tasks/$claimableTaskId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $restoreTaskBody -TimeoutSec 10
    if ($restoredTask.task.reward_points -ne $claimableTask.reward_points -or $restoredTask.task.key -ne $claimableTask.key) {
      throw "Admin claimable task restore did not persist the original values"
    }
  }

  $permissionTopicBody = @{
    slug = "permission-topic-$stamp"
    type = "topic"
    title = "Permission topic $stamp"
    body = "This topic validates author-only updates and deletes."
    tags = @("permission", "topic")
    category_id = $categoryId
    publish = $true
  } | ConvertTo-Json
  $permissionTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $permissionTopicBody -TimeoutSec 10
  $permissionTopicId = $permissionTopic.topic.id
  if (-not $permissionTopicId) {
    throw "Permission topic create response did not include topic.id"
  }
  $permissionTopicUpdateBody = @{
    title = "Permission topic updated $stamp"
    body = "Updated by the owner."
    tags = @("permission", "updated")
    category_id = $categoryId
  } | ConvertTo-Json
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics/$permissionTopicId" -Method Put -Headers $followeeHeaders -ContentType "application/json" -Body $permissionTopicUpdateBody -TimeoutSec 10
  $updatedPermissionTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics/$permissionTopicId" -Method Put -Headers $headers -ContentType "application/json" -Body $permissionTopicUpdateBody -TimeoutSec 10
  if ($updatedPermissionTopic.topic.title -ne "Permission topic updated $stamp") {
    throw "Owner topic update did not persist title"
  }
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics/$permissionTopicId" -Method Delete -Headers $followeeHeaders -TimeoutSec 10
  $deletedPermissionTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics/$permissionTopicId" -Method Delete -Headers $headers -TimeoutSec 10
  if ([int64]$deletedPermissionTopic.topic.status -ne 4) {
    throw "Owner topic delete did not archive topic"
  }

  $permissionArticleBody = @{
    slug = "permission-article-$stamp"
    title = "Permission article $stamp"
    summary = "Permission smoke"
    body = "This article validates author-only updates and deletes."
    cover_url = ""
    tags = @("permission", "article")
    publish = $true
  } | ConvertTo-Json
  $permissionArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles" -Method Post -Headers $headers -ContentType "application/json" -Body $permissionArticleBody -TimeoutSec 10
  $permissionArticleId = $permissionArticle.article.id
  if (-not $permissionArticleId) {
    throw "Permission article create response did not include article.id"
  }
  $permissionArticleUpdateBody = @{
    title = "Permission article updated $stamp"
    summary = "Updated by owner"
    body = "Updated by the owner."
    cover_url = ""
    tags = @("permission", "updated")
  } | ConvertTo-Json
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/articles/$permissionArticleId" -Method Put -Headers $followeeHeaders -ContentType "application/json" -Body $permissionArticleUpdateBody -TimeoutSec 10
  $updatedPermissionArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles/$permissionArticleId" -Method Put -Headers $headers -ContentType "application/json" -Body $permissionArticleUpdateBody -TimeoutSec 10
  if ($updatedPermissionArticle.article.title -ne "Permission article updated $stamp") {
    throw "Owner article update did not persist title"
  }
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/articles/$permissionArticleId" -Method Delete -Headers $followeeHeaders -TimeoutSec 10
  $deletedPermissionArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles/$permissionArticleId" -Method Delete -Headers $headers -TimeoutSec 10
  if ([int64]$deletedPermissionArticle.article.status -ne 4) {
    throw "Owner article delete did not archive article"
  }

  $membershipQaBounty = 30
  $blockedBountyTopicBody = @{
    slug = "qa-bounty-blocked-$stamp"
    type = "qa"
    title = "Blocked bounty QA $stamp"
    body = "This bounty QA must be rejected before a membership entitlement is active."
    tags = @("qa", "bounty")
    category_id = $categoryId
    bounty_score = $membershipQaBounty
    publish = $true
  } | ConvertTo-Json
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $blockedBountyTopicBody -TimeoutSec 10

  $topicTitle = "Topic smoke $stamp"
  $topicBody = @{
    slug = "topic-smoke-$stamp"
    type = "topic"
    title = $topicTitle
    body = "This topic validates the forum topic publishing path."
    tags = @("topic", "smoke")
    category_id = $categoryId
    publish = $true
  } | ConvertTo-Json
  $topic = Invoke-Api -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $topicBody -TimeoutSec 10
  $topicId = $topic.topic.id
  if (-not $topicId -or [int64]$topic.topic.status -ne 2) {
    throw "Topic create response did not include a published topic"
  }
  $topicDetail = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId" -Method Get -TimeoutSec 10
  if ($topicDetail.topic.title -ne $topicTitle) {
    throw "Topic detail did not return created topic"
  }
  if ([int64]$topicDetail.topic.category_id -ne $categoryId) {
    throw "Topic detail did not include selected category_id"
  }
  $topicList = Invoke-Api -Uri "$baseUrl/api/v1/topics?status=2&type=topic&limit=20&offset=0" -Method Get -TimeoutSec 10
  $topicListed = $false
  foreach ($item in @($topicList.items)) {
    if ([string]$item.id -eq [string]$topicId) {
      $topicListed = $true
    }
  }
  if (-not $topicListed) {
    throw "Topic list did not include created topic"
  }
  $categoryTopics = Invoke-Api -Uri "$baseUrl/api/v1/topics?status=2&type=topic&category_id=$categoryId&limit=20&offset=0" -Method Get -TimeoutSec 10
  $categoryTopicListed = $false
  foreach ($item in @($categoryTopics.items)) {
    if ([string]$item.id -eq [string]$topicId -and [int64]$item.category_id -eq $categoryId) {
      $categoryTopicListed = $true
    }
  }
  if (-not $categoryTopicListed) {
    throw "Category topic list did not include created topic"
  }
  $topicCommentBody = @{
    content = "Topic smoke comment"
    parent_id = 0
  } | ConvertTo-Json
  $topicComment = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $topicCommentBody -TimeoutSec 10
  $topicCommentId = $topicComment.comment.id
  if (-not $topicCommentId) {
    throw "Topic comment response did not include comment.id"
  }
  $topicComments = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/comments?page=1&page_size=20" -Method Get -TimeoutSec 10
  if (@($topicComments.items).Count -lt 1) {
    throw "Topic comments did not include created comment"
  }
  $permissionCommentBody = @{
    content = "Permission smoke comment"
    parent_id = 0
  } | ConvertTo-Json
  $permissionComment = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $permissionCommentBody -TimeoutSec 10
  $permissionCommentId = $permissionComment.comment.id
  if (-not $permissionCommentId) {
    throw "Permission comment response did not include comment.id"
  }
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/comments/$permissionCommentId" -Method Delete -Headers $followeeHeaders -TimeoutSec 10
  Invoke-Api -Uri "$baseUrl/api/v1/comments/$permissionCommentId" -Method Delete -Headers $headers -TimeoutSec 10 | Out-Null
  $permissionHiddenComments = Invoke-Api -Uri "$baseUrl/api/v1/admin/comments?entity_type=topic&entity_id=$topicId&status=0&page=1&page_size=20" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $permissionCommentHidden = $false
  foreach ($item in @($permissionHiddenComments.items)) {
    if ([string]$item.id -eq [string]$permissionCommentId -and [int64]$item.status -eq 0) {
      $permissionCommentHidden = $true
    }
  }
  if (-not $permissionCommentHidden) {
    throw "Owner comment delete did not hide permission comment"
  }
  $topicLike = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/like" -Method Post -Headers $headers -TimeoutSec 10
  if ([int64]$topicLike.count -lt 1) {
    throw "Topic like did not increment count"
  }
  $topicFavorite = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/favorite" -Method Post -Headers $headers -TimeoutSec 10
  if ([int64]$topicFavorite.count -lt 1) {
    throw "Topic favorite did not increment count"
  }
  $topicReactionCounts = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/reactions" -Method Get -TimeoutSec 10
  if ([int64]$topicReactionCounts.like_count -lt 1 -or [int64]$topicReactionCounts.favorite_count -lt 1) {
    throw "Topic reaction counts did not include like and favorite"
  }

  $topicFeed = $null
  $topicFeedItem = $null
  $topicFeedProjected = $false
  $topicFeedCountersProjected = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $topicFeed = Invoke-Api -Uri "$baseUrl/api/v1/feed?limit=20&offset=0" -Method Get -TimeoutSec 10
      foreach ($item in @($topicFeed.items)) {
        if ([string]$item.id -eq [string]$topicId -and $item.entity_type -eq "topic") {
          $topicFeedItem = $item
        }
      }
      if ($topicFeedItem) {
        $topicFeedProjected = $true
      }
      if ($topicFeedItem -and [int64]$topicFeedItem.comment_count -ge 1 -and [int64]$topicFeedItem.like_count -ge 1 -and [int64]$topicFeedItem.favorite_count -ge 1) {
        $topicFeedCountersProjected = $true
        break
      }
    } catch {
    }
  }
  if (-not $topicFeedProjected) {
    throw "Created topic was not projected into feed-service within timeout"
  }
  if (-not $topicFeedCountersProjected) {
    throw "Topic feed item counters were not projected from comment/reaction events"
  }

  $topicSearchIndexed = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $topicSearchUrl = "$baseUrl/api/v1/search/topics?q=$([uri]::EscapeDataString($topicTitle))"
      $topicSearch = Invoke-Api -Uri $topicSearchUrl -Method Get -TimeoutSec 10
      foreach ($item in @($topicSearch.items)) {
        if ([string]$item.topic.id -eq [string]$topicId) {
          $topicSearchIndexed = $true
        }
      }
      if ($topicSearchIndexed) {
        break
      }
    } catch {
    }
  }
  if (-not $topicSearchIndexed) {
    throw "Created topic was not indexed by search-service within timeout"
  }

  $adminTopics = Invoke-Api -Uri "$baseUrl/api/v1/admin/topics?status=2&type=topic&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $adminTopicListed = $false
  foreach ($item in @($adminTopics.items)) {
    if ([string]$item.id -eq [string]$topicId) {
      $adminTopicListed = $true
    }
  }
  if (-not $adminTopicListed) {
    throw "Admin topic list did not include created topic"
  }
  $title = "Frontend smoke post $stamp"
  $articleBody = @{
    slug = "frontend-smoke-$stamp"
    title = $title
    summary = "Gateway smoke"
    body = "This post was created through api-gateway during local smoke testing."
    cover_url = ""
    tags = @("smoke", "frontend")
    publish = $true
  } | ConvertTo-Json
  $article = Invoke-Api -Uri "$baseUrl/api/v1/articles" -Method Post -Headers $followeeHeaders -ContentType "application/json" -Body $articleBody -TimeoutSec 10
  $articleId = $article.article.id
  if (-not $articleId) {
    throw "Create article response did not include article.id"
  }
  $articleDetail = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId" -Method Get -TimeoutSec 10
  if ($articleDetail.article.title -ne $title) {
    throw "Article detail response did not match created article"
  }

  $moderationTitle = "Admin moderation smoke $stamp"
  $moderationBody = @{
    slug = "admin-moderation-smoke-$stamp"
    title = $moderationTitle
    summary = "Admin moderation smoke"
    body = "This post is used to validate admin article moderation."
    cover_url = ""
    tags = @("moderation", "smoke")
    publish = $true
  } | ConvertTo-Json
  $moderationArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles" -Method Post -Headers $followeeHeaders -ContentType "application/json" -Body $moderationBody -TimeoutSec 10
  $moderationArticleId = $moderationArticle.article.id
  if (-not $moderationArticleId) {
    throw "Moderation article response did not include article.id"
  }
  $adminArticles = Invoke-Api -Uri "$baseUrl/api/v1/admin/articles?status=2&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $moderationListed = $false
  foreach ($item in @($adminArticles.items)) {
    if ([string]$item.id -eq [string]$moderationArticleId) {
      $moderationListed = $true
    }
  }
  if (-not $moderationListed) {
    throw "Admin article list did not include moderation article"
  }
  $hiddenArticle = Invoke-Api -Uri "$baseUrl/api/v1/admin/articles/$moderationArticleId/hide" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$hiddenArticle.article.status -ne 3) {
    throw "Admin hide article did not mark article as hidden"
  }
  $archivedArticle = Invoke-Api -Uri "$baseUrl/api/v1/admin/articles/$moderationArticleId/archive" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$archivedArticle.article.status -ne 4) {
    throw "Admin archive article did not mark article as archived"
  }

  $tags = Invoke-Api -Uri "$baseUrl/api/v1/tags?limit=20" -Method Get -TimeoutSec 10
  $tagNames = @($tags.items | ForEach-Object { $_.name })
  if ($tagNames -notcontains "smoke" -or $tagNames -notcontains "frontend") {
    throw "Tag list did not include tags from published article"
  }
  $tagSuggestBody = @{
    query = "front"
    limit = 10
  } | ConvertTo-Json
  $tagSuggest = Invoke-Api -Uri "$baseUrl/api/v1/tags/autocomplete" -Method Post -ContentType "application/json" -Body $tagSuggestBody -TimeoutSec 10
  $suggestNames = @($tagSuggest.items | ForEach-Object { $_.name })
  if ($suggestNames -notcontains "frontend") {
    throw "Tag autocomplete did not include frontend"
  }

  $reportBody = @{
    reason = "content_violation"
    description = "Smoke report"
  } | ConvertTo-Json
  $report = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/report" -Method Post -Headers $headers -ContentType "application/json" -Body $reportBody -TimeoutSec 10
  if (-not $report.report.id) {
    throw "Report response did not include report.id"
  }
  Assert-ApiStatus 401 -Uri "$baseUrl/api/v1/admin/reports?status=1&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $reports = Invoke-Api -Uri "$baseUrl/api/v1/admin/reports?status=1&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $reportedItem = $null
  foreach ($item in @($reports.items)) {
    if ([string]$item.id -eq [string]$report.report.id) {
      $reportedItem = $item
    }
  }
  if (-not $reportedItem) {
    throw "Admin report list did not include submitted report"
  }
  $auditBody = @{
    status = 2
  } | ConvertTo-Json
  $auditedReport = Invoke-Api -Uri "$baseUrl/api/v1/admin/reports/$($report.report.id)/audit" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $auditBody -TimeoutSec 10
  if ([int64]$auditedReport.report.status -ne 2) {
    throw "Audit report response did not mark report as resolved"
  }

  $commentBody = @{
    content = "Smoke comment"
    parent_id = 0
  } | ConvertTo-Json
  $mutedArticleBody = @{
    slug = "muted-update-$stamp"
    title = "Muted update article $stamp"
    summary = "Mute update guard"
    body = "This article verifies that muted authors cannot update published content."
    cover_url = ""
    tags = @("mute", "update")
    publish = $true
  } | ConvertTo-Json
  $mutedArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles" -Method Post -Headers $headers -ContentType "application/json" -Body $mutedArticleBody -TimeoutSec 10
  $mutedArticleId = $mutedArticle.article.id
  if (-not $mutedArticleId -or [int64]$mutedArticle.article.status -ne 2) {
    throw "Muted update article create response did not include a published article"
  }
  $mutedTopicUpdateBody = @{
    title = "Muted topic update $stamp"
    body = "This update must be rejected while the author is muted."
    tags = @("mute", "update")
    category_id = $categoryId
    bounty_score = 0
  } | ConvertTo-Json
  $mutedArticleUpdateBody = @{
    title = "Muted article update $stamp"
    summary = "This update must be rejected while the author is muted."
    body = "This update must be rejected while the author is muted."
    cover_url = ""
    tags = @("mute", "update")
  } | ConvertTo-Json
  $mutedUser = Invoke-Api -Uri "$baseUrl/api/v1/admin/users/$($me.user.id)/mute" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$mutedUser.user.status -ne 2) {
    throw "Mute user response did not mark user as muted"
  }
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics/$topicId" -Method Put -Headers $headers -ContentType "application/json" -Body $mutedTopicUpdateBody -TimeoutSec 10
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/articles/$mutedArticleId" -Method Put -Headers $headers -ContentType "application/json" -Body $mutedArticleUpdateBody -TimeoutSec 10
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics/$topicId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $topicCommentBody -TimeoutSec 10
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/articles/$articleId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $commentBody -TimeoutSec 10
  $unmutedUser = Invoke-Api -Uri "$baseUrl/api/v1/admin/users/$($me.user.id)/unmute" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$unmutedUser.user.status -ne 1) {
    throw "Unmute user response did not mark user as active"
  }
  $moderationCommentBody = @{
    content = "Moderation smoke comment"
    parent_id = 0
  } | ConvertTo-Json
  $moderationComment = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $moderationCommentBody -TimeoutSec 10
  $moderationCommentId = $moderationComment.comment.id
  if (-not $moderationCommentId) {
    throw "Moderation comment response did not include comment.id"
  }
  $adminComments = Invoke-Api -Uri "$baseUrl/api/v1/admin/comments?entity_type=article&entity_id=$articleId&status=-1&page=1&page_size=20" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $moderationCommentListed = $false
  foreach ($item in @($adminComments.items)) {
    if ([string]$item.id -eq [string]$moderationCommentId) {
      $moderationCommentListed = $true
    }
  }
  if (-not $moderationCommentListed) {
    throw "Admin comment list did not include moderation comment"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/comments/$moderationCommentId/hide" -Method Post -Headers $adminHeaders -TimeoutSec 10 | Out-Null
  $hiddenComments = Invoke-Api -Uri "$baseUrl/api/v1/admin/comments?entity_type=article&entity_id=$articleId&status=0&page=1&page_size=20" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $moderationCommentHidden = $false
  foreach ($item in @($hiddenComments.items)) {
    if ([string]$item.id -eq [string]$moderationCommentId -and [int64]$item.status -eq 0) {
      $moderationCommentHidden = $true
    }
  }
  if (-not $moderationCommentHidden) {
    throw "Admin hide comment did not mark moderation comment as hidden"
  }
  $comment = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $commentBody -TimeoutSec 10
  $like = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/like" -Method Post -Headers $headers -TimeoutSec 10
  $favorite = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/favorite" -Method Post -Headers $headers -TimeoutSec 10
  $likes = Invoke-Api -Uri "$baseUrl/api/v1/users/current/likes?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $likeTopicListed = $false
  $likeArticleListed = $false
  foreach ($item in @($likes.items)) {
    if ($item.entity.entity_type -eq "topic" -and [string]$item.entity.entity_id -eq [string]$topicId) {
      $likeTopicListed = $true
    }
    if ($item.entity.entity_type -eq "article" -and [string]$item.entity.entity_id -eq [string]$articleId) {
      $likeArticleListed = $true
    }
  }
  if (-not $likeTopicListed -or -not $likeArticleListed) {
    throw "Current user likes did not include liked topic and article"
  }
  $favorites = Invoke-Api -Uri "$baseUrl/api/v1/users/current/favorites?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $favoriteTopicListed = $false
  $favoriteArticleListed = $false
  foreach ($item in @($favorites.items)) {
    if ($item.entity.entity_type -eq "topic" -and [string]$item.entity.entity_id -eq [string]$topicId) {
      $favoriteTopicListed = $true
    }
    if ($item.entity.entity_type -eq "article" -and [string]$item.entity.entity_id -eq [string]$articleId) {
      $favoriteArticleListed = $true
    }
  }
  if (-not $favoriteTopicListed -or -not $favoriteArticleListed) {
    throw "Current user favorites did not include favorited topic and article"
  }
  $reactionCacheRebuild = Invoke-ReactionCacheRebuild
  if (-not $reactionCacheRebuild.verified -or [int64]$reactionCacheRebuild.likes_loaded -lt 2 -or [int64]$reactionCacheRebuild.favorites_loaded -lt 2) {
    throw "Reaction cache rebuild did not verify loaded like and favorite caches"
  }
  $postRebuildTopicReactions = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/reactions" -Method Get -TimeoutSec 10
  $postRebuildArticleReactions = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/reactions" -Method Get -TimeoutSec 10
  if ([int64]$postRebuildTopicReactions.like_count -lt 1 -or [int64]$postRebuildTopicReactions.favorite_count -lt 1) {
    throw "Topic reaction counts changed after cache rebuild"
  }
  if ([int64]$postRebuildArticleReactions.like_count -lt 1 -or [int64]$postRebuildArticleReactions.favorite_count -lt 1) {
    throw "Article reaction counts changed after cache rebuild"
  }
  $hiddenTopic = Invoke-Api -Uri "$baseUrl/api/v1/admin/topics/$topicId/hide" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$hiddenTopic.topic.status -ne 3) {
    throw "Admin hide topic did not mark topic as hidden"
  }
  $archivedTopic = Invoke-Api -Uri "$baseUrl/api/v1/admin/topics/$topicId/archive" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$archivedTopic.topic.status -ne 4) {
    throw "Admin archive topic did not mark topic as archived"
  }
  $comments = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/comments?page=1&page_size=10" -Method Get -TimeoutSec 10

  $feed = $null
  $feedItem = $null
  $feedCountersProjected = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $feed = Invoke-Api -Uri "$baseUrl/api/v1/feed?limit=10&offset=0" -Method Get -TimeoutSec 10
      foreach ($item in @($feed.items)) {
        if ([string]$item.id -eq [string]$articleId) {
          $feedItem = $item
        }
      }
      if ($feedItem -and [int64]$feedItem.comment_count -ge 1 -and [int64]$feedItem.like_count -ge 1 -and [int64]$feedItem.favorite_count -ge 1) {
        $feedCountersProjected = $true
        break
      }
    } catch {
    }
  }
  if (-not $feedItem) {
    throw "Created article was not projected into feed-service within timeout"
  }
  if (-not $feedCountersProjected) {
    throw "Feed item counters were not projected from comment/reaction events"
  }

  $followingFeed = Invoke-Api -Uri "$baseUrl/api/v1/feed?sort=follow&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $followingFeedArticleListed = $false
  $followingFeedOwnTopicExcluded = $true
  foreach ($item in @($followingFeed.items)) {
    if ([string]$item.id -eq [string]$articleId -and $item.entity_type -eq "article") {
      $followingFeedArticleListed = $true
    }
    if ([string]$item.id -eq [string]$topicId -and $item.entity_type -eq "topic") {
      $followingFeedOwnTopicExcluded = $false
    }
  }
  if (-not $followingFeedArticleListed) {
    throw "Following feed did not include article from followed author"
  }
  if (-not $followingFeedOwnTopicExcluded) {
    throw "Following feed included topic from an unfollowed author"
  }

  $hotFeed = Invoke-Api -Uri "$baseUrl/api/v1/feed?sort=hot&limit=10&offset=0" -Method Get -TimeoutSec 10
  $foundHot = $false
  foreach ($item in @($hotFeed.items)) {
    if ([string]$item.id -eq [string]$articleId) {
      $foundHot = $true
    }
  }
  if (-not $foundHot) {
    throw "Created article was not available in hot feed"
  }

  $searchIndexed = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $searchUrl = "$baseUrl/api/v1/search/articles?q=$([uri]::EscapeDataString($title))"
      $search = Invoke-Api -Uri $searchUrl -Method Get -TimeoutSec 10
      foreach ($item in @($search.items)) {
        if ([string]$item.article.id -eq [string]$articleId) {
          $searchIndexed = $true
        }
      }
      if ($searchIndexed) {
        break
      }
    } catch {
    }
  }

  if (-not $searchIndexed) {
    throw "Created article was not indexed by search-service within timeout"
  }

  $notifications = $null
  $notificationTypes = @()
  $notificationsReady = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $notifications = Invoke-Api -Uri "$baseUrl/api/v1/notifications?limit=20&offset=0" -Method Get -Headers $followeeHeaders -TimeoutSec 10
      $notificationTypes = @($notifications.items | ForEach-Object { $_.type })
      if ($notificationTypes -contains "comment" -and $notificationTypes -contains "like" -and $notificationTypes -contains "favorite") {
        $notificationsReady = $true
        break
      }
    } catch {
    }
  }
  if (-not $notificationsReady) {
    throw "Author notifications were not projected from comment/reaction events"
  }
  $unread = Invoke-Api -Uri "$baseUrl/api/v1/notifications/unread-count" -Method Get -Headers $followeeHeaders -TimeoutSec 10
  if ([int64]$unread.count -lt 3) {
    throw "Unread notification count was lower than expected"
  }
  $firstNotification = @($notifications.items)[0]
  Invoke-Api -Uri "$baseUrl/api/v1/notifications/$($firstNotification.id)/read" -Method Post -Headers $followeeHeaders -TimeoutSec 10 | Out-Null
  $afterRead = Invoke-Api -Uri "$baseUrl/api/v1/notifications/unread-count" -Method Get -Headers $followeeHeaders -TimeoutSec 10
  if ([int64]$afterRead.count -ge [int64]$unread.count) {
    throw "Unread notification count did not decrease after mark read"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/notifications/read-all" -Method Post -Headers $followeeHeaders -TimeoutSec 10 | Out-Null
  $afterReadAll = Invoke-Api -Uri "$baseUrl/api/v1/notifications/unread-count" -Method Get -Headers $followeeHeaders -TimeoutSec 10
  $afterReadAllCount = [int64]$afterReadAll.count
  if ($afterReadAllCount -ne 0) {
    throw "Unread notification count was not zero after mark all read"
  }

  $actorCredit = $null
  $authorCredit = $null
  $actorLedger = $null
  $authorLedger = $null
  $creditsReady = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $actorCredit = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
      $authorCredit = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $followeeHeaders -TimeoutSec 10
      if ([int64]$actorCredit.balance.total -ge 27 -and [int64]$authorCredit.balance.total -ge 34) {
        $creditsReady = $true
        break
      }
    } catch {
    }
  }
  if (-not $creditsReady) {
    throw "Credit balances were not projected from user/article/comment/reaction events"
  }
  $actorLedger = Invoke-Api -Uri "$baseUrl/api/v1/credits/ledger?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $authorLedger = Invoke-Api -Uri "$baseUrl/api/v1/credits/ledger?limit=20&offset=0" -Method Get -Headers $followeeHeaders -TimeoutSec 10
  if (@($actorLedger.items).Count -lt 5) {
    throw "Actor credit ledger did not include expected events"
  }
  if (@($authorLedger.items).Count -lt 5) {
    throw "Author credit ledger did not include expected events"
  }

  $mallCreditTopUp = 1000
  $mallCreditBeforeTopUp = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  $mallCreditAdjustBody = @{
    delta = $mallCreditTopUp
    reason = "smoke_mall_topup"
    description = "Smoke mall checkout top-up"
    source_event_id = "smoke-mall-credit-$stamp"
  } | ConvertTo-Json
  $mallCreditAdjustment = Invoke-Api -Uri "$baseUrl/api/v1/admin/credits/users/$($me.user.id)/adjust" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $mallCreditAdjustBody -TimeoutSec 10
  $mallCreditAfterTopUp = [int64]$mallCreditAdjustment.balance.total
  if ($mallCreditAfterTopUp -lt ([int64]$mallCreditBeforeTopUp.balance.total + $mallCreditTopUp)) {
    throw "Admin credit adjustment did not increase user mall checkout balance"
  }

  $mallCategorySlug = "smoke-mall-$stamp"
  $mallCategoryBody = @{
    slug = $mallCategorySlug
    name = "Smoke Mall $stamp"
    description = "Smoke mall category"
    status = 2
    sort = 900
  } | ConvertTo-Json
  $createdMallCategory = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/categories" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $mallCategoryBody -TimeoutSec 10
  if (-not $createdMallCategory.category.id -or $createdMallCategory.category.slug -ne $mallCategorySlug) {
    throw "Admin mall category create did not return expected category"
  }
  $updatedMallCategoryBody = @{
    slug = $mallCategorySlug
    name = "Smoke Mall Updated $stamp"
    description = "Smoke mall category updated"
    status = 2
    sort = 901
  } | ConvertTo-Json
  $updatedMallCategory = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/categories/$($createdMallCategory.category.id)" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updatedMallCategoryBody -TimeoutSec 10
  if ($updatedMallCategory.category.name -ne "Smoke Mall Updated $stamp") {
    throw "Admin mall category update did not return updated name"
  }
  $publicMallCategories = Invoke-Api -Uri "$baseUrl/api/v1/mall/categories?limit=50&offset=0" -Method Get -TimeoutSec 10
  $publicMallCategoryListed = $false
  foreach ($item in @($publicMallCategories.items)) {
    if ($item.slug -eq $mallCategorySlug) {
      $publicMallCategoryListed = $true
    }
  }
  if (-not $publicMallCategoryListed) {
    throw "Public mall category list did not include smoke category"
  }

  $mallProductSku = "SMOKE-$stamp"
  $mallProductPrice = 20
  $mallProductStock = 5
  $mallProductBody = @{
    sku = $mallProductSku
    title = "Smoke Product $stamp"
    description = "Smoke mall product"
    category = $mallCategorySlug
    cover_url = ""
    price_credits = $mallProductPrice
    stock = $mallProductStock
    status = 2
    sort = 100
  } | ConvertTo-Json
  $createdMallProduct = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/products" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $mallProductBody -TimeoutSec 10
  $mallProductId = $createdMallProduct.product.id
  if (-not $mallProductId -or $createdMallProduct.product.sku -ne $mallProductSku) {
    throw "Admin mall product create did not return expected product"
  }
  $updatedMallProductBody = @{
    sku = $mallProductSku
    title = "Smoke Product Updated $stamp"
    description = "Smoke mall product updated"
    category = $mallCategorySlug
    cover_url = ""
    price_credits = $mallProductPrice
    stock = $mallProductStock
    status = 2
    sort = 101
  } | ConvertTo-Json
  $updatedMallProduct = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/products/$mallProductId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updatedMallProductBody -TimeoutSec 10
  if ($updatedMallProduct.product.title -ne "Smoke Product Updated $stamp") {
    throw "Admin mall product update did not return updated title"
  }
  $publicMallProducts = Invoke-Api -Uri "$baseUrl/api/v1/mall/products?category=$mallCategorySlug&limit=20&offset=0" -Method Get -TimeoutSec 10
  $publicMallProductListed = $false
  foreach ($item in @($publicMallProducts.items)) {
    if ([string]$item.id -eq [string]$mallProductId) {
      $publicMallProductListed = $true
    }
  }
  if (-not $publicMallProductListed) {
    throw "Public mall product list did not include smoke product"
  }

  $mallDigitalProductSku = "SMOKE-BADGE-$stamp"
  $mallDigitalGrantKey = "badge-smoke-$stamp"
  $mallDigitalProductPrice = 10
  $mallDigitalProductBody = @{
    sku = $mallDigitalProductSku
    title = "Smoke Badge Entitlement $stamp"
    description = "Smoke digital badge entitlement"
    category = "digital"
    cover_url = ""
    grant_type = "badge"
    grant_key = $mallDigitalGrantKey
    price_credits = $mallDigitalProductPrice
    stock = 3
    status = 2
    sort = 102
  } | ConvertTo-Json
  $createdMallDigitalProduct = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/products" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $mallDigitalProductBody -TimeoutSec 10
  $mallDigitalProductId = $createdMallDigitalProduct.product.id
  if (-not $mallDigitalProductId -or $createdMallDigitalProduct.product.sku -ne $mallDigitalProductSku -or $createdMallDigitalProduct.product.grant_type -ne "badge" -or $createdMallDigitalProduct.product.grant_key -ne $mallDigitalGrantKey) {
    throw "Admin mall digital product create did not return expected grant fields"
  }
  $publicMallDigitalProducts = Invoke-Api -Uri "$baseUrl/api/v1/mall/products?category=digital&limit=50&offset=0" -Method Get -TimeoutSec 10
  $publicMallDigitalProductListed = $false
  $mallMembershipProductListed = $false
  $mallMembershipProductId = $null
  $mallMembershipProductPrice = 0
  $mallThemeProductListed = $false
  $mallThemeProductId = $null
  $mallThemeProductPrice = 0
  $mallThemeDuplicateOrderRejected = $false
  $mallThemePendingDuplicateOrderRejected = $false
  $mallBadgeDuplicateOrderRejected = $false
  $mallBadgePendingDuplicateOrderRejected = $false
  foreach ($item in @($publicMallDigitalProducts.items)) {
    if ([string]$item.id -eq [string]$mallDigitalProductId -and $item.grant_type -eq "badge" -and $item.grant_key -eq $mallDigitalGrantKey) {
      $publicMallDigitalProductListed = $true
    }
    if ($item.sku -eq "vip-month" -and $item.grant_type -eq "membership" -and $item.grant_key -eq "vip-month") {
      $mallMembershipProductListed = $true
      $mallMembershipProductId = $item.id
      $mallMembershipProductPrice = [int64]$item.price_credits
    }
    if ($item.sku -eq "theme-pro" -and $item.grant_type -eq "theme" -and $item.grant_key -eq "theme-pro") {
      $mallThemeProductListed = $true
      $mallThemeProductId = $item.id
      $mallThemeProductPrice = [int64]$item.price_credits
    }
  }
  if (-not $publicMallDigitalProductListed) {
    throw "Public mall product list did not include smoke digital product grant fields"
  }
  if (-not $mallMembershipProductListed -or -not $mallMembershipProductId -or [int64]$mallMembershipProductPrice -le 0) {
    throw "Public mall product list did not include seeded membership product"
  }
  if (-not $mallThemeProductListed -or -not $mallThemeProductId -or [int64]$mallThemeProductPrice -le 0) {
    throw "Public mall product list did not include seeded theme product"
  }

  $mallCouponCode = "SMOKE$stamp"
  $mallCouponDiscount = 5
  $mallCouponBody = @{
    code = $mallCouponCode
    name = "Smoke Coupon $stamp"
    description = "Smoke mall coupon"
    discount_credits = $mallCouponDiscount
    min_order_credits = $mallProductPrice
    total_quota = 10
    per_user_limit = 1
    status = 2
    starts_at = 0
    ends_at = 0
  } | ConvertTo-Json
  $createdMallCoupon = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/coupons" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $mallCouponBody -TimeoutSec 10
  $mallCouponId = $createdMallCoupon.coupon.id
  if (-not $mallCouponId -or $createdMallCoupon.coupon.code -ne $mallCouponCode) {
    throw "Admin mall coupon create did not return expected coupon"
  }
  $updatedMallCouponBody = @{
    code = $mallCouponCode
    name = "Smoke Coupon Updated $stamp"
    description = "Smoke mall coupon updated"
    discount_credits = $mallCouponDiscount
    min_order_credits = $mallProductPrice
    total_quota = 10
    per_user_limit = 1
    status = 2
    starts_at = 0
    ends_at = 0
  } | ConvertTo-Json
  $updatedMallCoupon = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/coupons/$mallCouponId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updatedMallCouponBody -TimeoutSec 10
  if ($updatedMallCoupon.coupon.name -ne "Smoke Coupon Updated $stamp") {
    throw "Admin mall coupon update did not return updated name"
  }
  $publicMallCouponListed = $false
  $publicMallCouponOffset = 0
  $publicMallCouponTotal = 0
  do {
    $publicMallCoupons = Invoke-Api -Uri "$baseUrl/api/v1/mall/coupons?limit=100&offset=$publicMallCouponOffset" -Method Get -TimeoutSec 10
    $publicMallCouponItems = @($publicMallCoupons.items)
    foreach ($item in $publicMallCouponItems) {
      if ($item.code -eq $mallCouponCode) {
        $publicMallCouponListed = $true
        break
      }
    }
    $publicMallCouponTotal = [int]$publicMallCoupons.total
    $publicMallCouponOffset += $publicMallCouponItems.Count
  } while (-not $publicMallCouponListed -and $publicMallCouponItems.Count -gt 0 -and $publicMallCouponOffset -lt $publicMallCouponTotal)
  if (-not $publicMallCouponListed) {
    throw "Public mall coupon list did not include smoke coupon"
  }

  $claimedMallCoupon = Invoke-Api -Uri "$baseUrl/api/v1/mall/coupons/$mallCouponId/claim" -Method Post -Headers $headers -TimeoutSec 10
  if ([string]$claimedMallCoupon.usage.coupon_id -ne [string]$mallCouponId -or [string]$claimedMallCoupon.usage.user_id -ne [string]$me.user.id -or [int64]$claimedMallCoupon.usage.status -ne 4 -or $claimedMallCoupon.duplicate) {
    throw "Mall coupon claim did not return expected claimed usage"
  }
  $myClaimedMallCoupons = Invoke-Api -Uri "$baseUrl/api/v1/mall/coupons/mine?status=4&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $claimedMallCouponListed = $false
  foreach ($item in @($myClaimedMallCoupons.items)) {
    if ([string]$item.id -eq [string]$claimedMallCoupon.usage.id -and [string]$item.coupon_id -eq [string]$mallCouponId -and [string]$item.user_id -eq [string]$me.user.id -and [int64]$item.status -eq 4) {
      $claimedMallCouponListed = $true
    }
  }
  if (-not $claimedMallCouponListed) {
    throw "My mall coupons did not include claimed smoke coupon"
  }
  $mallCouponTermsLockBody = @{
    code = $mallCouponCode
    name = "Smoke Coupon Updated $stamp"
    description = "Smoke mall coupon updated"
    discount_credits = ($mallCouponDiscount + 1)
    min_order_credits = $mallProductPrice
    total_quota = 10
    per_user_limit = 1
    status = 2
    starts_at = 0
    ends_at = 0
  } | ConvertTo-Json
  Assert-ApiStatusMessage 412 "coupon terms cannot be changed after coupon usages" -Uri "$baseUrl/api/v1/admin/mall/coupons/$mallCouponId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $mallCouponTermsLockBody -TimeoutSec 10
  $adminMallCouponAfterTermsLock = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/coupons?keyword=$mallCouponCode&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $mallCouponTermsLocked = $false
  foreach ($item in @($adminMallCouponAfterTermsLock.items)) {
    if ([string]$item.id -eq [string]$mallCouponId -and [int64]$item.discount_credits -eq $mallCouponDiscount) {
      $mallCouponTermsLocked = $true
    }
  }
  if (-not $mallCouponTermsLocked) {
    throw "Mall coupon terms lock did not preserve original discount"
  }

  $mallFavoriteBefore = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/favorite" -Method Get -Headers $headers -TimeoutSec 10
  if ($mallFavoriteBefore.favorited) {
    throw "Smoke mall product was already favorited before favorite action"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/favorite" -Method Post -Headers $headers -TimeoutSec 10 | Out-Null
  $mallFavoriteAfter = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/favorite" -Method Get -Headers $headers -TimeoutSec 10
  if (-not $mallFavoriteAfter.favorited) {
    throw "Mall product favorite state was not true after favorite"
  }
  $mallFavorites = Invoke-Api -Uri "$baseUrl/api/v1/mall/favorites?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $mallFavoriteListed = $false
  foreach ($item in @($mallFavorites.items)) {
    if ([string]$item.product.id -eq [string]$mallProductId) {
      $mallFavoriteListed = $true
    }
  }
  if (-not $mallFavoriteListed) {
    throw "Mall favorites list did not include smoke product"
  }

  $mallAddressBody = @{
    receiver = "Smoke User"
    phone = "13800000000"
    province = "Smoke Province"
    city = "Smoke City"
    district = "Smoke District"
    detail = "Smoke Street $stamp"
    postal_code = "100000"
    is_default = $true
  } | ConvertTo-Json
  $createdMallAddress = Invoke-Api -Uri "$baseUrl/api/v1/mall/addresses" -Method Post -Headers $headers -ContentType "application/json" -Body $mallAddressBody -TimeoutSec 10
  $mallAddressId = $createdMallAddress.address.id
  if (-not $mallAddressId -or $createdMallAddress.address.receiver -ne "Smoke User") {
    throw "Mall address create did not return expected address"
  }
  $defaultMallAddress = Invoke-Api -Uri "$baseUrl/api/v1/mall/addresses/$mallAddressId/default" -Method Post -Headers $headers -TimeoutSec 10
  if (-not $defaultMallAddress.address.is_default) {
    throw "Mall set default address did not return default address"
  }
  $mallAddresses = Invoke-Api -Uri "$baseUrl/api/v1/mall/addresses?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $mallAddressListed = $false
  foreach ($item in @($mallAddresses.items)) {
    if ([string]$item.id -eq [string]$mallAddressId) {
      $mallAddressListed = $true
    }
  }
  if (-not $mallAddressListed) {
    throw "Mall address list did not include smoke address"
  }

  $mallCartItemBody = @{ quantity = 1 } | ConvertTo-Json
  $mallCartAfterAdd = Invoke-Api -Uri "$baseUrl/api/v1/mall/cart/items/$mallProductId" -Method Put -Headers $headers -ContentType "application/json" -Body $mallCartItemBody -TimeoutSec 10
  $mallCartItemListed = $false
  foreach ($item in @($mallCartAfterAdd.items)) {
    if ([string]$item.product.id -eq [string]$mallProductId -and [int64]$item.quantity -eq 1) {
      $mallCartItemListed = $true
    }
  }
  if (-not $mallCartItemListed) {
    throw "Mall cart did not include smoke product after set item"
  }

  $mallCheckoutBody = @{
    idempotency_key = "smoke-mall-checkout-$stamp"
    coupon_code = $mallCouponCode
    receiver = "Smoke User"
    phone = "13800000000"
    address = "Smoke Province Smoke City Smoke District Smoke Street $stamp"
  } | ConvertTo-Json
  $mallCheckout = Invoke-Api -Uri "$baseUrl/api/v1/mall/cart/checkout" -Method Post -Headers $headers -ContentType "application/json" -Body $mallCheckoutBody -TimeoutSec 10
  $mallOrder = $mallCheckout.order
  $mallOrderId = $mallOrder.id
  $mallOrderTotal = [int64]$mallOrder.total_credits
  if (-not $mallOrderId -or [int64]$mallOrder.status -ne 1 -or $mallOrderTotal -ne ($mallProductPrice - $mallCouponDiscount)) {
    throw "Mall cart checkout did not create expected pending payment order"
  }
  $mallCartAfterCheckout = Invoke-Api -Uri "$baseUrl/api/v1/mall/cart" -Method Get -Headers $headers -TimeoutSec 10
  $mallCartAfterCheckoutItems = @()
  if ($null -ne $mallCartAfterCheckout.items) {
    $mallCartAfterCheckoutItems = @($mallCartAfterCheckout.items)
  }
  if ($mallCartAfterCheckoutItems.Count -ne 0) {
    throw "Mall cart was not cleared after checkout"
  }

  $mallCreditBeforePay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  $mallPayBody = @{
    payment_method = "credits"
    idempotency_key = "smoke-mall-pay-$stamp"
  } | ConvertTo-Json
  $mallPaid = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$mallOrderId/pay" -Method Post -Headers $headers -ContentType "application/json" -Body $mallPayBody -TimeoutSec 10
  if ([int64]$mallPaid.order.status -ne 3) {
    throw "Mall pay did not move order to paid"
  }
  $mallProductAfterPaidOrder = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId" -Method Get -TimeoutSec 10
  $mallFulfillmentLockBody = @{
    sku = $mallProductSku
    title = "Smoke Product Updated $stamp"
    description = "Smoke mall product updated"
    category = "digital"
    cover_url = ""
    price_credits = $mallProductPrice
    stock = [int64]$mallProductAfterPaidOrder.product.stock
    status = 2
    sort = 101
  } | ConvertTo-Json
  Assert-ApiStatusMessage 412 "product fulfillment cannot be changed after paid orders" -Uri "$baseUrl/api/v1/admin/mall/products/$mallProductId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $mallFulfillmentLockBody -TimeoutSec 10
  $mallProductAfterFulfillmentLock = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId" -Method Get -TimeoutSec 10
  if ($mallProductAfterFulfillmentLock.product.category -ne $mallCategorySlug) {
    throw "Sold mall product fulfillment lock did not preserve original category"
  }
  $mallProductFulfillmentLocked = $true
  $mallCreditAfterPay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallCreditAfterPay.balance.total -ne ([int64]$mallCreditBeforePay.balance.total - $mallOrderTotal)) {
    throw "Mall pay did not debit expected credit amount"
  }
  $mallPaidReplay = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$mallOrderId/pay" -Method Post -Headers $headers -ContentType "application/json" -Body $mallPayBody -TimeoutSec 10
  if ([int64]$mallPaidReplay.order.status -ne 3) {
    throw "Mall payment replay did not preserve paid order state"
  }
  $mallCreditAfterPayReplay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallCreditAfterPayReplay.balance.total -ne [int64]$mallCreditAfterPay.balance.total) {
    throw "Mall payment replay charged credits more than once"
  }
  $mallOrderPayments = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders/$mallOrderId/payments" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($mallOrderPayments.items).Count -ne 1) {
    throw "Mall payment replay created duplicate payment attempts"
  }
  $firstMallPayment = @($mallOrderPayments.items)[0]
  if ([int64]$firstMallPayment.status -ne 2) {
    throw "Admin mall order payments did not include succeeded payment"
  }
  $mallPaymentReplayIdempotent = $true
  $mallCouponUsages = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/coupons/$mallCouponId/usages?status=2&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $mallCouponUsageListed = $false
  foreach ($item in @($mallCouponUsages.items)) {
    if ([string]$item.order_id -eq [string]$mallOrderId -and [int64]$item.discount_credits -eq $mallCouponDiscount) {
      $mallCouponUsageListed = $true
    }
  }
  if (-not $mallCouponUsageListed) {
    throw "Admin mall coupon usages did not include paid smoke order"
  }

  $adminMallOrders = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders?keyword=$($mallOrder.order_no)&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $adminMallOrderListed = $false
  foreach ($item in @($adminMallOrders.items)) {
    if ([string]$item.id -eq [string]$mallOrderId) {
      $adminMallOrderListed = $true
    }
  }
  if (-not $adminMallOrderListed) {
    throw "Admin mall order list did not include smoke order"
  }
  $shipMallOrderBody = @{
    status = 5
    shipping_carrier = "Smoke Express"
    tracking_no = "SMOKE$stamp"
    note = "Smoke order shipped"
  } | ConvertTo-Json
  $mallShipped = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders/$mallOrderId/status" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $shipMallOrderBody -TimeoutSec 10
  if ([int64]$mallShipped.order.status -ne 5) {
    throw "Admin mall ship did not move order to shipped"
  }
  $completeMallOrderBody = @{
    status = 6
    note = "Smoke order completed"
  } | ConvertTo-Json
  $mallCompleted = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders/$mallOrderId/status" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $completeMallOrderBody -TimeoutSec 10
  if ([int64]$mallCompleted.order.status -ne 6) {
    throw "Admin mall complete did not move order to completed"
  }
  $mallOrderLogs = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$mallOrderId/logs" -Method Get -Headers $headers -TimeoutSec 10
  if (@($mallOrderLogs.items).Count -lt 4) {
    throw "Mall order logs did not include expected lifecycle entries"
  }

  $mallReviewableOrders = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviewable-orders?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $mallReviewableOrderListed = $false
  foreach ($item in @($mallReviewableOrders.items)) {
    if ([string]$item.id -eq [string]$mallOrderId) {
      $mallReviewableOrderListed = $true
    }
  }
  if (-not $mallReviewableOrderListed) {
    throw "Mall reviewable orders did not include completed smoke order"
  }
  $mallReviewBody = @{
    order_id = $mallOrderId
    rating = 5
    content = "Smoke mall review $stamp"
  } | ConvertTo-Json
  $emailVerificationTopicCreateBody = @{
    slug = "email-verification-topic-$stamp"
    type = "topic"
    title = "Email verification topic $stamp"
    body = "A published topic used to verify the email gate on updates."
    tags = @("email", "verification")
    category_id = $categoryId
    bounty_score = 0
    publish = $true
  } | ConvertTo-Json
  $emailVerificationTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $emailVerificationTopicCreateBody -TimeoutSec 10
  $emailVerificationTopicId = $emailVerificationTopic.topic.id
  if (-not $emailVerificationTopicId -or [int64]$emailVerificationTopic.topic.status -ne 2) {
    throw "Email verification topic did not publish"
  }
  $emailVerificationArticleCreateBody = @{
    slug = "email-verification-article-$stamp"
    title = "Email verification article $stamp"
    summary = "A published article used to verify the email gate on updates."
    body = "Published article body for email verification update checks."
    cover_url = ""
    tags = @("email", "verification")
    publish = $true
  } | ConvertTo-Json
  $emailVerificationArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles" -Method Post -Headers $headers -ContentType "application/json" -Body $emailVerificationArticleCreateBody -TimeoutSec 10
  $emailVerificationArticleId = $emailVerificationArticle.article.id
  if (-not $emailVerificationArticleId -or [int64]$emailVerificationArticle.article.status -ne 2) {
    throw "Email verification article did not publish"
  }
  $emailVerificationSetting = @($adminSettings.items | Where-Object { $_.key -eq "auth.email_verification.required" }) | Select-Object -First 1
  if ($null -eq $emailVerificationSetting) {
    throw "Admin settings did not include auth.email_verification.required"
  }
  $mallReviewEmailVerificationRejected = $false
  $emailVerificationTopicUpdateRejected = $false
  $emailVerificationArticleUpdateRejected = $false
  try {
    $enableEmailVerificationBody = @{
      key = "auth.email_verification.required"
      value = "true"
      group = $emailVerificationSetting.group
      value_type = $emailVerificationSetting.value_type
      description = $emailVerificationSetting.description
      status = $emailVerificationSetting.status
    } | ConvertTo-Json
    $enabledEmailVerification = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/auth.email_verification.required" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $enableEmailVerificationBody -TimeoutSec 10
    if ($enabledEmailVerification.setting.value -ne "true") {
      throw "Admin email verification setting did not enable"
    }
    $emailVerificationTopicUpdateBody = @{
      title = "Blocked email verification topic update $stamp"
      body = "This published topic update must be rejected for an unverified author."
      tags = @("email", "verification")
      category_id = $categoryId
      bounty_score = 0
    } | ConvertTo-Json
    Assert-ApiStatusMessage 403 "email_not_verified" -Uri "$baseUrl/api/v1/topics/$emailVerificationTopicId" -Method Put -Headers $headers -ContentType "application/json" -Body $emailVerificationTopicUpdateBody -TimeoutSec 10
    $emailVerificationTopicUpdateRejected = $true
    $emailVerificationArticleUpdateBody = @{
      title = "Blocked email verification article update $stamp"
      summary = "This published article update must be rejected for an unverified author."
      body = "Updated body that must not be persisted."
      cover_url = ""
      tags = @("email", "verification")
    } | ConvertTo-Json
    Assert-ApiStatusMessage 403 "email_not_verified" -Uri "$baseUrl/api/v1/articles/$emailVerificationArticleId" -Method Put -Headers $headers -ContentType "application/json" -Body $emailVerificationArticleUpdateBody -TimeoutSec 10
    $emailVerificationArticleUpdateRejected = $true
    Assert-ApiStatusMessage 403 "email_not_verified" -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviews" -Method Post -Headers $headers -ContentType "application/json" -Body $mallReviewBody -TimeoutSec 10
    $mallReviewEmailVerificationRejected = $true
  } finally {
    $restoreEmailVerificationBody = @{
      key = "auth.email_verification.required"
      value = $emailVerificationSetting.value
      group = $emailVerificationSetting.group
      value_type = $emailVerificationSetting.value_type
      description = $emailVerificationSetting.description
      status = $emailVerificationSetting.status
    } | ConvertTo-Json
    $restoredEmailVerification = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/auth.email_verification.required" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $restoreEmailVerificationBody -TimeoutSec 10
    if ([string]$restoredEmailVerification.setting.value -ne [string]$emailVerificationSetting.value) {
      throw "Admin email verification setting did not restore"
    }
  }
  $mallReviewMutedUser = Invoke-Api -Uri "$baseUrl/api/v1/admin/users/$($me.user.id)/mute" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$mallReviewMutedUser.user.status -ne 2) {
    throw "Mall review mute did not mark user as muted"
  }
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviews" -Method Post -Headers $headers -ContentType "application/json" -Body $mallReviewBody -TimeoutSec 10
  $mallReviewUnmutedUser = Invoke-Api -Uri "$baseUrl/api/v1/admin/users/$($me.user.id)/unmute" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$mallReviewUnmutedUser.user.status -ne 1) {
    throw "Mall review unmute did not mark user as active"
  }
  $createdMallReview = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviews" -Method Post -Headers $headers -ContentType "application/json" -Body $mallReviewBody -TimeoutSec 10
  $mallReviewId = $createdMallReview.review.id
  if (-not $mallReviewId -or [int64]$createdMallReview.review.status -ne 1) {
    throw "Mall product review create did not return pending review"
  }
  $myPendingMallReviews = Invoke-Api -Uri "$baseUrl/api/v1/mall/reviews?product_id=$mallProductId&status=1&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  if (@($myPendingMallReviews.items | Where-Object { [string]$_.id -eq [string]$mallReviewId }).Count -ne 1) {
    throw "Current user's mall reviews did not include pending smoke review"
  }
  $adminMallReviews = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/reviews?product_id=$mallProductId&status=1&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminMallReviews.items | Where-Object { [string]$_.id -eq [string]$mallReviewId }).Count -ne 1) {
    throw "Admin mall reviews did not include smoke review"
  }
  $hideMallReviewBody = @{ status = 3 } | ConvertTo-Json
  $hiddenMallReview = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/reviews/$mallReviewId/status" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $hideMallReviewBody -TimeoutSec 10
  if ([int64]$hiddenMallReview.review.status -ne 3) {
    throw "Admin mall review hide did not update review status"
  }
  $publishMallReviewBody = @{ status = 2 } | ConvertTo-Json
  $publishedMallReview = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/reviews/$mallReviewId/status" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $publishMallReviewBody -TimeoutSec 10
  if ([int64]$publishedMallReview.review.status -ne 2) {
    throw "Admin mall review publish did not restore review status"
  }
  $publicMallReviews = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviews?limit=20&offset=0" -Method Get -TimeoutSec 10
  if (@($publicMallReviews.items | Where-Object { [string]$_.id -eq [string]$mallReviewId -and [int64]$_.status -eq 2 }).Count -ne 1) {
    throw "Public mall product reviews did not include published smoke review"
  }
  $myPublishedMallReviews = Invoke-Api -Uri "$baseUrl/api/v1/mall/reviews?product_id=$mallProductId&status=2&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  if (@($myPublishedMallReviews.items | Where-Object { [string]$_.id -eq [string]$mallReviewId }).Count -ne 1) {
    throw "Current user's mall reviews did not include published smoke review"
  }

  $mallRefundBody = @{
    reason = "smoke_after_sale"
    note = "Smoke refund $stamp"
  } | ConvertTo-Json
  $mallCreditBeforeRefund = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  $createdMallRefund = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$mallOrderId/refunds" -Method Post -Headers $headers -ContentType "application/json" -Body $mallRefundBody -TimeoutSec 10
  $mallRefundId = $createdMallRefund.refund.id
  if (-not $mallRefundId -or [int64]$createdMallRefund.refund.status -ne 1 -or [int64]$createdMallRefund.refund.amount_credits -ne $mallOrderTotal) {
    throw "Mall refund request did not return expected requested refund"
  }
  $mallRefunds = Invoke-Api -Uri "$baseUrl/api/v1/mall/refunds?status=1&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  if (@($mallRefunds.items | Where-Object { [string]$_.id -eq [string]$mallRefundId }).Count -ne 1) {
    throw "Mall refund list did not include smoke refund"
  }
  $adminMallRefunds = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/refunds?keyword=$($mallOrder.order_no)&status=1&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminMallRefunds.items | Where-Object { [string]$_.id -eq [string]$mallRefundId }).Count -ne 1) {
    throw "Admin mall refunds did not include requested smoke refund"
  }
  $reviewMallRefundBody = @{
    approved = $true
    admin_note = "Smoke refund approved"
    restore_stock = $true
  } | ConvertTo-Json
  $approvedMallRefund = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/refunds/$mallRefundId/review" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $reviewMallRefundBody -TimeoutSec 10
  if ([int64]$approvedMallRefund.refund.status -ne 3) {
    throw "Admin mall refund approval did not approve refund"
  }
  $mallCreditAfterRefund = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallCreditAfterRefund.balance.total -ne ([int64]$mallCreditBeforeRefund.balance.total + $mallOrderTotal)) {
    throw "Mall refund approval did not restore expected credit balance"
  }
  $mallOrderAfterRefund = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$mallOrderId" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallOrderAfterRefund.order.status -ne 8) {
    throw "Mall order did not move to refunded after refund approval"
  }
  $mallProductAfterRefund = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId" -Method Get -TimeoutSec 10
  if ([int64]$mallProductAfterRefund.product.stock -ne $mallProductStock) {
    throw "Mall refund approval did not restore product stock"
  }
  $hiddenMallReviewsAfterRefund = Invoke-Api -Uri "$baseUrl/api/v1/mall/reviews?product_id=$mallProductId&status=3&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  if (@($hiddenMallReviewsAfterRefund.items | Where-Object { [string]$_.id -eq [string]$mallReviewId }).Count -ne 1) {
    throw "Mall refund approval did not hide the published review from the owner's visible review history"
  }
  $publicMallReviewsAfterRefund = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviews?limit=20&offset=0" -Method Get -TimeoutSec 10
  if (@($publicMallReviewsAfterRefund.items | Where-Object { [string]$_.id -eq [string]$mallReviewId }).Count -ne 0) {
    throw "Mall refund approval did not remove the published review from public product reviews"
  }
  $mallProductStockLogs = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/products/$mallProductId/stock-logs?limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($mallProductStockLogs.items).Count -lt 3) {
    throw "Admin mall stock logs did not include create/order/refund stock changes"
  }
  $mallOverview = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/overview?low_stock_threshold=10" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$mallOverview.overview.order_total -lt 1 -or [int64]$mallOverview.overview.refunded_credits_total -lt $mallOrderTotal) {
    throw "Admin mall overview did not include smoke order/refund totals"
  }
  foreach ($property in @("succeeded_payment_credits_total", "failed_payment_total", "failed_payment_credits_total", "pending_refund_credits_total", "net_revenue_credits_total")) {
    if (@($mallOverview.overview.PSObject.Properties.Name) -notcontains $property) {
      throw "Admin mall overview did not include finance field $property"
    }
  }
  if ([int64]$mallOverview.overview.succeeded_payment_credits_total -lt $mallOrderTotal -or [int64]$mallOverview.overview.net_revenue_credits_total -lt 0 -or [int64]$mallOverview.overview.pending_refund_credits_total -lt 0) {
    throw "Admin mall overview did not include valid finance reconciliation totals"
  }
  if (@($mallOverview.overview.PSObject.Properties.Name) -notcontains "pending_outbox_total" -or [int64]$mallOverview.overview.pending_outbox_total -lt 0) {
    throw "Admin mall overview did not include valid pending outbox total"
  }
  $mallOutboxRequeueBody = @{
    statuses = @("failed", "dead_letter")
    limit = 1
  } | ConvertTo-Json -Depth 4
  $mallOutboxRequeue = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/outbox/requeue" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $mallOutboxRequeueBody -TimeoutSec 10
  if (@($mallOutboxRequeue.PSObject.Properties.Name) -notcontains "requeued" -or [int64]$mallOutboxRequeue.requeued -lt 0) {
    throw "Admin mall outbox requeue did not return a valid requeued count"
  }
  $rejectOrderBody = @{
    idempotency_key = "smoke-mall-reject-order-$stamp"
    items = @(@{
        product_id = $mallProductId
        quantity = 1
      })
    receiver = "Smoke User"
    phone = "13800000000"
    address = "Smoke Province Smoke City Smoke District Reject Street $stamp"
  } | ConvertTo-Json -Depth 5
  $rejectOrderCreated = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $rejectOrderBody -TimeoutSec 10
  $rejectOrder = $rejectOrderCreated.order
  $rejectOrderId = $rejectOrder.id
  if (-not $rejectOrderId -or [int64]$rejectOrder.status -ne 1 -or [int64]$rejectOrder.total_credits -ne $mallProductPrice) {
    throw "Mall reject-path order did not create expected pending payment order"
  }
  $rejectOrderPayBody = @{
    payment_method = "credits"
    idempotency_key = "smoke-mall-reject-pay-$stamp"
  } | ConvertTo-Json
  $rejectOrderPaid = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$rejectOrderId/pay" -Method Post -Headers $headers -ContentType "application/json" -Body $rejectOrderPayBody -TimeoutSec 10
  if ([int64]$rejectOrderPaid.order.status -ne 3) {
    throw "Mall reject-path pay did not move order to paid"
  }
  $shipRejectOrderBody = @{
    status = 5
    shipping_carrier = "Smoke Express"
    tracking_no = "REJECT$stamp"
    note = "Smoke reject-path order shipped"
  } | ConvertTo-Json
  $rejectOrderShipped = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders/$rejectOrderId/status" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $shipRejectOrderBody -TimeoutSec 10
  if ([int64]$rejectOrderShipped.order.status -ne 5) {
    throw "Mall reject-path ship did not move order to shipped"
  }
  $creditBeforeRejectReview = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$creditBeforeRejectReview.balance.total -ne ([int64]$mallCreditAfterRefund.balance.total - $mallProductPrice)) {
    throw "Mall reject-path pay did not debit expected credit amount"
  }
  $rejectRefundBody = @{
    reason = "smoke_reject_after_sale"
    note = "Smoke refund reject $stamp"
  } | ConvertTo-Json
  $rejectRefundCreated = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$rejectOrderId/refunds" -Method Post -Headers $headers -ContentType "application/json" -Body $rejectRefundBody -TimeoutSec 10
  $rejectRefundId = $rejectRefundCreated.refund.id
  if (-not $rejectRefundId -or [int64]$rejectRefundCreated.refund.status -ne 1 -or [int64]$rejectRefundCreated.refund.amount_credits -ne $mallProductPrice) {
    throw "Mall reject-path refund request did not return expected requested refund"
  }
  Assert-ApiStatusMessage 412 "invalid order state" -Uri "$baseUrl/api/v1/mall/orders/$rejectOrderId/confirm" -Method Post -Headers $headers -TimeoutSec 10
  $mallConfirmBlockedDuringRefund = $true
  $completeRejectOrderBody = @{
    status = 6
    note = "Smoke reject-path order completed"
  } | ConvertTo-Json
  Assert-ApiStatusMessage 412 "invalid order state" -Uri "$baseUrl/api/v1/admin/mall/orders/$rejectOrderId/status" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $completeRejectOrderBody -TimeoutSec 10
  $mallAdminStatusUpdateBlockedDuringRefund = $true
  $reviewableAfterRefundRequest = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviewable-orders?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  if (@($reviewableAfterRefundRequest.items | Where-Object { [string]$_.id -eq [string]$rejectOrderId }).Count -ne 0) {
    throw "Mall reviewable orders included an order with a refund request"
  }
  $reviewDuringRefundBody = @{
    order_id = $rejectOrderId
    rating = 5
    content = "Smoke review blocked by refund $stamp"
  } | ConvertTo-Json
  Assert-ApiStatusMessage 412 "invalid order state" -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviews" -Method Post -Headers $headers -ContentType "application/json" -Body $reviewDuringRefundBody -TimeoutSec 10
  $mallReviewBlockedDuringRefund = $true
  $rejectRefundReviewBody = @{
    approved = $false
    admin_note = "Smoke refund rejected"
    restore_stock = $true
  } | ConvertTo-Json
  $rejectedMallRefund = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/refunds/$rejectRefundId/review" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $rejectRefundReviewBody -TimeoutSec 10
  if ([int64]$rejectedMallRefund.refund.status -ne 4) {
    throw "Admin mall refund rejection did not reject refund"
  }
  $rejectOrderCompleted = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders/$rejectOrderId/status" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $completeRejectOrderBody -TimeoutSec 10
  if ([int64]$rejectOrderCompleted.order.status -ne 6) {
    throw "Mall reject-path complete did not move order to completed after refund rejection"
  }
  $creditAfterRejectReview = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$creditAfterRejectReview.balance.total -ne [int64]$creditBeforeRejectReview.balance.total) {
    throw "Mall refund rejection unexpectedly changed credit balance"
  }
  $rejectOrderAfterReview = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$rejectOrderId" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$rejectOrderAfterReview.order.status -ne 6) {
    throw "Mall refund rejection unexpectedly changed order status"
  }
  $productAfterRejectReview = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId" -Method Get -TimeoutSec 10
  if ([int64]$productAfterRejectReview.product.stock -ne ($mallProductStock - 1)) {
    throw "Mall refund rejection unexpectedly restored product stock"
  }
  $reviewableAfterRefundRejection = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviewable-orders?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  if (@($reviewableAfterRefundRejection.items | Where-Object { [string]$_.id -eq [string]$rejectOrderId }).Count -ne 1) {
    throw "Mall reviewable orders did not restore an order after its refund was rejected"
  }
  $reviewAfterRejectedRefundBody = @{
    order_id = $rejectOrderId
    rating = 5
    content = "Smoke review after refund rejection $stamp"
  } | ConvertTo-Json
  $reviewAfterRejectedRefund = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallProductId/reviews" -Method Post -Headers $headers -ContentType "application/json" -Body $reviewAfterRejectedRefundBody -TimeoutSec 10
  if (-not $reviewAfterRejectedRefund.review.id -or [int64]$reviewAfterRejectedRefund.review.status -ne 1) {
    throw "Mall refund rejection did not restore product review creation"
  }
  $mallReviewAllowedAfterRefundRejection = $true
  $rejectedRefunds = Invoke-Api -Uri "$baseUrl/api/v1/mall/refunds?status=4&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  if (@($rejectedRefunds.items | Where-Object { [string]$_.id -eq [string]$rejectRefundId }).Count -ne 1) {
    throw "Mall rejected refund list did not include smoke refund"
  }
  $expensiveProductSku = "SMOKE-EXP-$stamp"
  $expensiveProductPrice = [int64]$creditAfterRejectReview.balance.total + 100
  $expensiveProductBody = @{
    sku = $expensiveProductSku
    title = "Smoke Expensive Product $stamp"
    description = "Smoke insufficient credit product"
    category = $mallCategorySlug
    cover_url = ""
    price_credits = $expensiveProductPrice
    stock = 1
    status = 2
    sort = 102
  } | ConvertTo-Json
  $createdExpensiveProduct = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/products" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $expensiveProductBody -TimeoutSec 10
  $expensiveProductId = $createdExpensiveProduct.product.id
  if (-not $expensiveProductId -or [int64]$createdExpensiveProduct.product.price_credits -ne $expensiveProductPrice) {
    throw "Admin mall expensive product create did not return expected product"
  }
  $expensiveOrderBody = @{
    idempotency_key = "smoke-mall-insufficient-order-$stamp"
    items = @(@{
        product_id = $expensiveProductId
        quantity = 1
      })
    receiver = "Smoke User"
    phone = "13800000000"
    address = "Smoke Province Smoke City Smoke District Insufficient Street $stamp"
  } | ConvertTo-Json -Depth 5
  $expensiveOrderCreated = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $expensiveOrderBody -TimeoutSec 10
  $expensiveOrderId = $expensiveOrderCreated.order.id
  if (-not $expensiveOrderId -or [int64]$expensiveOrderCreated.order.status -ne 1 -or [int64]$expensiveOrderCreated.order.total_credits -ne $expensiveProductPrice) {
    throw "Mall insufficient-credit order did not create expected pending payment order"
  }
  $expensiveProductAfterOrder = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$expensiveProductId" -Method Get -TimeoutSec 10
  if ([int64]$expensiveProductAfterOrder.product.stock -ne 0) {
    throw "Mall insufficient-credit order did not lock product stock"
  }
  $expensivePayBody = @{
    payment_method = "credits"
    idempotency_key = "smoke-mall-insufficient-pay-$stamp"
  } | ConvertTo-Json
  Assert-ApiStatus 412 -Uri "$baseUrl/api/v1/mall/orders/$expensiveOrderId/pay" -Method Post -Headers $headers -ContentType "application/json" -Body $expensivePayBody -TimeoutSec 10
  $creditAfterInsufficientPay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$creditAfterInsufficientPay.balance.total -ne [int64]$creditAfterRejectReview.balance.total) {
    throw "Mall insufficient-credit pay unexpectedly changed credit balance"
  }
  $expensiveOrderAfterFailedPay = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$expensiveOrderId" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$expensiveOrderAfterFailedPay.order.status -ne 1) {
    throw "Mall insufficient-credit pay did not return order to pending payment"
  }
  $expensivePayments = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders/$expensiveOrderId/payments" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $failedExpensivePaymentListed = $false
  foreach ($item in @($expensivePayments.items)) {
    if ([int64]$item.status -eq 3 -and -not [string]::IsNullOrWhiteSpace([string]$item.failure_reason)) {
      $failedExpensivePaymentListed = $true
    }
  }
  if (-not $failedExpensivePaymentListed) {
    throw "Admin mall order payments did not include failed insufficient-credit payment"
  }
  Assert-ApiStatus 412 -Uri "$baseUrl/api/v1/mall/orders/$expensiveOrderId/pay" -Method Post -Headers $headers -ContentType "application/json" -Body $expensivePayBody -TimeoutSec 10
  $expensivePaymentsAfterRetry = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders/$expensiveOrderId/payments" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($expensivePaymentsAfterRetry.items).Count -ne 1 -or [int64]$expensivePaymentsAfterRetry.items[0].status -ne 3) {
    throw "Mall payment retry did not reuse the failed payment attempt"
  }
  $expensiveLogsAfterFailedPay = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$expensiveOrderId/logs" -Method Get -Headers $headers -TimeoutSec 10
  $paymentFailedLogListed = $false
  foreach ($item in @($expensiveLogsAfterFailedPay.items)) {
    if ($item.reason -eq "payment_failed") {
      $paymentFailedLogListed = $true
    }
  }
  if (-not $paymentFailedLogListed) {
    throw "Mall insufficient-credit order logs did not include payment_failed"
  }
  $mallOverviewAfterFailedPayment = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/overview?low_stock_threshold=10" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$mallOverviewAfterFailedPayment.overview.failed_payment_total -lt 1 -or [int64]$mallOverviewAfterFailedPayment.overview.failed_payment_credits_total -lt $expensiveProductPrice) {
    throw "Admin mall overview did not include failed payment reconciliation totals"
  }
  $expensiveOrderCanceled = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$expensiveOrderId/cancel" -Method Post -Headers $headers -TimeoutSec 10
  if ([int64]$expensiveOrderCanceled.order.status -ne 4) {
    throw "Mall insufficient-credit order cancel did not move order to canceled"
  }
  $expensiveProductAfterCancel = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$expensiveProductId" -Method Get -TimeoutSec 10
  if ([int64]$expensiveProductAfterCancel.product.stock -ne 1) {
    throw "Mall insufficient-credit order cancel did not release product stock"
  }

  $expiringOrderBody = @{
    idempotency_key = "smoke-mall-expiring-order-$stamp"
    items = @(@{
        product_id = $expensiveProductId
        quantity = 1
      })
    receiver = "Smoke User"
    phone = "13800000000"
    address = "Smoke Province Smoke City Smoke District Expiring Street $stamp"
  } | ConvertTo-Json -Depth 5
  $expiringOrderCreated = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $expiringOrderBody -TimeoutSec 10
  $expiringOrderId = $expiringOrderCreated.order.id
  if (-not $expiringOrderId -or [int64]$expiringOrderCreated.order.status -ne 1) {
    throw "Mall expiring order did not create expected pending payment order"
  }
  $expiringProductAfterOrder = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$expensiveProductId" -Method Get -TimeoutSec 10
  if ([int64]$expiringProductAfterOrder.product.stock -ne 0) {
    throw "Mall expiring order did not lock product stock"
  }
  Start-Sleep -Seconds 2
  $expiringOrderClosedByAdmin = $false
  for ($i = 0; $i -lt 5; $i++) {
    $closeExpiredBody = @{
      expire_after_seconds = 1
      limit = 100
    } | ConvertTo-Json
    $closedExpiredOrders = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders/expire" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $closeExpiredBody -TimeoutSec 10
    foreach ($item in @($closedExpiredOrders.items)) {
      if ([string]$item.id -eq [string]$expiringOrderId -and [int64]$item.status -eq 7) {
        $expiringOrderClosedByAdmin = $true
      }
    }
    if ($expiringOrderClosedByAdmin) {
      break
    }
  }
  if (-not $expiringOrderClosedByAdmin) {
    throw "Admin mall close expired orders did not close smoke pending order"
  }
  $recoverPayingBody = @{
    stale_after_seconds = 1
    limit = 100
  } | ConvertTo-Json
  $recoverPayingResult = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/orders/recover-paying" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $recoverPayingBody -TimeoutSec 10
  if ($null -eq $recoverPayingResult.recovered -or $null -eq $recoverPayingResult.failed) {
    throw "Admin mall recover paying orders did not return recovered/failed counters"
  }
  $expiringOrderAfterClose = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$expiringOrderId" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$expiringOrderAfterClose.order.status -ne 7) {
    throw "Mall expiring order did not move to closed"
  }
  $expiringProductAfterClose = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$expensiveProductId" -Method Get -TimeoutSec 10
  if ([int64]$expiringProductAfterClose.product.stock -ne 1) {
    throw "Mall expiring order close did not release product stock"
  }
  $expiringOrderLogs = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$expiringOrderId/logs" -Method Get -Headers $headers -TimeoutSec 10
  $expiredOrderLogListed = $false
  foreach ($item in @($expiringOrderLogs.items)) {
    if ($item.reason -eq "expired") {
      $expiredOrderLogListed = $true
    }
  }
  if (-not $expiredOrderLogListed) {
    throw "Mall expiring order logs did not include expired"
  }
  $expiredStockLogs = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/products/$expensiveProductId/stock-logs?reason=order_expired&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $expiredStockLogListed = $false
  foreach ($item in @($expiredStockLogs.items)) {
    if ([string]$item.reference_id -eq [string]$expiringOrderId -and [int64]$item.after_stock -eq 1) {
      $expiredStockLogListed = $true
    }
  }
  if (-not $expiredStockLogListed) {
    throw "Admin mall stock logs did not include expired order stock release"
  }

  $membershipOrderBody = @{
    idempotency_key = "smoke-mall-membership-order-$stamp"
    items = @(@{
        product_id = $mallMembershipProductId
        quantity = 1
      })
  } | ConvertTo-Json -Depth 5
  $membershipOrderCreated = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $membershipOrderBody -TimeoutSec 10
  $membershipOrder = $membershipOrderCreated.order
  $membershipOrderId = $membershipOrder.id
  if (-not $membershipOrderId -or [int64]$membershipOrder.status -ne 1 -or [int64]$membershipOrder.total_credits -ne [int64]$mallMembershipProductPrice) {
    throw "Mall membership order did not create expected pending payment order"
  }
  $membershipOrderItem = @($membershipOrder.items)[0]
  if ($membershipOrderItem.grant_type -ne "membership" -or $membershipOrderItem.grant_key -ne "vip-month") {
    throw "Mall membership order item did not snapshot membership grant fields"
  }
  $mallCreditBeforeMembershipPay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  $membershipPayBody = @{
    payment_method = "credits"
    idempotency_key = "smoke-mall-membership-pay-$stamp"
  } | ConvertTo-Json
  $membershipOrderPaid = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$membershipOrderId/pay" -Method Post -Headers $headers -ContentType "application/json" -Body $membershipPayBody -TimeoutSec 10
  if ([int64]$membershipOrderPaid.order.status -ne 6) {
    throw "Mall membership order pay did not auto-complete the membership order"
  }
  $mallCreditAfterMembershipPay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallCreditAfterMembershipPay.balance.total -ne ([int64]$mallCreditBeforeMembershipPay.balance.total - [int64]$mallMembershipProductPrice)) {
    throw "Mall membership order pay did not debit expected credit amount"
  }
  $membershipOrderEntitlements = @($membershipOrderPaid.order.digital_entitlements)
  if ($membershipOrderEntitlements.Count -lt 1) {
    throw "Mall membership order pay did not return issued membership entitlement"
  }
  $membershipOrderEntitlement = $membershipOrderEntitlements[0]
  if ($membershipOrderEntitlement.status -ne "ACTIVE" -or $membershipOrderEntitlement.grant_type -ne "membership" -or $membershipOrderEntitlement.grant_key -ne "vip-month" -or [string]::IsNullOrWhiteSpace([string]$membershipOrderEntitlement.fulfillment_code)) {
    throw "Mall membership order entitlement did not include expected active membership grant"
  }
  $mallMembershipEntitlements = Invoke-Api -Uri "$baseUrl/api/v1/mall/digital-entitlements?status=ACTIVE&limit=50&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $mallMembershipEntitlementListed = $false
  foreach ($item in @($mallMembershipEntitlements.items)) {
    if ([string]$item.order_id -eq [string]$membershipOrderId -and [string]$item.product_id -eq [string]$mallMembershipProductId -and $item.status -eq "ACTIVE" -and $item.grant_type -eq "membership" -and $item.grant_key -eq "vip-month") {
      $mallMembershipEntitlementListed = $true
    }
  }
  if (-not $mallMembershipEntitlementListed) {
    throw "Mall digital entitlement list did not include active membership grant"
  }

  $membershipProfileBackgroundUrl = "https://example.com/smoke-membership-bg-$stamp.webp"
  $membershipProfileBody = @{
    nickname = "Smoke Updated"
    avatar_url = ""
    bio = "Local smoke profile"
    background_url = $membershipProfileBackgroundUrl
  } | ConvertTo-Json
  $membershipProfile = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Put -Headers $headers -ContentType "application/json" -Body $membershipProfileBody -TimeoutSec 10
  if ($membershipProfile.user.background_url -ne $membershipProfileBackgroundUrl) {
    throw "Active membership entitlement did not allow setting profile background"
  }
  $membershipProfileRead = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Get -Headers $headers -TimeoutSec 10
  $membershipProfileBackgroundStored = [string]$membershipProfileRead.user.background_url -eq [string]$membershipProfileBackgroundUrl
  if (-not $membershipProfileBackgroundStored) {
    throw "Active membership profile background was not returned by current user profile"
  }

  $membershipQaDraftTitle = "Membership draft bounty QA $stamp"
  $membershipQaDraftBody = @{
    slug = "membership-qa-draft-$stamp"
    type = "qa"
    title = $membershipQaDraftTitle
    body = "This bounty QA draft must be rejected at publish time after membership is revoked."
    tags = @("membership", "qa", "draft")
    category_id = $categoryId
    bounty_score = $membershipQaBounty
    publish = $false
  } | ConvertTo-Json
  $membershipQaDraftTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $membershipQaDraftBody -TimeoutSec 10
  $membershipQaDraftTopicId = $membershipQaDraftTopic.topic.id
  if (-not $membershipQaDraftTopicId -or [int64]$membershipQaDraftTopic.topic.status -ne 1 -or $membershipQaDraftTopic.topic.type -ne "qa" -or [int64]$membershipQaDraftTopic.topic.bounty_score -ne $membershipQaBounty) {
    throw "Membership entitlement did not allow creating a bounty QA draft"
  }

  $membershipQaTitle = "Membership bounty QA $stamp"
  $membershipQaTopicBody = @{
    slug = "membership-qa-$stamp"
    type = "qa"
    title = $membershipQaTitle
    body = "This bounty QA validates the purchased membership entitlement."
    tags = @("membership", "qa")
    category_id = $categoryId
    bounty_score = $membershipQaBounty
    publish = $true
  } | ConvertTo-Json
  $membershipQaTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $membershipQaTopicBody -TimeoutSec 10
  $membershipQaTopicId = $membershipQaTopic.topic.id
  if (-not $membershipQaTopicId -or [int64]$membershipQaTopic.topic.status -ne 2 -or $membershipQaTopic.topic.type -ne "qa" -or [int64]$membershipQaTopic.topic.bounty_score -ne $membershipQaBounty) {
    throw "Membership entitlement did not allow publishing a bounty QA topic"
  }
  $membershipQaBountyReserved = $false
  $membershipQaActorLedger = $null
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $membershipQaActorLedger = Invoke-Api -Uri "$baseUrl/api/v1/credits/ledger?limit=50&offset=0" -Method Get -Headers $headers -TimeoutSec 10
      foreach ($item in @($membershipQaActorLedger.items)) {
        if ($item.reason -eq "qa_bounty_reserved" -and $item.source_type -eq "topic" -and [string]$item.source_id -eq [string]$membershipQaTopicId -and [int64]$item.delta -eq -$membershipQaBounty) {
          $membershipQaBountyReserved = $true
        }
      }
      if ($membershipQaBountyReserved) {
        break
      }
    } catch {
    }
  }
  if (-not $membershipQaBountyReserved) {
    throw "Membership bounty QA publishing did not project expected reserved credit ledger entry"
  }
  $membershipQaAnswerBody = @{
    content = "Membership bounty QA answer $stamp"
    parent_id = 0
  } | ConvertTo-Json
  $membershipQaAnswer = Invoke-Api -Uri "$baseUrl/api/v1/topics/$membershipQaTopicId/comments" -Method Post -Headers $followeeHeaders -ContentType "application/json" -Body $membershipQaAnswerBody -TimeoutSec 10
  $membershipQaAnswerId = $membershipQaAnswer.comment.id
  if (-not $membershipQaAnswerId) {
    throw "Membership bounty QA answer response did not include comment.id"
  }
  $membershipQaAccepted = Invoke-Api -Uri "$baseUrl/api/v1/topics/$membershipQaTopicId/comments/$membershipQaAnswerId/accept" -Method Post -Headers $headers -TimeoutSec 10
  if ([int64]$membershipQaAccepted.topic.accepted_comment_id -ne [int64]$membershipQaAnswerId -or $membershipQaAccepted.topic.qa_status -ne "resolved") {
    throw "Membership bounty QA accept response did not mark the answer as accepted"
  }
  $membershipQaBountySettled = $membershipQaBountyReserved
  $membershipQaAnswerRewarded = $false
  $membershipQaAnswerLedger = $null
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $membershipQaActorLedger = Invoke-Api -Uri "$baseUrl/api/v1/credits/ledger?limit=50&offset=0" -Method Get -Headers $headers -TimeoutSec 10
      $membershipQaAnswerLedger = Invoke-Api -Uri "$baseUrl/api/v1/credits/ledger?limit=50&offset=0" -Method Get -Headers $followeeHeaders -TimeoutSec 10
      foreach ($item in @($membershipQaActorLedger.items)) {
        if ($item.reason -eq "qa_bounty_paid" -and $item.source_type -eq "topic" -and [string]$item.source_id -eq [string]$membershipQaTopicId -and [int64]$item.delta -eq -$membershipQaBounty) {
          $membershipQaBountySettled = $true
        }
      }
      foreach ($item in @($membershipQaAnswerLedger.items)) {
        if ($item.reason -eq "qa_answer_accepted" -and $item.source_type -eq "comment" -and [string]$item.source_id -eq [string]$membershipQaAnswerId -and [int64]$item.delta -eq $membershipQaBounty) {
          $membershipQaAnswerRewarded = $true
        }
      }
      if ($membershipQaBountySettled -and $membershipQaAnswerRewarded) {
        break
      }
    } catch {
    }
  }
  if (-not $membershipQaBountySettled -or -not $membershipQaAnswerRewarded) {
    throw "Membership bounty QA acceptance did not project expected credit ledger entries"
  }

  $membershipQaRevocationTitle = "Membership revoked bounty QA $stamp"
  $membershipQaRevocationTopicBody = @{
    slug = "membership-qa-revoked-$stamp"
    type = "qa"
    title = $membershipQaRevocationTitle
    body = "This bounty QA must remain unsettled after its membership entitlement is revoked."
    tags = @("membership", "qa", "revoke")
    category_id = $categoryId
    bounty_score = $membershipQaBounty
    publish = $true
  } | ConvertTo-Json
  $membershipQaRevocationTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $membershipQaRevocationTopicBody -TimeoutSec 10
  $membershipQaRevocationTopicId = $membershipQaRevocationTopic.topic.id
  if (-not $membershipQaRevocationTopicId -or [int64]$membershipQaRevocationTopic.topic.status -ne 2 -or [int64]$membershipQaRevocationTopic.topic.bounty_score -ne $membershipQaBounty) {
    throw "Membership entitlement did not allow publishing a pending bounty QA topic"
  }
  $membershipQaRevocationAnswer = Invoke-Api -Uri "$baseUrl/api/v1/topics/$membershipQaRevocationTopicId/comments" -Method Post -Headers $followeeHeaders -ContentType "application/json" -Body $membershipQaAnswerBody -TimeoutSec 10
  $membershipQaRevocationAnswerId = $membershipQaRevocationAnswer.comment.id
  if (-not $membershipQaRevocationAnswerId) {
    throw "Pending membership bounty QA answer response did not include comment.id"
  }

  $membershipRefundBody = @{
    reason = "smoke_membership_after_sale"
    note = "Smoke membership entitlement refund $stamp"
  } | ConvertTo-Json
  $membershipRefundRejected = $false
  Assert-ApiStatus 412 -Uri "$baseUrl/api/v1/mall/orders/$membershipOrderId/refunds" -Method Post -Headers $headers -ContentType "application/json" -Body $membershipRefundBody -TimeoutSec 10
  $membershipRefundRejected = $true
  $membershipRevokeReason = "Smoke membership manual revoke $stamp"
  $membershipRevokeBody = @{
    reason = $membershipRevokeReason
  } | ConvertTo-Json
  $mallCreditBeforeMembershipRevoke = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  $revokedMembershipEntitlement = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/digital-entitlements/$($membershipOrderEntitlement.id)/revoke" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $membershipRevokeBody -TimeoutSec 10
  if (-not $revokedMembershipEntitlement.entitlement.id -or $revokedMembershipEntitlement.entitlement.status -ne "REVOKED" -or $revokedMembershipEntitlement.entitlement.revoke_reason -ne $membershipRevokeReason) {
    throw "Admin mall membership entitlement revoke did not return expected revoked entitlement"
  }
  $membershipOrderAfterRevoke = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$membershipOrderId" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$membershipOrderAfterRevoke.order.status -ne 6) {
    throw "Mall membership order should remain completed after manual entitlement revoke"
  }
  $mallCreditAfterMembershipRevoke = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallCreditAfterMembershipRevoke.balance.total -ne [int64]$mallCreditBeforeMembershipRevoke.balance.total) {
    throw "Mall membership entitlement revoke unexpectedly changed credit balance"
  }
  $mallRevokedMembershipEntitlements = Invoke-Api -Uri "$baseUrl/api/v1/mall/digital-entitlements?status=REVOKED&limit=50&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $mallMembershipEntitlementRevoked = $false
  foreach ($item in @($mallRevokedMembershipEntitlements.items)) {
    if ([string]$item.order_id -eq [string]$membershipOrderId -and [string]$item.product_id -eq [string]$mallMembershipProductId -and [string]$item.id -eq [string]$membershipOrderEntitlement.id -and $item.status -eq "REVOKED" -and $item.grant_type -eq "membership" -and $item.grant_key -eq "vip-month" -and $item.revoke_reason -eq $membershipRevokeReason) {
      $mallMembershipEntitlementRevoked = $true
    }
  }
  if (-not $mallMembershipEntitlementRevoked) {
    throw "Mall digital entitlement list did not include revoked membership grant after manual revoke"
  }
  $membershipProfileAfterRevoke = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Get -Headers $headers -TimeoutSec 10
  $membershipProfileBackgroundHiddenAfterRevoke = [string]::IsNullOrWhiteSpace([string]$membershipProfileAfterRevoke.user.background_url)
  if (-not $membershipProfileBackgroundHiddenAfterRevoke) {
    throw "Revoked membership entitlement still exposed profile background"
  }
  $membershipQaDraftPublishForbidden = $false
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics/$membershipQaDraftTopicId/publish" -Method Post -Headers $headers -TimeoutSec 10
  $membershipQaDraftPublishForbidden = $true
  $membershipQaDraftAdminPublishForbidden = $false
  Assert-ApiStatusMessage 403 "TOPIC_MEMBERSHIP_ENTITLEMENT_REQUIRED" -Uri "$baseUrl/api/v1/admin/topics/$membershipQaDraftTopicId/publish" -Method Post -Headers $adminHeaders -TimeoutSec 10
  $membershipQaDraftAdminPublishForbidden = $true
  $membershipQaAcceptForbiddenAfterRevoke = $false
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics/$membershipQaRevocationTopicId/comments/$membershipQaRevocationAnswerId/accept" -Method Post -Headers $headers -TimeoutSec 10
  $membershipQaAcceptForbiddenAfterRevoke = $true
  $membershipQaRevocationTopicAfterRevoke = Invoke-Api -Uri "$baseUrl/api/v1/topics/$membershipQaRevocationTopicId" -Method Get -TimeoutSec 10
  $membershipQaUnsettledAfterRevoke = [int64]$membershipQaRevocationTopicAfterRevoke.topic.accepted_comment_id -eq 0 -and $membershipQaRevocationTopicAfterRevoke.topic.qa_status -eq "open"
  if (-not $membershipQaUnsettledAfterRevoke) {
    throw "Revoked membership entitlement still allowed bounty QA acceptance"
  }

  $revokedBountyTopicBody = @{
    slug = "qa-bounty-revoked-$stamp"
    type = "qa"
    title = "Revoked bounty QA $stamp"
    body = "This bounty QA must be rejected after the membership entitlement is revoked."
    tags = @("qa", "revoked")
    category_id = $categoryId
    bounty_score = $membershipQaBounty
    publish = $true
  } | ConvertTo-Json
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $revokedBountyTopicBody -TimeoutSec 10

  $themeOrderBody = @{
    idempotency_key = "smoke-mall-theme-order-$stamp"
    items = @(@{
        product_id = $mallThemeProductId
        quantity = 1
      })
  } | ConvertTo-Json -Depth 5
  $themeOrderCreated = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $themeOrderBody -TimeoutSec 10
  $themeOrder = $themeOrderCreated.order
  $themeOrderId = $themeOrder.id
  if (-not $themeOrderId -or [int64]$themeOrder.status -ne 1 -or [int64]$themeOrder.total_credits -ne [int64]$mallThemeProductPrice) {
    throw "Mall theme order did not create expected pending payment order"
  }
  $themeOrderItem = @($themeOrder.items)[0]
  if ($themeOrderItem.grant_type -ne "theme" -or $themeOrderItem.grant_key -ne "theme-pro") {
    throw "Mall theme order item did not snapshot theme grant fields"
  }
  $pendingDuplicateThemeOrderBody = @{
    idempotency_key = "smoke-mall-theme-pending-duplicate-order-$stamp"
    items = @(@{
        product_id = $mallThemeProductId
        quantity = 1
      })
  } | ConvertTo-Json -Depth 5
  Assert-ApiStatusMessage 412 "pending theme order already exists" -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $pendingDuplicateThemeOrderBody -TimeoutSec 10
  $mallThemePendingDuplicateOrderRejected = $true
  $mallCreditBeforeThemePay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  $themePayBody = @{
    payment_method = "credits"
    idempotency_key = "smoke-mall-theme-pay-$stamp"
  } | ConvertTo-Json
  $themeOrderPaid = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$themeOrderId/pay" -Method Post -Headers $headers -ContentType "application/json" -Body $themePayBody -TimeoutSec 10
  if ([int64]$themeOrderPaid.order.status -ne 6) {
    throw "Mall theme order pay did not auto-complete the theme order"
  }
  $mallCreditAfterThemePay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallCreditAfterThemePay.balance.total -ne ([int64]$mallCreditBeforeThemePay.balance.total - [int64]$mallThemeProductPrice)) {
    throw "Mall theme order pay did not debit expected credit amount"
  }
  $themeOrderEntitlements = @($themeOrderPaid.order.digital_entitlements)
  if ($themeOrderEntitlements.Count -lt 1) {
    throw "Mall theme order pay did not return issued theme entitlement"
  }
  $themeOrderEntitlement = $themeOrderEntitlements[0]
  if ($themeOrderEntitlement.status -ne "ACTIVE" -or $themeOrderEntitlement.grant_type -ne "theme" -or $themeOrderEntitlement.grant_key -ne "theme-pro" -or [string]::IsNullOrWhiteSpace([string]$themeOrderEntitlement.fulfillment_code)) {
    throw "Mall theme order entitlement did not include expected active theme grant"
  }
  $mallThemeEntitlements = Invoke-Api -Uri "$baseUrl/api/v1/mall/digital-entitlements?status=ACTIVE&grant_type=theme&grant_key=theme-pro&limit=50&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $mallThemeEntitlementListed = $false
  foreach ($item in @($mallThemeEntitlements.items)) {
    if ([string]$item.order_id -eq [string]$themeOrderId -and [string]$item.product_id -eq [string]$mallThemeProductId -and $item.status -eq "ACTIVE" -and $item.grant_type -eq "theme" -and $item.grant_key -eq "theme-pro") {
      $mallThemeEntitlementListed = $true
    }
  }
  if (-not $mallThemeEntitlementListed) {
    throw "Mall digital entitlement list did not include active theme grant"
  }
  $duplicateThemeOrderBody = @{
    idempotency_key = "smoke-mall-theme-duplicate-order-$stamp"
    items = @(@{
        product_id = $mallThemeProductId
        quantity = 1
      })
  } | ConvertTo-Json -Depth 5
  Assert-ApiStatusMessage 412 "active theme entitlement already exists" -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $duplicateThemeOrderBody -TimeoutSec 10
  $mallThemeDuplicateOrderRejected = $true

  $themeProfileBody = @{
    nickname = "Smoke Updated"
    avatar_url = ""
    bio = "Local smoke profile"
    background_url = ""
    profile_theme = "theme-pro"
  } | ConvertTo-Json
  $themeProfile = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Put -Headers $headers -ContentType "application/json" -Body $themeProfileBody -TimeoutSec 10
  if ($themeProfile.user.profile_theme -ne "theme-pro") {
    throw "Active theme entitlement did not allow setting profile theme"
  }
  $themeProfileRead = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Get -Headers $headers -TimeoutSec 10
  $mallThemeProfileThemeStored = [string]$themeProfileRead.user.profile_theme -eq "theme-pro"
  if (-not $mallThemeProfileThemeStored) {
    throw "Active theme profile theme was not returned by current user profile"
  }

  $themeRevokeReason = "Smoke theme manual revoke $stamp"
  $themeRevokeBody = @{
    reason = $themeRevokeReason
  } | ConvertTo-Json
  $mallCreditBeforeThemeRevoke = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  $revokedThemeEntitlement = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/digital-entitlements/$($themeOrderEntitlement.id)/revoke" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $themeRevokeBody -TimeoutSec 10
  if (-not $revokedThemeEntitlement.entitlement.id -or $revokedThemeEntitlement.entitlement.status -ne "REVOKED" -or $revokedThemeEntitlement.entitlement.revoke_reason -ne $themeRevokeReason -or $revokedThemeEntitlement.entitlement.grant_type -ne "theme" -or $revokedThemeEntitlement.entitlement.grant_key -ne "theme-pro") {
    throw "Admin mall theme entitlement revoke did not return expected revoked entitlement"
  }
  $themeOrderAfterRevoke = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$themeOrderId" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$themeOrderAfterRevoke.order.status -ne 6) {
    throw "Mall theme order should remain completed after manual entitlement revoke"
  }
  $mallCreditAfterThemeRevoke = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallCreditAfterThemeRevoke.balance.total -ne [int64]$mallCreditBeforeThemeRevoke.balance.total) {
    throw "Mall theme entitlement revoke unexpectedly changed credit balance"
  }
  $mallRevokedThemeEntitlements = Invoke-Api -Uri "$baseUrl/api/v1/mall/digital-entitlements?status=REVOKED&grant_type=theme&grant_key=theme-pro&limit=50&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $mallThemeEntitlementRevoked = $false
  foreach ($item in @($mallRevokedThemeEntitlements.items)) {
    if ([string]$item.order_id -eq [string]$themeOrderId -and [string]$item.product_id -eq [string]$mallThemeProductId -and [string]$item.id -eq [string]$themeOrderEntitlement.id -and $item.status -eq "REVOKED" -and $item.grant_type -eq "theme" -and $item.grant_key -eq "theme-pro" -and $item.revoke_reason -eq $themeRevokeReason) {
      $mallThemeEntitlementRevoked = $true
    }
  }
  if (-not $mallThemeEntitlementRevoked) {
    throw "Mall digital entitlement list did not include revoked theme grant after manual revoke"
  }
  $themeProfileAfterRevoke = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Get -Headers $headers -TimeoutSec 10
  $mallThemeProfileThemeDefaultAfterRevoke = [string]$themeProfileAfterRevoke.user.profile_theme -eq "default"
  if (-not $mallThemeProfileThemeDefaultAfterRevoke) {
    throw "Revoked theme entitlement still exposed profile theme"
  }

  $digitalOrderBody = @{
    idempotency_key = "smoke-mall-digital-order-$stamp"
    items = @(@{
        product_id = $mallDigitalProductId
        quantity = 1
      })
  } | ConvertTo-Json -Depth 5
  $digitalOrderCreated = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $digitalOrderBody -TimeoutSec 10
  $digitalOrder = $digitalOrderCreated.order
  $digitalOrderId = $digitalOrder.id
  if (-not $digitalOrderId -or [int64]$digitalOrder.status -ne 1 -or [int64]$digitalOrder.total_credits -ne $mallDigitalProductPrice) {
    throw "Mall digital order did not create expected pending payment order"
  }
  $digitalOrderItem = @($digitalOrder.items)[0]
  if ($digitalOrderItem.grant_type -ne "badge" -or $digitalOrderItem.grant_key -ne $mallDigitalGrantKey) {
    throw "Mall digital order item did not snapshot expected grant fields"
  }
  $pendingDuplicateBadgeOrderBody = @{
    idempotency_key = "smoke-mall-badge-pending-duplicate-order-$stamp"
    items = @(@{
        product_id = $mallDigitalProductId
        quantity = 1
      })
  } | ConvertTo-Json -Depth 5
  Assert-ApiStatusMessage 412 "pending badge order already exists" -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $pendingDuplicateBadgeOrderBody -TimeoutSec 10
  $mallBadgePendingDuplicateOrderRejected = $true
  $mallCreditBeforeDigitalPay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  $digitalPayBody = @{
    payment_method = "credits"
    idempotency_key = "smoke-mall-digital-pay-$stamp"
  } | ConvertTo-Json
  $digitalOrderPaid = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$digitalOrderId/pay" -Method Post -Headers $headers -ContentType "application/json" -Body $digitalPayBody -TimeoutSec 10
  if ([int64]$digitalOrderPaid.order.status -ne 6) {
    throw "Mall digital order pay did not auto-complete the digital order"
  }
  $mallCreditAfterDigitalPay = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallCreditAfterDigitalPay.balance.total -ne ([int64]$mallCreditBeforeDigitalPay.balance.total - $mallDigitalProductPrice)) {
    throw "Mall digital order pay did not debit expected credit amount"
  }
  $digitalOrderEntitlements = @($digitalOrderPaid.order.digital_entitlements)
  if ($digitalOrderEntitlements.Count -lt 1) {
    throw "Mall digital order pay did not return issued digital entitlement"
  }
  $digitalOrderEntitlement = $digitalOrderEntitlements[0]
  if ($digitalOrderEntitlement.status -ne "ACTIVE" -or $digitalOrderEntitlement.grant_type -ne "badge" -or $digitalOrderEntitlement.grant_key -ne $mallDigitalGrantKey -or [string]::IsNullOrWhiteSpace([string]$digitalOrderEntitlement.fulfillment_code)) {
    throw "Mall digital order entitlement did not include expected active grant fields"
  }
  $mallDigitalEntitlements = Invoke-Api -Uri "$baseUrl/api/v1/mall/digital-entitlements?status=ACTIVE&limit=50&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $mallDigitalEntitlementListed = $false
  foreach ($item in @($mallDigitalEntitlements.items)) {
    if ([string]$item.order_id -eq [string]$digitalOrderId -and [string]$item.product_id -eq [string]$mallDigitalProductId -and $item.status -eq "ACTIVE" -and $item.grant_type -eq "badge" -and $item.grant_key -eq $mallDigitalGrantKey) {
      $mallDigitalEntitlementListed = $true
    }
  }
  if (-not $mallDigitalEntitlementListed) {
    throw "Mall digital entitlement list did not include active smoke grant"
  }
  $duplicateBadgeOrderBody = @{
    idempotency_key = "smoke-mall-badge-duplicate-order-$stamp"
    items = @(@{
        product_id = $mallDigitalProductId
        quantity = 1
      })
  } | ConvertTo-Json -Depth 5
  Assert-ApiStatusMessage 412 "active badge entitlement already exists" -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $headers -ContentType "application/json" -Body $duplicateBadgeOrderBody -TimeoutSec 10
  $mallBadgeDuplicateOrderRejected = $true
  $mallDigitalProductAfterPaidOrder = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallDigitalProductId" -Method Get -TimeoutSec 10
  $mallDigitalGrantLockBody = @{
    sku = $mallDigitalProductSku
    title = "Smoke Badge Entitlement $stamp"
    description = "Smoke digital badge entitlement"
    category = "digital"
    cover_url = ""
    grant_type = "badge"
    grant_key = "$mallDigitalGrantKey-mutated"
    price_credits = $mallDigitalProductPrice
    stock = [int64]$mallDigitalProductAfterPaidOrder.product.stock
    status = 2
    sort = 102
  } | ConvertTo-Json
  Assert-ApiStatusMessage 412 "product grant cannot be changed after paid orders" -Uri "$baseUrl/api/v1/admin/mall/products/$mallDigitalProductId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $mallDigitalGrantLockBody -TimeoutSec 10
  $mallDigitalGrantLocked = $true
  $mallDigitalProductAfterGrantLock = Invoke-Api -Uri "$baseUrl/api/v1/mall/products/$mallDigitalProductId" -Method Get -TimeoutSec 10
  if ($mallDigitalProductAfterGrantLock.product.grant_type -ne "badge" -or $mallDigitalProductAfterGrantLock.product.grant_key -ne $mallDigitalGrantKey) {
    throw "Mall digital product grant changed after grant lock rejection"
  }
  $digitalRefundBody = @{
    reason = "smoke_digital_after_sale"
    note = "Smoke digital entitlement refund $stamp"
  } | ConvertTo-Json
  $createdDigitalRefund = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$digitalOrderId/refunds" -Method Post -Headers $headers -ContentType "application/json" -Body $digitalRefundBody -TimeoutSec 10
  $digitalRefundId = $createdDigitalRefund.refund.id
  if (-not $digitalRefundId -or [int64]$createdDigitalRefund.refund.status -ne 1 -or [int64]$createdDigitalRefund.refund.amount_credits -ne $mallDigitalProductPrice) {
    throw "Mall digital refund request did not return expected requested refund"
  }
  $reviewDigitalRefundBody = @{
    approved = $true
    admin_note = "Smoke digital refund approved"
    restore_stock = $true
  } | ConvertTo-Json
  $approvedDigitalRefund = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/refunds/$digitalRefundId/review" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $reviewDigitalRefundBody -TimeoutSec 10
  if ([int64]$approvedDigitalRefund.refund.status -ne 3) {
    throw "Admin mall digital refund approval did not approve refund"
  }
  $digitalOrderAfterRefund = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$digitalOrderId" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$digitalOrderAfterRefund.order.status -ne 8) {
    throw "Mall digital order did not move to refunded after refund approval"
  }
  $mallCreditAfterDigitalRefund = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
  if ([int64]$mallCreditAfterDigitalRefund.balance.total -ne [int64]$mallCreditBeforeDigitalPay.balance.total) {
    throw "Mall digital refund approval did not restore expected credit balance"
  }
  $mallRevokedDigitalEntitlements = Invoke-Api -Uri "$baseUrl/api/v1/mall/digital-entitlements?status=REVOKED&limit=50&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $mallDigitalEntitlementRevoked = $false
  foreach ($item in @($mallRevokedDigitalEntitlements.items)) {
    if ([string]$item.order_id -eq [string]$digitalOrderId -and [string]$item.product_id -eq [string]$mallDigitalProductId -and [string]$item.refund_id -eq [string]$digitalRefundId -and $item.status -eq "REVOKED" -and $item.grant_type -eq "badge" -and $item.grant_key -eq $mallDigitalGrantKey) {
      $mallDigitalEntitlementRevoked = $true
    }
  }
  if (-not $mallDigitalEntitlementRevoked) {
    throw "Mall digital entitlement list did not include revoked smoke grant after refund"
  }

  $mallNotifications = $null
  $mallNotificationTypes = @()
  $mallNotificationsReady = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $mallNotifications = Invoke-Api -Uri "$baseUrl/api/v1/notifications?limit=50&offset=0" -Method Get -Headers $headers -TimeoutSec 10
      $mallNotificationTypes = @($mallNotifications.items | ForEach-Object { $_.type })
      if (
        $mallNotificationTypes -contains "mall_order_shipped" -and
        $mallNotificationTypes -contains "mall_order_completed" -and
        $mallNotificationTypes -contains "mall_refund_approved" -and
        $mallNotificationTypes -contains "mall_refund_rejected"
      ) {
        $mallNotificationsReady = $true
        break
      }
    } catch {
    }
  }
  if (-not $mallNotificationsReady) {
    throw "Mall order/refund notifications were not projected from mall events"
  }

  $unfavoriteArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/favorite" -Method Delete -Headers $headers -TimeoutSec 10
  if ([int64]$unfavoriteArticle.count -ne 0) {
    throw "Article unfavorite did not decrement count"
  }
  $favoritesAfterUnfavorite = Invoke-Api -Uri "$baseUrl/api/v1/users/current/favorites?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $favoriteArticleRemoved = $true
  $favoriteTopicStillListed = $false
  foreach ($item in @($favoritesAfterUnfavorite.items)) {
    if ($item.entity.entity_type -eq "article" -and [string]$item.entity.entity_id -eq [string]$articleId) {
      $favoriteArticleRemoved = $false
    }
    if ($item.entity.entity_type -eq "topic" -and [string]$item.entity.entity_id -eq [string]$topicId) {
      $favoriteTopicStillListed = $true
    }
  }
  if (-not $favoriteArticleRemoved -or -not $favoriteTopicStillListed) {
    throw "Current user favorites did not reflect article unfavorite"
  }
  $unlikeArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/like" -Method Delete -Headers $headers -TimeoutSec 10
  if ([int64]$unlikeArticle.count -ne 0) {
    throw "Article unlike did not decrement count"
  }
  $likesAfterUnlike = Invoke-Api -Uri "$baseUrl/api/v1/users/current/likes?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $likeArticleRemoved = $true
  $likeTopicStillListed = $false
  foreach ($item in @($likesAfterUnlike.items)) {
    if ($item.entity.entity_type -eq "article" -and [string]$item.entity.entity_id -eq [string]$articleId) {
      $likeArticleRemoved = $false
    }
    if ($item.entity.entity_type -eq "topic" -and [string]$item.entity.entity_id -eq [string]$topicId) {
      $likeTopicStillListed = $true
    }
  }
  if (-not $likeArticleRemoved -or -not $likeTopicStillListed) {
    throw "Current user likes did not reflect article unlike"
  }

  [pscustomobject]@{
    gateway = $baseUrl
    authProviders = $authProviderNames
    oauthGithubMinYears = $githubAuthProvider.min_account_years
    webmasterEnabled = $authConfig.webmaster_enabled
    passwordChanged = $true
    passwordResetRequested = $passwordResetRequest.accepted
    passwordResetLinkApplied = $true
    emailVerificationRequested = $emailVerificationRequest.accepted
    emailVerificationLinkApplied = $verifiedSmokeUser.user.email_verified
    username = $username
    adminUsername = $adminUsername
    rbacRoleKeys = $roleKeys
    createdAdminId = $createdAdmin.user.id
    createdAdminRoles = $assignedAdmin.user.roles
    adminDashboardMetrics = @($adminOverview.metrics).Count
    adminDashboardDailyRows = @($adminOverview.daily).Count
    moderatorDashboardForbidden = $moderatorDashboardForbidden
    mallReadonlyMenuNames = $mallReadonlyMenuNames
    mallReadonlyProductsForbidden = $mallReadonlyProductsForbidden
    mallReadonlyCloseExpiredForbidden = $mallReadonlyCloseExpiredForbidden
    createdAdminDashboardMetrics = @($createdAdminOverview.metrics).Count
    userId = $me.user.id
    governanceUserListed = $governanceUserListed
    followeeId = $followeeId
    userSearchTotal = $userSearch.total
    userSearchListed = $userSearchListed
    followRoundTrip = -not $unfollowState.following
    refollowedForFeed = $refollowState.following
    categoryId = $categoryId
    categoryName = $categoryDetail.category.name
    links = @($links.items).Count
    tasks = @($tasks.items).Count
    adminBadges = @($adminBadges.items).Count
    adminLevels = @($adminLevels.items).Count
    publicLevels = @($publicLevels.items).Count
    adminForbiddenWords = @($adminForbiddenWords.items).Count
    adminSettings = @($adminSettings.items).Count
    adminEmailLogs = @($adminEmailLogs.items).Count
    adminLinks = @($adminLinks.items).Count
    adminTasks = @($adminTasks.items).Count
    permissionTopicArchivedStatus = $deletedPermissionTopic.topic.status
    permissionArticleArchivedStatus = $deletedPermissionArticle.article.status
    topicId = $topicId
    topicListed = $topicListed
    categoryTopicListed = $categoryTopicListed
    topicCommentId = $topicCommentId
    permissionCommentHidden = $permissionCommentHidden
    topicLikeCount = $topicReactionCounts.like_count
    topicFavoriteCount = $topicReactionCounts.favorite_count
    topicFeedProjected = $topicFeedProjected
    topicFeedLikeCount = $topicFeedItem.like_count
    topicFeedFavoriteCount = $topicFeedItem.favorite_count
    topicFeedCommentCount = $topicFeedItem.comment_count
    topicSearchIndexed = $topicSearchIndexed
    adminTopicListed = $adminTopicListed
    hiddenTopicStatus = $hiddenTopic.topic.status
    archivedTopicStatus = $archivedTopic.topic.status
    articleId = $articleId
    articleTitle = $articleDetail.article.title
    tags = $tagNames
    reportId = $report.report.id
    reportStatus = $auditedReport.report.status
    moderationArticleId = $moderationArticleId
    moderationArticleHiddenStatus = $hiddenArticle.article.status
    moderationArticleArchivedStatus = $archivedArticle.article.status
    moderationCommentId = $moderationCommentId
    moderationCommentHidden = $moderationCommentHidden
    mutedUserStatus = $mutedUser.user.status
    unmutedUserStatus = $unmutedUser.user.status
    commentId = $comment.comment.id
    likeCount = $like.count
    favoriteCount = $favorite.count
    currentUserLikes = @($likes.items).Count
    likeTopicListed = $likeTopicListed
    likeArticleListed = $likeArticleListed
    currentUserFavorites = @($favorites.items).Count
    favoriteTopicListed = $favoriteTopicListed
    favoriteArticleListed = $favoriteArticleListed
    reactionCacheVerified = $reactionCacheRebuild.verified
    reactionCacheLikesLoaded = $reactionCacheRebuild.likes_loaded
    reactionCacheFavoritesLoaded = $reactionCacheRebuild.favorites_loaded
    listedComments = @($comments.items).Count
    feedItems = @($feed.items).Count
    feedLikeCount = $feedItem.like_count
    feedFavoriteCount = $feedItem.favorite_count
    feedCommentCount = $feedItem.comment_count
    followingFeedItems = @($followingFeed.items).Count
    followingFeedArticleListed = $followingFeedArticleListed
    followingFeedOwnTopicExcluded = $followingFeedOwnTopicExcluded
    searchIndexed = $searchIndexed
    notificationTypes = $notificationTypes
    unreadBeforeRead = $unread.count
    unreadAfterRead = $afterRead.count
    unreadAfterReadAll = $afterReadAllCount
    actorCreditTotal = $actorCredit.balance.total
    authorCreditTotal = $authorCredit.balance.total
    actorCreditLedger = @($actorLedger.items).Count
    authorCreditLedger = @($authorLedger.items).Count
    mallCategoryId = $createdMallCategory.category.id
    mallCategoryListed = $publicMallCategoryListed
    mallProductId = $mallProductId
    mallProductListed = $publicMallProductListed
    mallDigitalProductId = $mallDigitalProductId
    mallDigitalProductListed = $publicMallDigitalProductListed
    mallMembershipProductListed = $mallMembershipProductListed
    mallMembershipProductId = $mallMembershipProductId
    mallMembershipOrderId = $membershipOrderId
    mallMembershipOrderStatus = $membershipOrderPaid.order.status
    mallMembershipEntitlementListed = $mallMembershipEntitlementListed
    mallMembershipProfileBackgroundStored = $membershipProfileBackgroundStored
    mallMembershipProfileBackgroundHiddenAfterRevoke = $membershipProfileBackgroundHiddenAfterRevoke
    mallMembershipQaDraftTopicId = $membershipQaDraftTopicId
    mallMembershipQaDraftPublishForbidden = $membershipQaDraftPublishForbidden
    mallMembershipQaDraftAdminPublishForbidden = $membershipQaDraftAdminPublishForbidden
    mallMembershipQaTopicId = $membershipQaTopicId
    mallMembershipQaAnswerId = $membershipQaAnswerId
    mallMembershipQaBountyReserved = $membershipQaBountyReserved
    mallMembershipQaBountySettled = $membershipQaBountySettled
    mallMembershipQaAnswerRewarded = $membershipQaAnswerRewarded
    mallMembershipQaRevocationTopicId = $membershipQaRevocationTopicId
    mallMembershipQaRevocationAnswerId = $membershipQaRevocationAnswerId
    mallMembershipQaAcceptForbiddenAfterRevoke = $membershipQaAcceptForbiddenAfterRevoke
    mallMembershipQaUnsettledAfterRevoke = $membershipQaUnsettledAfterRevoke
    mallMembershipRefundRejected = $membershipRefundRejected
    mallMembershipRevokeReason = $membershipRevokeReason
    mallMembershipOrderAfterRevokeStatus = $membershipOrderAfterRevoke.order.status
    mallMembershipEntitlementRevoked = $mallMembershipEntitlementRevoked
    mallThemeProductListed = $mallThemeProductListed
    mallThemeProductId = $mallThemeProductId
    mallThemeOrderId = $themeOrderId
    mallThemeOrderStatus = $themeOrderPaid.order.status
    mallThemeEntitlementListed = $mallThemeEntitlementListed
    mallThemeDuplicateOrderRejected = $mallThemeDuplicateOrderRejected
    mallThemePendingDuplicateOrderRejected = $mallThemePendingDuplicateOrderRejected
    mallBadgeDuplicateOrderRejected = $mallBadgeDuplicateOrderRejected
    mallBadgePendingDuplicateOrderRejected = $mallBadgePendingDuplicateOrderRejected
    mallThemeProfileThemeStored = $mallThemeProfileThemeStored
    mallThemeProfileThemeDefaultAfterRevoke = $mallThemeProfileThemeDefaultAfterRevoke
    mallThemeRevokeReason = $themeRevokeReason
    mallThemeEntitlementRevoked = $mallThemeEntitlementRevoked
    mallDigitalOrderId = $digitalOrderId
    mallDigitalOrderStatus = $digitalOrderPaid.order.status
    mallDigitalGrantKey = $mallDigitalGrantKey
    mallDigitalEntitlementListed = $mallDigitalEntitlementListed
    mallDigitalGrantLocked = $mallDigitalGrantLocked
    mallDigitalRefundId = $digitalRefundId
    mallDigitalOrderRefundedStatus = $digitalOrderAfterRefund.order.status
    mallDigitalEntitlementRevoked = $mallDigitalEntitlementRevoked
    mallCreditAfterDigitalRefund = $mallCreditAfterDigitalRefund.balance.total
    mallCouponId = $mallCouponId
    mallCouponListed = $publicMallCouponListed
    mallCouponTermsLocked = $mallCouponTermsLocked
    mallFavoriteListed = $mallFavoriteListed
    mallAddressId = $mallAddressId
    mallCartCheckedOut = $mallCartAfterCheckoutItems.Count -eq 0
    mallOrderId = $mallOrderId
    mallOrderTotalCredits = $mallOrderTotal
    mallPaidStatus = $mallPaid.order.status
    mallProductFulfillmentLocked = $mallProductFulfillmentLocked
    mallShippedStatus = $mallShipped.order.status
    mallCompletedStatus = $mallCompleted.order.status
    mallOrderLogs = @($mallOrderLogs.items).Count
    mallCouponUsageListed = $mallCouponUsageListed
    mallPaymentStatus = $firstMallPayment.status
    mallPaymentReplayIdempotent = $mallPaymentReplayIdempotent
    mallAdminOrderListed = $adminMallOrderListed
    mallReviewId = $mallReviewId
    mallReviewStatus = $publishedMallReview.review.status
    mallReviewEmailVerificationRejected = $mallReviewEmailVerificationRejected
    emailVerificationTopicUpdateRejected = $emailVerificationTopicUpdateRejected
    emailVerificationArticleUpdateRejected = $emailVerificationArticleUpdateRejected
    mallMyReviewListed = @($myPublishedMallReviews.items | Where-Object { [string]$_.id -eq [string]$mallReviewId }).Count -eq 1
    mallPublicReviewListed = @($publicMallReviews.items | Where-Object { [string]$_.id -eq [string]$mallReviewId }).Count -eq 1
    mallReviewHiddenAfterRefund = @($hiddenMallReviewsAfterRefund.items | Where-Object { [string]$_.id -eq [string]$mallReviewId }).Count -eq 1
    mallPublicReviewHiddenAfterRefund = @($publicMallReviewsAfterRefund.items | Where-Object { [string]$_.id -eq [string]$mallReviewId }).Count -eq 0
    mallConfirmBlockedDuringRefund = $mallConfirmBlockedDuringRefund
    mallAdminStatusUpdateBlockedDuringRefund = $mallAdminStatusUpdateBlockedDuringRefund
    mallReviewBlockedDuringRefund = $mallReviewBlockedDuringRefund
    mallReviewAllowedAfterRefundRejection = $mallReviewAllowedAfterRefundRejection
    mallReviewAfterRejectedRefundId = $reviewAfterRejectedRefund.review.id
    mallRefundId = $mallRefundId
    mallRefundStatus = $approvedMallRefund.refund.status
    mallOrderRefundedStatus = $mallOrderAfterRefund.order.status
    mallStockAfterRefund = $mallProductAfterRefund.product.stock
    mallStockLogs = @($mallProductStockLogs.items).Count
    mallRejectedRefundId = $rejectRefundId
    mallRejectedRefundStatus = $rejectedMallRefund.refund.status
    mallRejectOrderStatus = $rejectOrderAfterReview.order.status
    mallStockAfterReject = $productAfterRejectReview.product.stock
    mallCreditAfterReject = $creditAfterRejectReview.balance.total
    mallInsufficientOrderId = $expensiveOrderId
    mallInsufficientOrderStatusAfterPay = $expensiveOrderAfterFailedPay.order.status
    mallInsufficientOrderCanceledStatus = $expensiveOrderCanceled.order.status
    mallInsufficientPaymentFailed = $failedExpensivePaymentListed
    mallInsufficientPaymentFailedLog = $paymentFailedLogListed
    mallInsufficientStockAfterCancel = $expensiveProductAfterCancel.product.stock
    mallExpiredOrderId = $expiringOrderId
    mallExpiredOrderClosedStatus = $expiringOrderAfterClose.order.status
    mallExpiredOrderStockAfterClose = $expiringProductAfterClose.product.stock
    mallExpiredOrderLogListed = $expiredOrderLogListed
    mallExpiredStockLogListed = $expiredStockLogListed
    mallCreditAfterPay = $mallCreditAfterPay.balance.total
    mallCreditAfterRefund = $mallCreditAfterRefund.balance.total
    mallOverviewOrderTotal = $mallOverview.overview.order_total
    mallOverviewRefundedCreditsTotal = $mallOverview.overview.refunded_credits_total
    mallOverviewNetRevenueCreditsTotal = $mallOverview.overview.net_revenue_credits_total
    mallOverviewFailedPaymentTotal = $mallOverviewAfterFailedPayment.overview.failed_payment_total
    mallNotificationTypes = $mallNotificationTypes
    mallNotificationsProjected = $mallNotificationsReady
    favoriteArticleRemoved = $favoriteArticleRemoved
    favoriteTopicStillListed = $favoriteTopicStillListed
    likeArticleRemoved = $likeArticleRemoved
    likeTopicStillListed = $likeTopicStillListed
  } | ConvertTo-Json -Depth 4
} finally {
  if (-not $KeepRunning) {
    Stop-StartedProcesses
  }
}
