param(
  [int]$GatewayPort = 18080,
  [int]$MallPort = 0,
  [int]$SearchPort = 0,
  [switch]$SkipBuild,
  [switch]$SkipBackend,
  [switch]$SkipInfraCheck,
  [switch]$SkipFrontend,
  [switch]$SkipAdmin,
  [switch]$SkipAttachments,
  [string]$MinIOEndpoint = "",
  [string]$MinIOStorageEndpoint = "",
  [string]$MinIOContainer = "",
  [string]$MinIOBucket = "",
  [string]$MinIOAccessKey = "",
  [string]$MinIOSecretKey = "",
  [int]$ProjectionRetries = 60
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$ApiBase = "http://127.0.0.1:$GatewayPort/api/v1"
$minIOHealthEndpointProvided = $PSBoundParameters.ContainsKey("MinIOEndpoint")

function Invoke-Step {
  param(
    [string]$Name,
    [scriptblock]$Block
  )

  Write-Host ""
  Write-Host "==> $Name"
  & $Block
}

function Invoke-WithEnv {
  param(
    [hashtable]$Values,
    [scriptblock]$Block
  )

  $previous = @{}
  foreach ($key in $Values.Keys) {
    $previous[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
    [Environment]::SetEnvironmentVariable($key, [string]$Values[$key], "Process")
  }

  try {
    & $Block
  } finally {
    foreach ($key in $Values.Keys) {
      [Environment]::SetEnvironmentVariable($key, $previous[$key], "Process")
    }
  }
}

function Import-ProcessEnvironmentFile {
  param([string]$Path)

  if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
    return
  }
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
    if (($content.StartsWith('"') -and $content.EndsWith('"')) -or ($content.StartsWith("'") -and $content.EndsWith("'"))) {
      $content = $content.Substring(1, $content.Length - 2)
    }
    [Environment]::SetEnvironmentVariable($name, $content, "Process")
  }
}

function Resolve-MinIOValue {
  param(
    [string]$ExplicitValue,
    [string]$EnvironmentName
  )

  if (-not [string]::IsNullOrWhiteSpace($ExplicitValue)) {
    return $ExplicitValue.Trim()
  }
  $value = [Environment]::GetEnvironmentVariable($EnvironmentName, "Process")
  if ([string]::IsNullOrWhiteSpace($value)) {
    return ""
  }
  return $value.Trim()
}

