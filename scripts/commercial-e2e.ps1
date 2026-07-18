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
  [string]$MinIOEndpoint = "http://127.0.0.1:9000/minio/health/live",
  [string]$MinIOStorageEndpoint = "",
  [string]$MinIOContainer = "bbs-local-minio",
  [string]$MinIOBucket = "bbs-local",
  [string]$MinIOAccessKey = "minioadmin",
  [string]$MinIOSecretKey = "minioadmin",
  [int]$ProjectionRetries = 60
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$ApiBase = "http://127.0.0.1:$GatewayPort/api/v1"

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
    @{ Name = "MongoDB"; Host = "127.0.0.1"; Port = 27017 }
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
      "Typical local startup:",
      "  cd backend\deployments\local",
      "  docker compose --profile comments --profile events --profile search --profile files up -d",
      "  .\scripts\bootstrap.ps1 -Full",
      "",
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

$resolvedMinIOStorageEndpoint = ""
if (-not $SkipAttachments) {
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
