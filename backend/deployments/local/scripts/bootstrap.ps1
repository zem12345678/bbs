param(
  [switch]$Full,
  [switch]$Events,
  [switch]$Comments,
  [switch]$Search,
  [switch]$Files
)

$ErrorActionPreference = "Stop"

$LocalRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $LocalRoot

if ($Full) {
  $Events = $true
  $Comments = $true
  $Search = $true
  $Files = $true
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

function Publish-NacosConfig {
  param(
    [string]$NacosUrl,
    [string]$Namespace,
    [string]$Group,
    [string]$FilePath
  )

  $file = Get-Item $FilePath
  $content = Get-Content -Raw -LiteralPath $file.FullName
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

Write-Host "Bootstrapping BBS local infrastructure..."

Wait-Tcp 127.0.0.1 5432 "PostgreSQL"
Wait-Tcp 127.0.0.1 6379 "Redis"
Wait-Tcp 127.0.0.1 2379 "etcd"
Wait-Tcp 127.0.0.1 8848 "Nacos"

Write-Host "Applying PostgreSQL schemas and local app users..."
docker compose exec -T postgres psql -U postgres -d bbs -f /docker-entrypoint-initdb.d/001-create-database-and-schemas.sql

$nacosUrl = "http://127.0.0.1:8848"
$nacosNamespace = "bbs-local"
$nacosGroup = "BBS_LOCAL"

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
  Wait-Tcp 127.0.0.1 9092 "Kafka"
  Write-Host "Creating Kafka topics..."
  Get-Content .\kafka\topics.txt | Where-Object { $_.Trim() -ne "" } | ForEach-Object {
    $topic = $_.Trim()
    docker compose exec -T kafka kafka-topics.sh --bootstrap-server 127.0.0.1:29092 --create --if-not-exists --topic $topic --partitions 1 --replication-factor 1 | Out-Host
  }
}

if ($Comments) {
  Wait-Tcp 127.0.0.1 27017 "MongoDB"
  Write-Host "Creating MongoDB comment indexes..."
  docker compose exec -T mongodb mongosh /docker-entrypoint-initdb.d/001-comments-indexes.js
}

if ($Search) {
  Wait-Tcp 127.0.0.1 9200 "Elasticsearch"
  Write-Host "Creating Elasticsearch indices..."
  Get-ChildItem .\elasticsearch -Filter *.mapping.json | Sort-Object Name | ForEach-Object {
    $index = $_.Name.Replace(".mapping.json", "")
    $exists = $false
    try {
      Invoke-WebRequest -Method Head -Uri "http://127.0.0.1:9200/$index" -UseBasicParsing | Out-Null
      $exists = $true
    } catch {
      $exists = $false
    }

    if ($exists) {
      Write-Host "elasticsearch index exists: $index"
    } else {
      $body = Get-Content -Raw -LiteralPath $_.FullName
      Invoke-RestMethod -Method Put -Uri "http://127.0.0.1:9200/$index" -ContentType "application/json" -Body $body | Out-Null
      Write-Host "elasticsearch index created: $index"
    }
  }
}

if ($Files) {
  Wait-Tcp 127.0.0.1 9000 "MinIO"
  Write-Host "Creating MinIO bucket..."
  docker run --rm --network bbs-local-net minio/mc:latest sh -c "mc alias set local http://minio:9000 minioadmin minioadmin >/dev/null && mc mb --ignore-existing local/bbs-local"
}

Write-Host ""
Write-Host "Local infra bootstrap complete."
Write-Host "Core endpoints:"
Write-Host "  PostgreSQL:    127.0.0.1:5432"
Write-Host "  Redis:         127.0.0.1:6379"
Write-Host "  etcd:          127.0.0.1:2379"
Write-Host "  Nacos:         http://127.0.0.1:8848/nacos/"
if ($Events) { Write-Host "  Kafka:         127.0.0.1:9092"; Write-Host "  Kafka UI:      http://127.0.0.1:8088" }
if ($Comments) { Write-Host "  MongoDB:       127.0.0.1:27017" }
if ($Search) { Write-Host "  Elasticsearch: http://127.0.0.1:9200" }
if ($Files) { Write-Host "  MinIO:         http://127.0.0.1:9001" }

