param(
  [switch]$Confirm
)

$ErrorActionPreference = "Stop"

if (-not $Confirm) {
  throw "Refusing to reset local infrastructure without -Confirm."
}

$LocalRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $LocalRoot

$projectName = "bbs-local"
if (Test-Path .\.env) {
  Get-Content .\.env | ForEach-Object {
    if ($_ -match "^COMPOSE_PROJECT_NAME=(.+)$") {
      $projectName = $Matches[1].Trim()
    }
  }
}

if ([string]::IsNullOrWhiteSpace($projectName) -or $projectName -ne "bbs-local") {
  throw "Refusing to delete volumes for unexpected Compose project: '$projectName'."
}

Write-Host "Volumes currently owned by ${projectName}:"
docker volume ls --filter "label=com.docker.compose.project=$projectName" --format "  {{.Name}}"

Write-Host "Stopping current Compose services and deleting their declared volumes..."
docker compose down -v
Write-Host "Reset complete."
