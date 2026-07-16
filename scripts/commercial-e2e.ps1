param(
  [int]$GatewayPort = 18080,
  [int]$MallPort = 0,
  [switch]$SkipBuild,
  [switch]$SkipBackend,
  [switch]$SkipInfraCheck,
  [switch]$SkipFrontend,
  [switch]$SkipAdmin,
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

function Assert-LocalInfrastructure {
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

  if ($missing.Count -gt 0) {
    $hint = @(
      "Commercial E2E requires local infrastructure before backend smoke.",
      "Missing or unhealthy:",
      ($missing | ForEach-Object { "  - $_" }),
      "",
      "Typical local startup:",
      "  cd backend\deployments\local",
      "  docker compose --profile comments --profile events --profile search up -d",
      "  .\scripts\bootstrap.ps1 -Comments -Events -Search",
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
if ($ProjectionRetries -lt 1) {
  throw "ProjectionRetries must be greater than 0"
}

if (-not $SkipBackend) {
  if (-not $SkipInfraCheck) {
    Invoke-Step "local infrastructure preflight" {
      Assert-LocalInfrastructure
    }
  }

  Invoke-Step "backend commercial smoke" {
    $smokeArgs = @{
      GatewayPort = $GatewayPort
      KeepRunning = $true
      ProjectionRetries = $ProjectionRetries
    }
    if ($MallPort -gt 0) {
      $smokeArgs.MallPort = $MallPort
    }
    if ($SkipBuild) {
      $smokeArgs.SkipBuild = $true
    }
    & (Join-Path $RepoRoot "backend\scripts\smoke-local.ps1") @smokeArgs
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
    & (Join-Path $RepoRoot "backend\scripts\check-local-backend.ps1") @checkArgs
  }
}

if (-not $SkipFrontend) {
  Invoke-Step "frontend mall e2e" {
    Invoke-WithEnv @{ API_BASE = $ApiBase; VITE_API_BASE = $ApiBase } {
      Push-Location (Join-Path $RepoRoot "frontend")
      try {
        npm run e2e:mall
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
      } finally {
        Pop-Location
      }
    }
  }
}

Write-Host ""
Write-Host "Commercial E2E completed against $ApiBase"
