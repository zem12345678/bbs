param(
  [string[]]$Services = @(
    "admin-service",
    "user-service",
    "content-service",
    "comment-service",
    "credit-service",
    "file-service",
    "mall-service",
    "notification-service",
    "reaction-service",
    "chat-service"
  )
)

$ErrorActionPreference = "Stop"

$BackendRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..")).Path
$ServicesRoot = Join-Path $BackendRoot "services"

foreach ($serviceName in $Services) {
  $serviceDir = Join-Path $ServicesRoot $serviceName
  if (-not (Test-Path (Join-Path $serviceDir "go.mod"))) {
    throw "Unknown BBS service '$serviceName'."
  }

  Push-Location $serviceDir
  try {
    Write-Host "Migrating $serviceName..."
    & go run ./cmd migrate -c configs/config.yaml
    if ($LASTEXITCODE -ne 0) {
      throw "$serviceName migration failed with exit code $LASTEXITCODE."
    }
  } finally {
    Pop-Location
  }
}

Write-Host "Local BBS migrations completed."
