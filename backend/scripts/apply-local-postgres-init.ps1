param(
  [string]$HostName = "127.0.0.1",
  [int]$Port = 5432,
  [string]$Database = "bbs",
  [string]$SuperUser = "postgres",
  [string]$SuperPassword = ""
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$InitSql = Join-Path $RepoRoot "backend\deployments\local\postgres\init\001-create-database-and-schemas.sql"

if (-not (Test-Path $InitSql)) {
  throw "PostgreSQL init SQL not found: $InitSql"
}

$psql = Get-Command psql -ErrorAction SilentlyContinue
if ($null -eq $psql) {
  throw "psql not found in PATH"
}

if ($SuperPassword) {
  $env:PGPASSWORD = $SuperPassword
}

& $psql.Source -h $HostName -p $Port -U $SuperUser -d $Database -v ON_ERROR_STOP=1 -f $InitSql
if ($LASTEXITCODE -ne 0) {
  throw "PostgreSQL init failed with exit code $LASTEXITCODE"
}

Write-Host "PostgreSQL local init applied: $InitSql"
