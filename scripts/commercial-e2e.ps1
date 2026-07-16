param(
  [int]$GatewayPort = 18080,
  [int]$MallPort = 0,
  [switch]$SkipBuild,
  [switch]$SkipBackend,
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
