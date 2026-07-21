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
  throw "Refusing to reset an unexpected Compose project: '$projectName'."
}

$mailpitContainerIDs = @(docker compose ps --all --quiet mailpit | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
if ($LASTEXITCODE -ne 0) {
  throw "Could not inspect the BBS Mailpit Compose service."
}

if ($mailpitContainerIDs.Count -eq 0) {
  Write-Host "BBS Mailpit is not present; no container was removed."
} else {
  Write-Host "Stopping and removing only the BBS Mailpit container..."
  docker compose rm --stop --force mailpit
  if ($LASTEXITCODE -ne 0) {
    throw "Could not remove the BBS Mailpit container."
  }
}
Write-Host "Reset complete."
