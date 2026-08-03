param(
  [switch]$Full,
  [switch]$Events,
  [switch]$Comments,
  [switch]$Search,
  [switch]$Files,
  [switch]$Mail,
  [string]$PostgresHost = "127.0.0.1",
  [int]$PostgresPort = 5432,
  [string]$PostgresUser = "postgres",
  [string]$PostgresDatabase = "bbs"
)

$ErrorActionPreference = "Stop"

$LocalRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $LocalRoot

function Import-LocalEnvironment {
  $environmentFile = Join-Path $LocalRoot ".env"
  if (-not (Test-Path -LiteralPath $environmentFile)) {
    return
  }

  foreach ($line in Get-Content -LiteralPath $environmentFile) {
    $value = $line.Trim()
    if ($value.Length -eq 0 -or $value.StartsWith("#")) {
      continue
    }
    $separator = $value.IndexOf("=")
    if ($separator -lt 1) {
      throw "Invalid environment entry in ${environmentFile}: $line"
    }
    $name = $value.Substring(0, $separator).Trim()
    $content = $value.Substring($separator + 1).Trim()
    if (($content.StartsWith('"') -and $content.EndsWith('"')) -or ($content.StartsWith("'") -and $content.EndsWith("'"))) {
      $content = $content.Substring(1, $content.Length - 2)
    }
    [Environment]::SetEnvironmentVariable($name, $content, "Process")
  }
}

function Get-LocalEnvironmentValue {
  param(
    [string]$Name,
    [string]$Default = ""
  )

  $value = [Environment]::GetEnvironmentVariable($Name, "Process")
  if ([string]::IsNullOrWhiteSpace($value)) {
    return $Default
  }
  return $value.Trim()
}

function Get-RequiredLocalEnvironmentValue {
  param([string]$Name)

  $value = Get-LocalEnvironmentValue -Name $Name
  if ([string]::IsNullOrWhiteSpace($value)) {
    throw "$Name must be set in $LocalRoot\.env before publishing local Nacos configs."
  }
  if ($value -match "[\r\n]") {
    throw "$Name must not contain newlines."
  }
  if ($value -match "(?i)^(replace-with|change-me|example-|your-)") {
    throw "$Name must be replaced with a real local value in $LocalRoot\.env."
  }
  return $value
}

function ConvertTo-YamlSingleQuotedScalar {
  param([string]$Value)

  return "'" + $Value.Replace("'", "''") + "'"
}

function Resolve-NacosConfigContent {
  param([System.IO.FileInfo]$File)

  $content = Get-Content -Raw -LiteralPath $File.FullName
  $replacements = @{
    "__BBS_LOCAL_MINIO_ENDPOINT__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "MINIO_ENDPOINT")
    "__BBS_LOCAL_MINIO_BUCKET__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "MINIO_BUCKET")
    "__BBS_LOCAL_MINIO_ACCESS_KEY__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "MINIO_ACCESS_KEY")
    "__BBS_LOCAL_MINIO_SECRET_KEY__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "MINIO_SECRET_KEY")
    "__BBS_LOCAL_GATEWAY_JWT_SECRET__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_GATEWAY_JWT_SECRET")
    "__BBS_LOCAL_USER_JWT_SECRET__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_USER_JWT_SECRET")
    "__BBS_LOCAL_USER_MFA_ENCRYPTION_KEY__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_USER_MFA_ENCRYPTION_KEY")
    "__BBS_LOCAL_ADMIN_JWT_SECRET__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_ADMIN_JWT_SECRET")
    "__BBS_LOCAL_ADMIN_DEFAULT_PASSWORD__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_ADMIN_DEFAULT_PASSWORD")
    "__BBS_LOCAL_ADMIN_SECRET_ENCRYPTION_KEY__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_ADMIN_SECRET_ENCRYPTION_KEY")
    "__BBS_LOCAL_ADMIN_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_ADMIN_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_USER_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_USER_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_CHAT_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_CHAT_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_COMMENT_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_COMMENT_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_CONTENT_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_CONTENT_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_CREDIT_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_CREDIT_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_MALL_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_MALL_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_FILE_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_FILE_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_FEED_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_FEED_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_REACTION_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_REACTION_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_NOTIFICATION_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_NOTIFICATION_INTERNAL_AUTH_TOKEN")
    "__BBS_LOCAL_SEARCH_INTERNAL_AUTH_TOKEN__" = ConvertTo-YamlSingleQuotedScalar (Get-RequiredLocalEnvironmentValue "BBS_LOCAL_SEARCH_INTERNAL_AUTH_TOKEN")
  }
  foreach ($token in $replacements.Keys) {
    $content = $content.Replace($token, $replacements[$token])
  }
	if ($content -match "__BBS_LOCAL_[A-Z0-9_]+__") {
		throw "Unresolved local secret placeholder in $($File.FullName)."
  }
  return $content
}

if ($Full) {
  $Events = $true
  $Comments = $true
  $Search = $true
  $Files = $true
  $Mail = $true
}