function Test-TcpEndpoint {
  param(
    [string]$HostName,
    [int]$Port,
    [int]$TimeoutMilliseconds = 500
  )

  $client = [System.Net.Sockets.TcpClient]::new()
  try {
    $async = $client.BeginConnect($HostName, $Port, $null, $null)
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

function Test-HttpEndpoint {
  param([string]$Url)

  try {
    Invoke-RestMethod -Uri $Url -Method Get -TimeoutSec 3 | Out-Null
    return $true
  } catch {
    return $false
  }
}

function Test-ElasticsearchEndpoint {
  param([string]$Url)

  try {
    $health = Invoke-RestMethod -Uri $Url -Method Get -TimeoutSec 3
    return @("yellow", "green") -contains [string]$health.status
  } catch {
    return $false
  }
}

function Resolve-MinIOStorageEndpoint {
  param(
    [string]$HealthEndpoint,
    [string]$ExplicitEndpoint
  )

  $source = if ([string]::IsNullOrWhiteSpace($ExplicitEndpoint)) { $HealthEndpoint } else { $ExplicitEndpoint }
  $uri = $null
  if (-not [Uri]::TryCreate($source, [UriKind]::Absolute, [ref]$uri) -or $uri.Scheme -notin @("http", "https") -or [string]::IsNullOrWhiteSpace($uri.Host)) {
    throw "MinIO storage endpoint must be an absolute HTTP(S) URL"
  }
  if (-not [string]::IsNullOrWhiteSpace($ExplicitEndpoint) -and $uri.AbsolutePath -notin @("", "/")) {
    throw "MinIOStorageEndpoint must not contain a path"
  }
  return $uri.GetLeftPart([UriPartial]::Authority)
}

function Assert-LocalInfrastructure {
  param([switch]$RequireMinIO)

  $missing = New-Object System.Collections.Generic.List[string]

  foreach ($item in @(
    @{ Name = "PostgreSQL"; Host = "127.0.0.1"; Port = 5432 },
    @{ Name = "Redis"; Host = "127.0.0.1"; Port = 6379 },
    @{ Name = "Kafka"; Host = "127.0.0.1"; Port = 9092 },
    @{ Name = "etcd"; Host = "127.0.0.1"; Port = 2379 },
    @{ Name = "MongoDB"; Host = "127.0.0.1"; Port = 27017 },
    @{ Name = "Mailpit SMTP"; Host = "127.0.0.1"; Port = 1025 },
    @{ Name = "Mailpit HTTP"; Host = "127.0.0.1"; Port = 8025 }
  )) {
    if (-not (Test-TcpEndpoint $item.Host $item.Port)) {
      $missing.Add("$($item.Name) $($item.Host):$($item.Port)")
    }
  }

  if (-not (Test-HttpEndpoint "http://127.0.0.1:8848/nacos/")) {
    $missing.Add("Nacos http://127.0.0.1:8848/nacos/")
  }
  if (-not (Test-ElasticsearchEndpoint "http://127.0.0.1:9200/_cluster/health")) {
    $missing.Add("Elasticsearch http://127.0.0.1:9200/_cluster/health")
  }
  if ($RequireMinIO -and -not (Test-HttpEndpoint $MinIOEndpoint)) {
    $missing.Add("MinIO $MinIOEndpoint")
  }

  if ($missing.Count -gt 0) {
    $hint = @(
      "Commercial E2E requires local infrastructure before backend smoke.",
      "Missing or unhealthy:",
      ($missing | ForEach-Object { "  - $_" }),
      "",
      "Typical local preparation:",
      "  cd backend\deployments\local",
      "  docker compose up -d # Starts only BBS Mailpit.",
      "  .\scripts\bootstrap.ps1 -Full",
      "",
      "PostgreSQL, Redis, Kafka, etcd, MongoDB, Nacos, Elasticsearch, and MinIO must already be running.",
      "Pass -SkipInfraCheck only when these dependencies are provided elsewhere."
    ) -join [Environment]::NewLine
    throw $hint
  }

  Write-Host "local infrastructure ready"
}

if ($GatewayPort -le 0) {
  throw "GatewayPort must be greater than 0"
}
if ($MallPort -lt 0) {
  throw "MallPort must be greater than or equal to 0"
}
if ($SearchPort -lt 0) {
  throw "SearchPort must be greater than or equal to 0"
}
if ($ProjectionRetries -lt 1) {
  throw "ProjectionRetries must be greater than 0"
}

$localEnvironmentFile = Join-Path $RepoRoot "backend\deployments\local\.env"
Import-ProcessEnvironmentFile $localEnvironmentFile

$resolvedMinIOStorageEndpoint = ""
if (-not $SkipAttachments) {
  if ([string]::IsNullOrWhiteSpace($MinIOStorageEndpoint) -and -not $minIOHealthEndpointProvided) {
    $MinIOStorageEndpoint = Resolve-MinIOValue $MinIOStorageEndpoint "MINIO_ENDPOINT"
  }
  if ([string]::IsNullOrWhiteSpace($MinIOEndpoint)) {
    if ([string]::IsNullOrWhiteSpace($MinIOStorageEndpoint)) {
      throw "Set MINIO_ENDPOINT in $localEnvironmentFile or pass -MinIOEndpoint."
    }
    $MinIOEndpoint = "$($MinIOStorageEndpoint.TrimEnd('/'))/minio/health/live"
  }
  $MinIOContainer = Resolve-MinIOValue $MinIOContainer "MINIO_CONTAINER"
  $MinIOBucket = Resolve-MinIOValue $MinIOBucket "MINIO_BUCKET"
  $MinIOAccessKey = Resolve-MinIOValue $MinIOAccessKey "MINIO_ACCESS_KEY"
  $MinIOSecretKey = Resolve-MinIOValue $MinIOSecretKey "MINIO_SECRET_KEY"
  foreach ($required in @{
    MINIO_CONTAINER = $MinIOContainer
    MINIO_BUCKET = $MinIOBucket
    MINIO_ACCESS_KEY = $MinIOAccessKey
    MINIO_SECRET_KEY = $MinIOSecretKey
  }.GetEnumerator()) {
    if ([string]::IsNullOrWhiteSpace([string]$required.Value)) {
      throw "Set $($required.Key) in $localEnvironmentFile or pass the matching MinIO parameter."
    }
  }
  $resolvedMinIOStorageEndpoint = Resolve-MinIOStorageEndpoint $MinIOEndpoint $MinIOStorageEndpoint
}

if (-not $SkipBackend) {
  if (-not $SkipInfraCheck) {
    Invoke-Step "local infrastructure preflight" {
      Assert-LocalInfrastructure -RequireMinIO:(-not $SkipAttachments)
    }
  }

  Invoke-Step "backend commercial smoke" {
    $smokeArgs = @{
      GatewayPort = $GatewayPort
      KeepRunning = $true
      RefreshRunningServices = $true
      ProjectionRetries = $ProjectionRetries
    }
    if ($MallPort -gt 0) {
      $smokeArgs.MallPort = $MallPort
    }
    if ($SearchPort -gt 0) {
      $smokeArgs.SearchPort = $SearchPort
    }
    if ($SkipBuild) {
      $smokeArgs.SkipBuild = $true
    }
    if ($SkipAttachments) {
      & (Join-Path $RepoRoot "backend\scripts\smoke-local.ps1") @smokeArgs
    } else {
      Invoke-WithEnv @{
        BBS_GATEWAY_STORAGE_ENDPOINT = $resolvedMinIOStorageEndpoint
        BBS_GATEWAY_STORAGE_BUCKET = $MinIOBucket
        BBS_GATEWAY_STORAGE_ACCESS_KEY = $MinIOAccessKey
        BBS_GATEWAY_STORAGE_SECRET_KEY = $MinIOSecretKey
      } {
        & (Join-Path $RepoRoot "backend\scripts\smoke-local.ps1") @smokeArgs
      }
    }
  }

  Invoke-Step "backend service status" {
    $checkArgs = @{
      Profile = "commercial"
      GatewayPort = $GatewayPort
      RequireDiscovery = $true
      Strict = $true
    }
    if ($MallPort -gt 0) {
      $checkArgs.MallPort = $MallPort
    }
    if ($SearchPort -gt 0) {
      $checkArgs.SearchPort = $SearchPort
    }
    & (Join-Path $RepoRoot "backend\scripts\check-local-backend.ps1") @checkArgs
  }

  if (-not $SkipAttachments) {
    Invoke-Step "paid attachment smoke" {
      $attachmentArgs = @{
        BaseUrl = "http://127.0.0.1:$GatewayPort"
        MinIOContainer = $MinIOContainer
        MinIOEndpoint = $resolvedMinIOStorageEndpoint
        MinIOBucket = $MinIOBucket
        MinIOAccessKey = $MinIOAccessKey
        MinIOSecretKey = $MinIOSecretKey
      }
      & (Join-Path $RepoRoot "backend\scripts\attachment-smoke.ps1") @attachmentArgs
    }
  }
}

if (-not $SkipFrontend) {
  Invoke-Step "frontend mall e2e" {
    Invoke-WithEnv @{ API_BASE = $ApiBase; VITE_API_BASE = $ApiBase; MALL_E2E_ATTACHMENTS = if ($SkipAttachments) { "0" } else { "1" } } {
      Push-Location (Join-Path $RepoRoot "frontend")
      try {
        npm run e2e:mall
        if ($LASTEXITCODE -ne 0) {
          throw "frontend mall e2e failed with exit code $LASTEXITCODE"
        }
      } finally {
        Pop-Location
      }
    }
  }
}

if (-not $SkipAdmin) {
  Invoke-Step "admin mall e2e" {
    Invoke-WithEnv @{ API_BASE = $ApiBase; VITE_API_BASE = $ApiBase } {
      Push-Location (Join-Path $RepoRoot "vue-pure-admin")
      try {
        pnpm e2e:mall
        if ($LASTEXITCODE -ne 0) {
          throw "admin mall e2e failed with exit code $LASTEXITCODE"
        }
      } finally {
        Pop-Location
      }
    }
  }
}

Write-Host ""
Write-Host "Commercial E2E completed against $ApiBase"