function Wait-Tcp {
  param(
    [string]$HostName,
    [int]$Port,
    [string]$Name,
    [int]$Retries = 60
  )

  for ($i = 1; $i -le $Retries; $i++) {
    try {
      $client = [System.Net.Sockets.TcpClient]::new()
      $async = $client.BeginConnect($HostName, $Port, $null, $null)
      if ($async.AsyncWaitHandle.WaitOne(1000)) {
        $client.EndConnect($async)
        $client.Close()
        Write-Host "ready: $Name ($HostName`:$Port)"
        return
      }
      $client.Close()
    } catch {
      Start-Sleep -Seconds 1
    }
  }

  throw "Timed out waiting for $Name at $HostName`:$Port"
}

function Wait-Http {
  param(
    [string]$Url,
    [string]$Name,
    [int]$Retries = 60
  )

  for ($i = 1; $i -le $Retries; $i++) {
    try {
      Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 2 | Out-Null
      Write-Host "ready: $Name ($Url)"
      return
    } catch {
      Start-Sleep -Seconds 1
    }
  }

  throw "Timed out waiting for $Name at $Url"
}

function Publish-NacosConfig {
  param(
    [string]$NacosUrl,
    [string]$Namespace,
    [string]$Group,
    [string]$FilePath
  )

  $file = Get-Item $FilePath
  $content = Resolve-NacosConfigContent -File $file
  $body = @{
    dataId = $file.Name
    group = $Group
    tenant = $Namespace
    content = $content
    type = "yaml"
  }

  Invoke-RestMethod -Method Post -Uri "$NacosUrl/nacos/v1/cs/configs" -Body $body | Out-Null
  Write-Host "nacos config: $($file.Name)"
}

Import-LocalEnvironment

# Validate every value injected into the checked-in Nacos templates before
# touching PostgreSQL or publishing any configuration. This keeps a partially
# bootstrapped local environment from being created when .env still contains
# placeholders or is incomplete.
@(
  "MINIO_ENDPOINT",
  "MINIO_BUCKET",
  "MINIO_ACCESS_KEY",
  "MINIO_SECRET_KEY",
  "BBS_LOCAL_GATEWAY_JWT_SECRET",
  "BBS_LOCAL_USER_JWT_SECRET",
  "BBS_LOCAL_USER_MFA_ENCRYPTION_KEY",
  "BBS_LOCAL_ADMIN_JWT_SECRET",
  "BBS_LOCAL_ADMIN_DEFAULT_PASSWORD",
  "BBS_LOCAL_ADMIN_SECRET_ENCRYPTION_KEY",
  "BBS_LOCAL_ADMIN_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_USER_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_CHAT_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_COMMENT_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_CONTENT_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_CREDIT_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_MALL_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_FILE_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_FEED_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_REACTION_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_NOTIFICATION_INTERNAL_AUTH_TOKEN",
  "BBS_LOCAL_SEARCH_INTERNAL_AUTH_TOKEN"
) | ForEach-Object {
  Get-RequiredLocalEnvironmentValue -Name $_ | Out-Null
}

$nacosUrl = (Get-LocalEnvironmentValue -Name "NACOS_URL" -Default "http://127.0.0.1:8848").TrimEnd("/")
$nacosUri = $null
if (-not [Uri]::TryCreate($nacosUrl, [UriKind]::Absolute, [ref]$nacosUri) -or $nacosUri.Scheme -notin @("http", "https") -or [string]::IsNullOrWhiteSpace($nacosUri.Host)) {
  throw "NACOS_URL must be an absolute http or https URL."
}
$nacosPort = if ($nacosUri.IsDefaultPort) { if ($nacosUri.Scheme -eq "https") { 443 } else { 80 } } else { $nacosUri.Port }
$nacosNamespace = Get-LocalEnvironmentValue -Name "NACOS_NAMESPACE" -Default "bbs-local"
$nacosGroup = Get-LocalEnvironmentValue -Name "NACOS_GROUP" -Default "BBS_LOCAL"

Write-Host "Bootstrapping BBS local infrastructure..."

Wait-Tcp $PostgresHost $PostgresPort "PostgreSQL"
Wait-Tcp 127.0.0.1 6379 "Redis"
Wait-Tcp 127.0.0.1 2379 "etcd"
Wait-Tcp $nacosUri.Host $nacosPort "Nacos"

# Check every selected external dependency before changing PostgreSQL or
# publishing Nacos configuration. In particular, -Full must not leave a
# partially initialized local environment when an optional dependency is down.
if ($Events) {
  Wait-Tcp 127.0.0.1 9092 "Kafka"
}

if ($Comments) {
  Wait-Tcp 127.0.0.1 27017 "MongoDB"
}

if ($Search) {
  $elasticsearchUrl = (Get-LocalEnvironmentValue -Name "ELASTICSEARCH_URL" -Default "http://127.0.0.1:9200").TrimEnd("/")
  Wait-Http "$elasticsearchUrl/_cluster/health" "Elasticsearch"
}

if ($Files) {
  $minioEndpoint = (Get-RequiredLocalEnvironmentValue "MINIO_ENDPOINT").TrimEnd("/")
  Wait-Http "$minioEndpoint/minio/health/live" "MinIO"
}

if ($Mail) {
  Wait-Tcp 127.0.0.1 1025 "Mailpit SMTP"
  Wait-Tcp 127.0.0.1 8025 "Mailpit HTTP"
}

Write-Host "Applying PostgreSQL schemas and local app users..."
if ($PostgresDatabase -notmatch "^[A-Za-z_][A-Za-z0-9_]*$") {
  throw "PostgresDatabase must be a simple PostgreSQL identifier: $PostgresDatabase"
}

$databaseExists = & psql --host $PostgresHost --port $PostgresPort --username $PostgresUser --dbname postgres --tuples-only --no-align --command "SELECT 1 FROM pg_database WHERE datname = '$PostgresDatabase'"
if ($LASTEXITCODE -ne 0) {
  throw "Could not check local PostgreSQL database. Set PGPASSWORD before running this script if the server requires a password."
}
if (@($databaseExists | Where-Object { $_.Trim() -eq "1" }).Count -eq 0) {
  $createDatabaseSql = "CREATE DATABASE `"$PostgresDatabase`""
  & psql --host $PostgresHost --port $PostgresPort --username $PostgresUser --dbname postgres --command $createDatabaseSql
  if ($LASTEXITCODE -ne 0) {
    throw "Could not create local PostgreSQL database '$PostgresDatabase'."
  }
  Write-Host "postgres database created: $PostgresDatabase"
}

$initScript = Join-Path $LocalRoot "postgres\init\001-create-database-and-schemas.sql"
& psql --host $PostgresHost --port $PostgresPort --username $PostgresUser --dbname $PostgresDatabase --file $initScript
if ($LASTEXITCODE -ne 0) {
  throw "Local PostgreSQL initialization failed. Set PGPASSWORD before running this script if the server requires a password."
}

Write-Host "Preparing Nacos namespace/configs..."
try {
  Invoke-RestMethod -Method Post -Uri "$nacosUrl/nacos/v1/console/namespaces" -Body @{
    customNamespaceId = $nacosNamespace
    namespaceName = $nacosNamespace
    namespaceDesc = "BBS local development"
  } | Out-Null
  Write-Host "nacos namespace: $nacosNamespace"
} catch {
  Write-Host "nacos namespace exists or API skipped: $nacosNamespace"
}

Get-ChildItem -LiteralPath .\nacos\configs -Filter *.yaml | Sort-Object Name | ForEach-Object {
  Publish-NacosConfig -NacosUrl $nacosUrl -Namespace $nacosNamespace -Group $nacosGroup -FilePath $_.FullName
}

if ($Events) {
  Write-Host "Kafka is external; bootstrap does not create, delete, or alter its topics."
}

if ($Comments) {
  Write-Host "MongoDB is external; comment-service ensures its required indexes on startup."
}

if ($Search) {
  Write-Host "Creating Elasticsearch indices..."
  Get-ChildItem .\elasticsearch -Filter *.mapping.json | Sort-Object Name | ForEach-Object {
    $index = $_.Name.Replace(".mapping.json", "")
    $exists = $false
    try {
      Invoke-WebRequest -Method Head -Uri "$elasticsearchUrl/$index" -UseBasicParsing | Out-Null
      $exists = $true
    } catch {
      $exists = $false
    }

    if ($exists) {
      Write-Host "elasticsearch index exists: $index"
    } else {
      $body = Get-Content -Raw -LiteralPath $_.FullName
      Invoke-RestMethod -Method Put -Uri "$elasticsearchUrl/$index" -ContentType "application/json" -Body $body | Out-Null
      Write-Host "elasticsearch index created: $index"
    }
  }
}

if ($Files) {
  Write-Host "MinIO is external; bucket '$((Get-RequiredLocalEnvironmentValue "MINIO_BUCKET"))' is created on first upload."
}

Write-Host ""
Write-Host "Local infra bootstrap complete."
Write-Host "Core endpoints:"
Write-Host "  PostgreSQL:    $PostgresHost`:$PostgresPort"
Write-Host "  Redis:         127.0.0.1:6379"
Write-Host "  etcd:          127.0.0.1:2379"
Write-Host "  Nacos:         $nacosUrl/nacos/"
if ($Events) { Write-Host "  Kafka:         127.0.0.1:9092"; Write-Host "  Kafka UI:      http://127.0.0.1:8088" }
if ($Comments) { Write-Host "  MongoDB:       127.0.0.1:27017" }
if ($Search) { Write-Host "  Elasticsearch: $(Get-LocalEnvironmentValue -Name "ELASTICSEARCH_URL" -Default "http://127.0.0.1:9200")" }
if ($Files) { Write-Host "  MinIO:         $(Get-RequiredLocalEnvironmentValue "MINIO_ENDPOINT")" }
if ($Mail) { Write-Host "  Mailpit:       http://127.0.0.1:8025" }
