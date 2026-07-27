param(
  [string]$BaseUrl = "http://127.0.0.1:18080",
  [string]$AdminUsername = "admin",
  [string]$AdminPassword = "Admin123!",
  [string]$EnvironmentFile = "",
  [string]$MinIOContainer = "",
  [string]$MinIOEndpoint = "",
  [string]$MinIOBucket = "",
  [string]$MinIOAccessKey = "",
  [string]$MinIOSecretKey = "",
  [switch]$SkipMinIOVerification
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")

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

if ([string]::IsNullOrWhiteSpace($EnvironmentFile)) {
  $EnvironmentFile = Join-Path $RepoRoot "backend\deployments\local\.env"
}
Import-ProcessEnvironmentFile $EnvironmentFile

if ([string]::IsNullOrWhiteSpace($MinIOContainer)) { $MinIOContainer = $env:MINIO_CONTAINER }
if ([string]::IsNullOrWhiteSpace($MinIOEndpoint)) { $MinIOEndpoint = $env:MINIO_ENDPOINT }
if ([string]::IsNullOrWhiteSpace($MinIOBucket)) { $MinIOBucket = $env:MINIO_BUCKET }
if ([string]::IsNullOrWhiteSpace($MinIOAccessKey)) { $MinIOAccessKey = $env:MINIO_ACCESS_KEY }
if ([string]::IsNullOrWhiteSpace($MinIOSecretKey)) { $MinIOSecretKey = $env:MINIO_SECRET_KEY }

$baseUrl = $BaseUrl.TrimEnd("/")
$stamp = Get-Date -Format "yyMMddHHmmssfff"
$priceCredits = 5000
$updatedPriceCredits = 3000
$buyerTopUp = 10000
$tempDirectory = Join-Path ([System.IO.Path]::GetTempPath()) "bbs-attachment-smoke-$stamp"
$sourceFile = Join-Path $tempDirectory "paid-attachment.txt"
$downloadFile = Join-Path $tempDirectory "downloaded-attachment.txt"
$downloadHeadersFile = Join-Path $tempDirectory "download.headers"
$attachmentID = 0
$missingObjectAttachmentID = 0
$archivedTopicAttachmentID = 0
$topicID = 0
$membershipProductID = 0
$membershipEntitlementID = 0
$membershipGrantKey = "vip-attachment-$stamp"
$membershipPriceCredits = 1
$minioObjectKeys = @()
$archivedAttachmentIDs = @()
$author = $null
$minioMcConfigDirectory = "/tmp/bbs-attachment-smoke-$stamp"

if (-not $SkipMinIOVerification -and ([string]::IsNullOrWhiteSpace($MinIOContainer) -or [string]::IsNullOrWhiteSpace($MinIOEndpoint) -or [string]::IsNullOrWhiteSpace($MinIOBucket) -or [string]::IsNullOrWhiteSpace($MinIOAccessKey) -or [string]::IsNullOrWhiteSpace($MinIOSecretKey))) {
  throw "Set MINIO_CONTAINER, MINIO_ENDPOINT, MINIO_BUCKET, MINIO_ACCESS_KEY, and MINIO_SECRET_KEY in $EnvironmentFile, or use -SkipMinIOVerification."
}

function Convert-ApiResponse {
  param([string]$Raw)

  $response = $Raw | ConvertFrom-Json
  if ($null -ne $response -and $null -ne $response.code -and [int64]$response.code -ne 0) {
    throw "API error $($response.code): $($response.message)"
  }
  if ($null -ne $response -and $null -ne $response.data) {
    return $response.data
  }
  return $response
}

function Invoke-Api {
  $response = Microsoft.PowerShell.Utility\Invoke-RestMethod @args
  if ($null -ne $response -and $null -ne $response.code -and [int64]$response.code -ne 0) {
    throw "API error $($response.code): $($response.message)"
  }
  if ($null -ne $response -and $null -ne $response.data) {
    return $response.data
  }
  return $response
}

function Invoke-MultipartApi {
  param(
    [string]$Uri,
    [hashtable]$Headers,
    [string]$FilePath,
    [string]$Filename,
    [int64]$PriceCredits,
    [int]$ExpectedStatus = 200
  )

  $responseFile = Join-Path $tempDirectory ([System.IO.Path]::GetRandomFileName())
  $curlArgs = @(
    "--silent",
    "--show-error",
    "--max-time", "30",
    "--request", "POST",
    "--header", "Authorization: $($Headers.Authorization)",
    "--form", "price_credits=$PriceCredits",
    "--form", "file=@$FilePath;filename=$Filename;type=text/plain; charset=utf-8",
    "--output", $responseFile,
    "--write-out", "%{http_code}",
    $Uri
  )
  $status = & curl.exe @curlArgs
  if ($LASTEXITCODE -ne 0) {
    throw "Multipart upload failed with curl exit code $LASTEXITCODE"
  }
  $actualStatus = [int](($status -join "").Trim())
  $raw = Get-Content -LiteralPath $responseFile -Raw
  Remove-Item -LiteralPath $responseFile -Force
  if ($actualStatus -ne $ExpectedStatus) {
    throw "Expected multipart upload HTTP $ExpectedStatus, got HTTP ${actualStatus}: $raw"
  }
  $data = $null
  if ($actualStatus -ge 200 -and $actualStatus -lt 300) {
    $data = Convert-ApiResponse $raw
  }
  return @{ Raw = $raw; Data = $data }
}

function Invoke-JsonApi {
  param(
    [string]$Uri,
    [hashtable]$Headers,
    [string]$Body,
    [string]$Method = "PATCH",
    [int]$ExpectedStatus = 200
  )

  $responseFile = Join-Path $tempDirectory ([System.IO.Path]::GetRandomFileName())
  $requestFile = Join-Path $tempDirectory ([System.IO.Path]::GetRandomFileName())
  [System.IO.File]::WriteAllText($requestFile, $Body, [System.Text.UTF8Encoding]::new($false))
  $curlArgs = @(
    "--silent",
    "--show-error",
    "--max-time", "30",
    "--request", $Method,
    "--header", "Authorization: $($Headers.Authorization)",
    "--header", "Content-Type: application/json",
    "--data-binary", "@$requestFile",
    "--output", $responseFile,
    "--write-out", "%{http_code}",
    $Uri
  )
  $status = & curl.exe @curlArgs
  if ($LASTEXITCODE -ne 0) {
    Remove-Item -LiteralPath $requestFile -Force
    throw "JSON request failed with curl exit code $LASTEXITCODE"
  }
  $actualStatus = [int](($status -join "").Trim())
  $raw = Get-Content -LiteralPath $responseFile -Raw
  Remove-Item -LiteralPath $responseFile -Force
  Remove-Item -LiteralPath $requestFile -Force
  if ($actualStatus -ne $ExpectedStatus) {
    throw "Expected JSON request HTTP $ExpectedStatus, got HTTP ${actualStatus}: $raw"
  }
  $data = $null
  if ($actualStatus -ge 200 -and $actualStatus -lt 300) {
    $data = Convert-ApiResponse $raw
  }
  return @{ Raw = $raw; Data = $data }
}

function Invoke-Download {
  param(
    [string]$Uri,
    [hashtable]$Headers,
    [string]$OutputFile,
    [string]$HeadersFile,
    [int]$ExpectedStatus
  )

  $status = & curl.exe `
    "--silent" `
    "--show-error" `
    "--max-time" "30" `
    "--request" "GET" `
    "--header" "Authorization: $($Headers.Authorization)" `
    "--output" $OutputFile `
    "--dump-header" $HeadersFile `
    "--write-out" "%{http_code}" `
    $Uri
  if ($LASTEXITCODE -ne 0) {
    throw "Attachment download failed with curl exit code $LASTEXITCODE"
  }
  $actualStatus = [int](($status -join "").Trim())
  if ($actualStatus -ne $ExpectedStatus) {
    $body = if (Test-Path -LiteralPath $OutputFile) { Get-Content -LiteralPath $OutputFile -Raw } else { "" }
    throw "Expected download HTTP $ExpectedStatus, got HTTP ${actualStatus}: $body"
  }
}

function Register-User {
  param(
    [string]$Prefix,
    [string]$Nickname
  )

  $username = "$Prefix$stamp"
  $registerBody = @{
    username = $username
    email = "$username@example.com"
    password = "Password123!"
    nickname = $Nickname
  } | ConvertTo-Json
  $registered = Invoke-Api -Uri "$baseUrl/api/v1/auth/register" -Method Post -ContentType "application/json" -Body $registerBody -TimeoutSec 15
  if (-not $registered.access_token -or -not $registered.user.id) {
    throw "Registration did not return an access token and user id for $username"
  }
  return @{ Id = [int64]$registered.user.id; Headers = @{ Authorization = "Bearer $($registered.access_token)" }; Username = $username }
}

function Get-CreditBalance {
  param([hashtable]$Headers)

  $response = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $Headers -TimeoutSec 15
  if ($null -eq $response.balance) {
    throw "Credit balance response did not include balance"
  }
  return [int64]$response.balance.total
}

function Add-Credits {
  param(
    [int64]$UserID,
    [int64]$Delta,
    [hashtable]$AdminHeaders,
    [string]$SourceEventID
  )

  $body = @{
    delta = $Delta
    reason = "attachment_smoke_topup"
    description = "Attachment smoke test credit top-up"
    source_event_id = $SourceEventID
  } | ConvertTo-Json
  $response = Invoke-Api -Uri "$baseUrl/api/v1/admin/credits/users/$UserID/adjust" -Method Post -Headers $AdminHeaders -ContentType "application/json" -Body $body -TimeoutSec 15
  if ($null -eq $response.balance) {
    throw "Credit adjustment did not return balance"
  }
}

function Test-MinIOBucketExists {
  $lines = & docker.exe exec $MinIOContainer mc --config-dir $minioMcConfigDirectory ls --json local
  if ($LASTEXITCODE -ne 0) {
    throw "Could not list MinIO buckets"
  }
  foreach ($line in @($lines)) {
    if ([string]::IsNullOrWhiteSpace([string]$line)) {
      continue
    }
    $item = $line | ConvertFrom-Json
    if ($item.type -eq "folder" -and ([string]$item.key).TrimEnd("/") -eq $MinIOBucket) {
      return $true
    }
  }
  return $false
}

function Get-MinIOTopicObjects {
  param([int64]$TopicID)

  if (-not (Test-MinIOBucketExists)) {
    return @()
  }

  $lines = & docker.exe exec $MinIOContainer mc --config-dir $minioMcConfigDirectory ls --recursive --json "local/$MinIOBucket/topics/$TopicID/"
  if ($LASTEXITCODE -ne 0) {
    throw "Could not list MinIO objects for topic $TopicID"
  }
  $objects = @()
  foreach ($line in @($lines)) {
    if ([string]::IsNullOrWhiteSpace([string]$line)) {
      continue
    }
    $item = $line | ConvertFrom-Json
    if ($item.type -eq "file" -and $item.key) {
      $prefix = "topics/$TopicID/"
      if (-not ([string]$item.key).StartsWith($prefix, [System.StringComparison]::Ordinal)) {
        $item.key = $prefix + $item.key
      }
      $objects += $item
    }
  }
  return @($objects)
}

function Remove-MinIOObject {
  param([string]$ObjectKey)

  & docker.exe exec $MinIOContainer mc --config-dir $minioMcConfigDirectory rm --force "local/$MinIOBucket/$ObjectKey" | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw "Could not remove MinIO object $ObjectKey"
  }
}

function Remove-MinIOTopicObjects {
  param([int64]$TopicID)

  if ($TopicID -le 0) {
    return
  }
  if (-not (Test-MinIOBucketExists)) {
    return
  }
  & docker.exe exec $MinIOContainer mc --config-dir $minioMcConfigDirectory rm --recursive --force "local/$MinIOBucket/topics/$TopicID/" | Out-Null
}

function Set-MinIOAlias {
  & docker.exe exec $MinIOContainer mc --config-dir $minioMcConfigDirectory alias set local "http://127.0.0.1:9000" $MinIOAccessKey $MinIOSecretKey | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw "Could not configure MinIO alias for container '$MinIOContainer'"
  }
}

try {
  New-Item -ItemType Directory -Path $tempDirectory -Force | Out-Null
  $attachmentContent = "paid attachment smoke $stamp"
  [System.IO.File]::WriteAllBytes($sourceFile, [System.Text.Encoding]::UTF8.GetBytes($attachmentContent))
  $sourceLength = (Get-Item -LiteralPath $sourceFile).Length

  Invoke-Api -Uri "$baseUrl/healthz" -Method Get -TimeoutSec 10 | Out-Null

  if (-not $SkipMinIOVerification) {
    & docker.exe inspect --type container $MinIOContainer | Out-Null
    if ($LASTEXITCODE -ne 0) {
      throw "External MinIO container '$MinIOContainer' is not available. Use -SkipMinIOVerification only when object-level verification is unavailable."
    }
    Invoke-WebRequest -UseBasicParsing -Uri "$($MinIOEndpoint.TrimEnd('/'))/minio/health/live" -TimeoutSec 10 | Out-Null
    Set-MinIOAlias
  }

  $adminLoginBody = @{ account = $AdminUsername; password = $AdminPassword } | ConvertTo-Json
  $admin = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/login" -Method Post -ContentType "application/json" -Body $adminLoginBody -TimeoutSec 15
  if (-not $admin.access_token) {
    throw "Admin login did not return an access token"
  }
  $adminHeaders = @{ Authorization = "Bearer $($admin.access_token)" }

  $categories = Invoke-Api -Uri "$baseUrl/api/v1/categories?status=2&limit=20&offset=0" -Method Get -TimeoutSec 15
  $category = @($categories.items | Where-Object { $_.slug -eq "general" }) | Select-Object -First 1
  if (-not $category -or -not $category.id) {
    throw "The seeded general category is required for attachment smoke testing"
  }

  $author = Register-User -Prefix "at" -Nickname "Attachment Author"
  $buyer = Register-User -Prefix "bt" -Nickname "Attachment Buyer"
  $insufficientBuyer = Register-User -Prefix "pt" -Nickname "Attachment Poor Buyer"

  $topicBody = @{
    slug = "attachment-smoke-$stamp"
    type = "topic"
    title = "Attachment smoke $stamp"
    body = "A topic used to verify paid attachment authorization."
    tags = @("attachment", "smoke")
    category_id = [int64]$category.id
    publish = $true
  } | ConvertTo-Json
  $topic = Invoke-Api -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $author.Headers -ContentType "application/json" -Body $topicBody -TimeoutSec 15
  $topicID = [int64]$topic.topic.id
  if ($topicID -le 0) {
    throw "Topic creation did not return topic.id"
  }

  $adminSettings = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings?limit=100&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 15
  $emailVerificationSetting = @($adminSettings.items | Where-Object { $_.key -eq "auth.email_verification.required" }) | Select-Object -First 1
  if ($null -eq $emailVerificationSetting) {
    throw "Admin settings did not include auth.email_verification.required"
  }
  $emailVerificationAttachmentBlocked = $false
  try {
    $enableEmailVerificationBody = @{
      key = "auth.email_verification.required"
      value = "true"
      group = $emailVerificationSetting.group
      value_type = $emailVerificationSetting.value_type
      description = $emailVerificationSetting.description
      status = $emailVerificationSetting.status
    } | ConvertTo-Json
    $enabledEmailVerification = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/auth.email_verification.required" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $enableEmailVerificationBody -TimeoutSec 15
    if ($enabledEmailVerification.setting.value -ne "true") {
      throw "Admin email verification setting did not enable"
    }
    Invoke-MultipartApi -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Headers $author.Headers -FilePath $sourceFile -Filename "email-verification-required.txt" -PriceCredits $priceCredits -ExpectedStatus 403 | Out-Null
    if (-not $SkipMinIOVerification -and @(Get-MinIOTopicObjects -TopicID $topicID).Count -ne 0) {
      throw "Unverified attachment upload wrote an object"
    }
    $emailVerificationAttachmentBlocked = $true
  } finally {
    $restoreEmailVerificationBody = @{
      key = "auth.email_verification.required"
      value = $emailVerificationSetting.value
      group = $emailVerificationSetting.group
      value_type = $emailVerificationSetting.value_type
      description = $emailVerificationSetting.description
      status = $emailVerificationSetting.status
    } | ConvertTo-Json
    $restoredEmailVerification = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/auth.email_verification.required" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $restoreEmailVerificationBody -TimeoutSec 15
    if ([string]$restoredEmailVerification.setting.value -ne [string]$emailVerificationSetting.value) {
      throw "Admin email verification setting did not restore"
    }
  }

  $objectsBeforeMembershipGate = @()
  if (-not $SkipMinIOVerification) {
    $objectsBeforeMembershipGate = @(Get-MinIOTopicObjects -TopicID $topicID)
  }
  $missingMembershipUpload = Invoke-MultipartApi -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Headers $author.Headers -FilePath $sourceFile -Filename "membership-required.txt" -PriceCredits $priceCredits -ExpectedStatus 403
  if ($missingMembershipUpload.Raw -notmatch "membership entitlement required for paid attachments") {
    throw "Non-member paid attachment upload did not return the membership entitlement error"
  }
  if (-not $SkipMinIOVerification -and @(Get-MinIOTopicObjects -TopicID $topicID).Count -ne $objectsBeforeMembershipGate.Count) {
    throw "Non-member paid attachment upload wrote an object"
  }

  $membershipProductBody = @{
    sku = $membershipGrantKey
    title = "Attachment Membership $stamp"
    description = "Attachment smoke membership entitlement"
    category = "digital"
    cover_url = ""
    grant_type = "membership"
    grant_key = $membershipGrantKey
    price_credits = $membershipPriceCredits
    stock = 2
    status = 2
    sort = 999
  } | ConvertTo-Json
  $membershipProduct = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/products" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $membershipProductBody -TimeoutSec 15
  $membershipProductID = [int64]$membershipProduct.product.id
  if ($membershipProductID -le 0 -or $membershipProduct.product.grant_type -ne "membership" -or $membershipProduct.product.grant_key -ne $membershipGrantKey) {
    throw "Attachment membership product did not return the expected membership grant"
  }
  Add-Credits -UserID $author.Id -Delta $membershipPriceCredits -AdminHeaders $adminHeaders -SourceEventID "attachment-smoke-membership-topup-$stamp"
  $membershipOrderBody = @{
    idempotency_key = "attachment-smoke-membership-order-$stamp"
    expected_original_credits = $membershipPriceCredits
    items = @(@{
        product_id = $membershipProductID
        quantity = 1
      })
  } | ConvertTo-Json -Depth 5
  $membershipOrderCreated = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders" -Method Post -Headers $author.Headers -ContentType "application/json" -Body $membershipOrderBody -TimeoutSec 15
  $membershipOrderID = [int64]$membershipOrderCreated.order.id
  if ($membershipOrderID -le 0 -or [int64]$membershipOrderCreated.order.status -ne 1) {
    throw "Attachment membership order did not create a pending payment order"
  }
  $membershipPayBody = @{
    payment_method = "credits"
    idempotency_key = "attachment-smoke-membership-pay-$stamp"
  } | ConvertTo-Json
  $membershipOrderPaid = Invoke-Api -Uri "$baseUrl/api/v1/mall/orders/$membershipOrderID/pay" -Method Post -Headers $author.Headers -ContentType "application/json" -Body $membershipPayBody -TimeoutSec 15
  if ([int64]$membershipOrderPaid.order.status -ne 6) {
    throw "Attachment membership order did not auto-complete"
  }
  $membershipEntitlement = @($membershipOrderPaid.order.digital_entitlements | Where-Object { $_.status -eq "ACTIVE" -and $_.grant_type -eq "membership" -and $_.grant_key -eq $membershipGrantKey }) | Select-Object -First 1
  $membershipEntitlementID = [int64]$membershipEntitlement.id
  if ($membershipEntitlementID -le 0 -or [int64]$membershipEntitlement.expires_at -le [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()) {
    throw "Attachment membership order did not issue a future active entitlement"
  }

  Add-Credits -UserID $buyer.Id -Delta $buyerTopUp -AdminHeaders $adminHeaders -SourceEventID "attachment-smoke-topup-$stamp"
  $buyerBalanceBefore = Get-CreditBalance -Headers $buyer.Headers
  if ($buyerBalanceBefore -lt $priceCredits) {
    throw "Funded buyer has insufficient test balance: $buyerBalanceBefore"
  }

  Invoke-MultipartApi -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Headers $buyer.Headers -FilePath $sourceFile -Filename "forbidden.txt" -PriceCredits 0 -ExpectedStatus 403 | Out-Null

  $uploaded = Invoke-MultipartApi -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Headers $author.Headers -FilePath $sourceFile -Filename "paid-attachment.txt" -PriceCredits $priceCredits
  if ($uploaded.Raw -match '"object_key"') {
    throw "Attachment upload response exposed object_key"
  }
  $attachmentID = [int64]$uploaded.Data.id
  if ($attachmentID -le 0 -or [int64]$uploaded.Data.price_credits -ne $priceCredits -or [int64]$uploaded.Data.size_bytes -ne $sourceLength) {
    throw "Attachment upload did not return the expected metadata"
  }

  $listedAttachments = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Method Get -TimeoutSec 15
  $listedAttachment = @($listedAttachments.items | Where-Object { [int64]$_.id -eq $attachmentID }) | Select-Object -First 1
  if (-not $listedAttachment -or [int64]$listedAttachment.price_credits -ne $priceCredits) {
    throw "Topic attachment list did not include the uploaded paid attachment"
  }

  if (-not $SkipMinIOVerification) {
    $minioObjects = @(Get-MinIOTopicObjects -TopicID $topicID)
    if ($minioObjects.Count -ne 1 -or [int64]$minioObjects[0].size -ne $sourceLength) {
      throw "MinIO did not contain exactly one uploaded attachment with the expected size"
    }
    $minioObjectKeys = @($minioObjects | ForEach-Object { [string]$_.key })
  }

  $authorBalanceBefore = Get-CreditBalance -Headers $author.Headers
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $author.Headers -OutputFile $downloadFile -HeadersFile $downloadHeadersFile -ExpectedStatus 200
  $authorBalanceAfter = Get-CreditBalance -Headers $author.Headers
  if ($authorBalanceAfter -ne $authorBalanceBefore) {
    throw "Attachment author was charged for downloading their own attachment"
  }
  if ((Get-FileHash -Algorithm SHA256 -LiteralPath $sourceFile).Hash -ne (Get-FileHash -Algorithm SHA256 -LiteralPath $downloadFile).Hash) {
    throw "Downloaded attachment contents did not match the uploaded object"
  }
  $authorDownloadHistory = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $author.Headers -TimeoutSec 15
  $authorDownloadRecord = @($authorDownloadHistory.items | Where-Object { [int64]$_.attachment.id -eq $attachmentID }) | Select-Object -First 1
  if (-not $authorDownloadRecord -or $authorDownloadRecord.status -ne "AUTHORIZED" -or [int64]$authorDownloadRecord.charged_credits -ne 0) {
    throw "Owner attachment download history did not record a free authorized download"
  }
  if (($authorDownloadHistory | ConvertTo-Json -Depth 8 -Compress) -match '"object_key"') {
    throw "Owner attachment download history exposed object_key"
  }

  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $buyer.Headers -OutputFile $downloadFile -HeadersFile $downloadHeadersFile -ExpectedStatus 200
  $buyerBalanceAfterFirstDownload = Get-CreditBalance -Headers $buyer.Headers
  if ($buyerBalanceAfterFirstDownload -ne ($buyerBalanceBefore - $priceCredits)) {
    throw "First paid attachment download did not debit exactly $priceCredits credits"
  }
  $authorBalanceAfterFirstBuyerDownload = Get-CreditBalance -Headers $author.Headers
  if ($authorBalanceAfterFirstBuyerDownload -ne ($authorBalanceBefore + $priceCredits)) {
    throw "First paid attachment download did not credit the author exactly $priceCredits credits"
  }
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $buyer.Headers -OutputFile $downloadFile -HeadersFile $downloadHeadersFile -ExpectedStatus 200
  $buyerBalanceAfterSecondDownload = Get-CreditBalance -Headers $buyer.Headers
  if ($buyerBalanceAfterSecondDownload -ne $buyerBalanceAfterFirstDownload) {
    throw "Repeated attachment download charged credits more than once"
  }
  $authorBalanceAfterSecondBuyerDownload = Get-CreditBalance -Headers $author.Headers
  if ($authorBalanceAfterSecondBuyerDownload -ne $authorBalanceAfterFirstBuyerDownload) {
    throw "Repeated attachment download credited the author more than once"
  }
  $authorSaleEventID = "attachment-download:$($attachmentID):$($buyer.Id)"
  $authorCreditLedger = Invoke-Api -Uri "$baseUrl/api/v1/credits/ledger?limit=50&offset=0" -Method Get -Headers $author.Headers -TimeoutSec 15
  $authorSaleEntries = @($authorCreditLedger.items | Where-Object {
    $_.reason -eq "attachment_sale" -and
    [string]$_.source_event_id -eq $authorSaleEventID -and
    [string]$_.source_type -eq "attachment" -and
    [int64]$_.source_id -eq $attachmentID -and
    [int64]$_.delta -eq $priceCredits
  })
  if ($authorSaleEntries.Count -ne 1) {
    throw "Attachment sale did not create exactly one author ledger entry"
  }
  $authorSaleLedger = $authorSaleEntries[0]
  $authorSaleHistory = Invoke-Api -Uri "$baseUrl/api/v1/attachments/sales?limit=10&offset=0" -Method Get -Headers $author.Headers -TimeoutSec 15
  $authorSaleRecords = @($authorSaleHistory.items | Where-Object { [int64]$_.attachment.id -eq $attachmentID })
  if ($authorSaleRecords.Count -ne 1) {
    throw "Attachment sale history did not retain exactly one paid sale after a repeated download"
  }
  $authorSaleRecord = $authorSaleRecords[0]
  if ([int64]$authorSaleRecord.earned_credits -ne $priceCredits -or [int64]$authorSaleRecord.sold_at -le 0) {
    throw "Attachment sale history did not retain the earned credits and sale time"
  }
  if ([int64]$authorSaleHistory.total -ne 1 -or [int64]$authorSaleHistory.total_earned_credits -ne $priceCredits) {
    throw "Attachment sale history did not return the expected summary"
  }
  if (($authorSaleHistory | ConvertTo-Json -Depth 8 -Compress) -match '"object_key"') {
    throw "Attachment sale history exposed object_key"
  }
  $buyerDownloadHistory = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $buyer.Headers -TimeoutSec 15
  $buyerDownloadRecord = @($buyerDownloadHistory.items | Where-Object { [int64]$_.attachment.id -eq $attachmentID }) | Select-Object -First 1
  if (-not $buyerDownloadRecord -or $buyerDownloadRecord.status -ne "AUTHORIZED" -or [int64]$buyerDownloadRecord.charged_credits -ne $priceCredits -or [int64]$buyerDownloadRecord.authorized_at -le 0) {
    throw "Buyer attachment download history did not retain the paid authorization"
  }
  if (($buyerDownloadHistory | ConvertTo-Json -Depth 8 -Compress) -match '"object_key"') {
    throw "Buyer attachment download history exposed object_key"
  }

  $insufficientBalanceBefore = Get-CreditBalance -Headers $insufficientBuyer.Headers
  $authorBalanceBeforeInsufficientDownload = Get-CreditBalance -Headers $author.Headers
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $insufficientBuyer.Headers -OutputFile (Join-Path $tempDirectory "insufficient-download.body") -HeadersFile $downloadHeadersFile -ExpectedStatus 412
  $insufficientBalanceAfter = Get-CreditBalance -Headers $insufficientBuyer.Headers
  if ($insufficientBalanceAfter -ne $insufficientBalanceBefore) {
    throw "Insufficient-credit attachment download changed the buyer balance"
  }
  $authorBalanceAfterInsufficientDownload = Get-CreditBalance -Headers $author.Headers
  if ($authorBalanceAfterInsufficientDownload -ne $authorBalanceBeforeInsufficientDownload) {
    throw "Insufficient-credit attachment download credited the author"
  }
  $insufficientBuyerDownloadHistory = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $insufficientBuyer.Headers -TimeoutSec 15
  if (@($insufficientBuyerDownloadHistory.items | Where-Object { $null -ne $_ }).Count -ne 0) {
    throw "Pending attachment download was exposed in download history"
  }

  $updatePriceBody = @{ price_credits = $updatedPriceCredits } | ConvertTo-Json -Compress
  Invoke-JsonApi -Uri "$baseUrl/api/v1/attachments/$attachmentID" -Headers $buyer.Headers -Body $updatePriceBody -ExpectedStatus 403 | Out-Null
  $emailVerificationPriceUpdateBlocked = $false
  try {
    $enableEmailVerificationBody = @{
      key = "auth.email_verification.required"
      value = "true"
      group = $emailVerificationSetting.group
      value_type = $emailVerificationSetting.value_type
      description = $emailVerificationSetting.description
      status = $emailVerificationSetting.status
    } | ConvertTo-Json
    $enabledEmailVerification = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/auth.email_verification.required" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $enableEmailVerificationBody -TimeoutSec 15
    if ($enabledEmailVerification.setting.value -ne "true") {
      throw "Admin email verification setting did not enable for attachment price update"
    }
    Invoke-JsonApi -Uri "$baseUrl/api/v1/attachments/$attachmentID" -Headers $author.Headers -Body $updatePriceBody -ExpectedStatus 403 | Out-Null
    $emailVerificationPriceUpdateBlocked = $true
  } finally {
    $restoreEmailVerificationBody = @{
      key = "auth.email_verification.required"
      value = $emailVerificationSetting.value
      group = $emailVerificationSetting.group
      value_type = $emailVerificationSetting.value_type
      description = $emailVerificationSetting.description
      status = $emailVerificationSetting.status
    } | ConvertTo-Json
    $restoredEmailVerification = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/auth.email_verification.required" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $restoreEmailVerificationBody -TimeoutSec 15
    if ([string]$restoredEmailVerification.setting.value -ne [string]$emailVerificationSetting.value) {
      throw "Admin email verification setting did not restore after attachment price update"
    }
  }
  $priceUpdated = Invoke-JsonApi -Uri "$baseUrl/api/v1/attachments/$attachmentID" -Headers $author.Headers -Body $updatePriceBody
  if ($priceUpdated.Raw -match '"object_key"') {
    throw "Attachment price update response exposed object_key"
  }
  if ([int64]$priceUpdated.Data.id -ne $attachmentID -or [int64]$priceUpdated.Data.price_credits -ne $updatedPriceCredits -or $priceUpdated.Data.status -ne "ACTIVE") {
    throw "Attachment price update did not return the expected metadata"
  }
  $updatedListedAttachments = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Method Get -TimeoutSec 15
  $updatedListedAttachment = @($updatedListedAttachments.items | Where-Object { [int64]$_.id -eq $attachmentID }) | Select-Object -First 1
  if (-not $updatedListedAttachment -or [int64]$updatedListedAttachment.price_credits -ne $updatedPriceCredits) {
    throw "Topic attachment list did not return the updated attachment price"
  }

  $buyerBalanceBeforeUpdatedPriceRetry = Get-CreditBalance -Headers $buyer.Headers
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $buyer.Headers -OutputFile $downloadFile -HeadersFile $downloadHeadersFile -ExpectedStatus 200
  $buyerBalanceAfterUpdatedPriceRetry = Get-CreditBalance -Headers $buyer.Headers
  if ($buyerBalanceAfterUpdatedPriceRetry -ne $buyerBalanceBeforeUpdatedPriceRetry) {
    throw "Previously authorized buyer was charged after an attachment price update"
  }
  $buyerDownloadHistoryAfterPriceUpdate = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $buyer.Headers -TimeoutSec 15
  $buyerDownloadRecordAfterPriceUpdate = @($buyerDownloadHistoryAfterPriceUpdate.items | Where-Object { [int64]$_.attachment.id -eq $attachmentID }) | Select-Object -First 1
  if (-not $buyerDownloadRecordAfterPriceUpdate -or [int64]$buyerDownloadRecordAfterPriceUpdate.charged_credits -ne $priceCredits -or [int64]$buyerDownloadRecordAfterPriceUpdate.attachment.price_credits -ne $updatedPriceCredits) {
    throw "Authorized buyer history did not retain its original charge after an attachment price update"
  }

  Add-Credits -UserID $insufficientBuyer.Id -Delta $priceCredits -AdminHeaders $adminHeaders -SourceEventID "attachment-smoke-pending-retry-topup-$stamp"
  $insufficientBalanceBeforeRetry = Get-CreditBalance -Headers $insufficientBuyer.Headers
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $insufficientBuyer.Headers -OutputFile (Join-Path $tempDirectory "insufficient-retry-download.body") -HeadersFile $downloadHeadersFile -ExpectedStatus 200
  $insufficientBalanceAfterRetry = Get-CreditBalance -Headers $insufficientBuyer.Headers
  if ($insufficientBalanceAfterRetry -ne ($insufficientBalanceBeforeRetry - $priceCredits)) {
    throw "Pending attachment download retry did not retain its original price"
  }
  $insufficientBuyerHistoryAfterRetry = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $insufficientBuyer.Headers -TimeoutSec 15
  $insufficientBuyerRecordAfterRetry = @($insufficientBuyerHistoryAfterRetry.items | Where-Object { [int64]$_.attachment.id -eq $attachmentID }) | Select-Object -First 1
  if (-not $insufficientBuyerRecordAfterRetry -or [int64]$insufficientBuyerRecordAfterRetry.charged_credits -ne $priceCredits) {
    throw "Pending attachment download history did not retain its original charge"
  }

  $priceUpdateBuyer = Register-User -Prefix "ut" -Nickname "Attachment Updated Price Buyer"
  Add-Credits -UserID $priceUpdateBuyer.Id -Delta $buyerTopUp -AdminHeaders $adminHeaders -SourceEventID "attachment-smoke-updated-price-topup-$stamp"
  $priceUpdateBuyerBalanceBefore = Get-CreditBalance -Headers $priceUpdateBuyer.Headers
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $priceUpdateBuyer.Headers -OutputFile (Join-Path $tempDirectory "updated-price-download.body") -HeadersFile $downloadHeadersFile -ExpectedStatus 200
  $priceUpdateBuyerBalanceAfter = Get-CreditBalance -Headers $priceUpdateBuyer.Headers
  if ($priceUpdateBuyerBalanceAfter -ne ($priceUpdateBuyerBalanceBefore - $updatedPriceCredits)) {
    throw "New attachment buyer did not pay the updated price"
  }
  $priceUpdateBuyerHistory = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $priceUpdateBuyer.Headers -TimeoutSec 15
  $priceUpdateBuyerRecord = @($priceUpdateBuyerHistory.items | Where-Object { [int64]$_.attachment.id -eq $attachmentID }) | Select-Object -First 1
  if (-not $priceUpdateBuyerRecord -or [int64]$priceUpdateBuyerRecord.charged_credits -ne $updatedPriceCredits) {
    throw "New attachment buyer history did not retain the updated charge"
  }

  if (-not $SkipMinIOVerification) {
    $missingObjectUpload = Invoke-MultipartApi -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Headers $author.Headers -FilePath $sourceFile -Filename "missing-object.txt" -PriceCredits $priceCredits
    $missingObjectAttachmentID = [int64]$missingObjectUpload.Data.id
    if ($missingObjectAttachmentID -le 0) {
      throw "Missing-object attachment upload did not return an id"
    }
    $objectsAfterMissingUpload = @(Get-MinIOTopicObjects -TopicID $topicID)
    $newObjects = @($objectsAfterMissingUpload | Where-Object { $minioObjectKeys -notcontains [string]$_.key })
    if ($newObjects.Count -ne 1) {
      throw "Could not identify the object for missing-object authorization testing"
    }
    Remove-MinIOObject -ObjectKey ([string]$newObjects[0].key)
    $buyerBalanceBeforeMissingObject = Get-CreditBalance -Headers $buyer.Headers
    Invoke-Download -Uri "$baseUrl/api/v1/attachments/$missingObjectAttachmentID/download" -Headers $buyer.Headers -OutputFile (Join-Path $tempDirectory "missing-object-download.body") -HeadersFile $downloadHeadersFile -ExpectedStatus 502
    $buyerBalanceAfterMissingObject = Get-CreditBalance -Headers $buyer.Headers
    if ($buyerBalanceAfterMissingObject -ne $buyerBalanceBeforeMissingObject) {
      throw "Missing attachment object authorized a paid download"
    }
  }

  $membershipRevokeReason = "Attachment smoke membership revoke $stamp"
  $membershipRevokeBody = @{ reason = $membershipRevokeReason } | ConvertTo-Json
  $revokedMembershipEntitlement = Invoke-Api -Uri "$baseUrl/api/v1/admin/mall/digital-entitlements/$membershipEntitlementID/revoke" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $membershipRevokeBody -TimeoutSec 15
  if ([int64]$revokedMembershipEntitlement.entitlement.id -ne $membershipEntitlementID -or $revokedMembershipEntitlement.entitlement.status -ne "REVOKED" -or $revokedMembershipEntitlement.entitlement.revoke_reason -ne $membershipRevokeReason) {
    throw "Attachment membership entitlement revoke did not return the expected result"
  }
  $objectsBeforeRevokedMembershipUpload = @()
  if (-not $SkipMinIOVerification) {
    $objectsBeforeRevokedMembershipUpload = @(Get-MinIOTopicObjects -TopicID $topicID)
  }
  $revokedMembershipUpload = Invoke-MultipartApi -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Headers $author.Headers -FilePath $sourceFile -Filename "revoked-membership.txt" -PriceCredits $priceCredits -ExpectedStatus 403
  if ($revokedMembershipUpload.Raw -notmatch "membership entitlement required for paid attachments") {
    throw "Revoked membership paid attachment upload did not return the membership entitlement error"
  }
  if (-not $SkipMinIOVerification -and @(Get-MinIOTopicObjects -TopicID $topicID).Count -ne $objectsBeforeRevokedMembershipUpload.Count) {
    throw "Revoked membership paid attachment upload wrote an object"
  }
  $buyerBalanceBeforeRevokedMembershipRetry = Get-CreditBalance -Headers $buyer.Headers
  $authorBalanceBeforeRevokedMembershipRetry = Get-CreditBalance -Headers $author.Headers
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $buyer.Headers -OutputFile $downloadFile -HeadersFile $downloadHeadersFile -ExpectedStatus 200
  if ((Get-CreditBalance -Headers $buyer.Headers) -ne $buyerBalanceBeforeRevokedMembershipRetry -or (Get-CreditBalance -Headers $author.Headers) -ne $authorBalanceBeforeRevokedMembershipRetry) {
    throw "Revoked author membership changed balances for an already authorized attachment download"
  }
  $revokedSaleBuyer = Register-User -Prefix "rm" -Nickname "Attachment Revoked Membership Buyer"
  Add-Credits -UserID $revokedSaleBuyer.Id -Delta $buyerTopUp -AdminHeaders $adminHeaders -SourceEventID "attachment-smoke-revoked-membership-buyer-topup-$stamp"
  $revokedSaleBuyerBalanceBefore = Get-CreditBalance -Headers $revokedSaleBuyer.Headers
  $authorBalanceBeforeRevokedSale = Get-CreditBalance -Headers $author.Headers
  $revokedSaleDownloadFile = Join-Path $tempDirectory "revoked-membership-sale.body"
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $revokedSaleBuyer.Headers -OutputFile $revokedSaleDownloadFile -HeadersFile $downloadHeadersFile -ExpectedStatus 412
  $revokedSaleDownloadBody = Get-Content -LiteralPath $revokedSaleDownloadFile -Raw
  if ($revokedSaleDownloadBody -notmatch "paid attachment sales unavailable because the author membership entitlement is inactive") {
    throw "Revoked membership paid attachment sale did not return the author membership error"
  }
  if ((Get-CreditBalance -Headers $revokedSaleBuyer.Headers) -ne $revokedSaleBuyerBalanceBefore -or (Get-CreditBalance -Headers $author.Headers) -ne $authorBalanceBeforeRevokedSale) {
    throw "Revoked membership paid attachment sale changed buyer or author balances"
  }
  $revokedSaleBuyerHistory = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $revokedSaleBuyer.Headers -TimeoutSec 15
  if (@($revokedSaleBuyerHistory.items | Where-Object { $null -ne $_ }).Count -ne 0) {
    throw "Revoked membership paid attachment sale created a download record"
  }
  $revokedPriceBody = @{ price_credits = ($updatedPriceCredits + 1) } | ConvertTo-Json -Compress
  $revokedPriceUpdate = Invoke-JsonApi -Uri "$baseUrl/api/v1/attachments/$attachmentID" -Headers $author.Headers -Body $revokedPriceBody -ExpectedStatus 403
  if ($revokedPriceUpdate.Raw -notmatch "membership entitlement required for paid attachments") {
    throw "Revoked membership paid attachment price update did not return the membership entitlement error"
  }
  $freePriceBody = @{ price_credits = 0 } | ConvertTo-Json -Compress
  $freePriceUpdate = Invoke-JsonApi -Uri "$baseUrl/api/v1/attachments/$attachmentID" -Headers $author.Headers -Body $freePriceBody
  if ([int64]$freePriceUpdate.Data.price_credits -ne 0) {
    throw "Revoked membership owner could not lower an attachment price to free"
  }

  $archived = Invoke-Api -Uri "$baseUrl/api/v1/attachments/$attachmentID" -Method Delete -Headers $author.Headers -TimeoutSec 15
  if ($archived.status -ne "ARCHIVED") {
    throw "Attachment archive did not return ARCHIVED status"
  }
  $authorSaleHistoryAfterArchive = Invoke-Api -Uri "$baseUrl/api/v1/attachments/sales?limit=10&offset=0" -Method Get -Headers $author.Headers -TimeoutSec 15
  $archivedSaleRecords = @($authorSaleHistoryAfterArchive.items | Where-Object { [int64]$_.attachment.id -eq $attachmentID })
  if ($archivedSaleRecords.Count -eq 0 -or @($archivedSaleRecords | Where-Object { $_.attachment.status -ne "ARCHIVED" }).Count -ne 0) {
    throw "Archived attachment sale history did not preserve archived metadata"
  }
  $expectedArchivedSaleTotal = 3
  $expectedArchivedSaleCredits = ([int64]$priceCredits * 2) + [int64]$updatedPriceCredits
  if ([int64]$authorSaleHistoryAfterArchive.total -ne $expectedArchivedSaleTotal -or [int64]$authorSaleHistoryAfterArchive.total_earned_credits -ne $expectedArchivedSaleCredits) {
    throw "Archived attachment sale history did not retain the expected summary"
  }
  $archivedAttachmentIDs += $attachmentID
  $buyerBalanceBeforeArchivedDownload = Get-CreditBalance -Headers $buyer.Headers
  $archivedDownloadFile = Join-Path $tempDirectory "archived-download.body"
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $buyer.Headers -OutputFile $archivedDownloadFile -HeadersFile $downloadHeadersFile -ExpectedStatus 200
  $buyerBalanceAfterArchivedDownload = Get-CreditBalance -Headers $buyer.Headers
  if ($buyerBalanceAfterArchivedDownload -ne $buyerBalanceBeforeArchivedDownload) {
    throw "Archived attachment replay changed the buyer balance"
  }
  if ((Get-FileHash -Algorithm SHA256 -LiteralPath $sourceFile).Hash -ne (Get-FileHash -Algorithm SHA256 -LiteralPath $archivedDownloadFile).Hash) {
    throw "Archived attachment replay did not return the original object"
  }
  $archivedUnpaidBuyer = Register-User -Prefix "rt" -Nickname "Archived Attachment Unpaid Buyer"
  $archivedUnpaidBalanceBefore = Get-CreditBalance -Headers $archivedUnpaidBuyer.Headers
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $archivedUnpaidBuyer.Headers -OutputFile (Join-Path $tempDirectory "archived-unpaid-download.body") -HeadersFile $downloadHeadersFile -ExpectedStatus 412
  $archivedUnpaidBalanceAfter = Get-CreditBalance -Headers $archivedUnpaidBuyer.Headers
  if ($archivedUnpaidBalanceAfter -ne $archivedUnpaidBalanceBefore) {
    throw "Unpaid archived attachment download changed the buyer balance"
  }
  $buyerDownloadHistoryAfterArchive = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $buyer.Headers -TimeoutSec 15
  $buyerArchivedDownloadRecord = @($buyerDownloadHistoryAfterArchive.items | Where-Object { [int64]$_.attachment.id -eq $attachmentID }) | Select-Object -First 1
  if (-not $buyerArchivedDownloadRecord -or $buyerArchivedDownloadRecord.attachment.status -ne "ARCHIVED" -or [int64]$buyerArchivedDownloadRecord.charged_credits -ne $priceCredits) {
    throw "Archived attachment was not retained in buyer download history"
  }

  if ($missingObjectAttachmentID -gt 0) {
    Invoke-Api -Uri "$baseUrl/api/v1/attachments/$missingObjectAttachmentID" -Method Delete -Headers $author.Headers -TimeoutSec 15 | Out-Null
    $archivedAttachmentIDs += $missingObjectAttachmentID
  }

  $archivedTopicUpload = Invoke-MultipartApi -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Headers $author.Headers -FilePath $sourceFile -Filename "topic-archived.txt" -PriceCredits 0
  $archivedTopicAttachmentID = [int64]$archivedTopicUpload.Data.id
  if ($archivedTopicAttachmentID -le 0) {
    throw "Topic-archive attachment upload did not return an id"
  }
  $archivedTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicID" -Method Delete -Headers $author.Headers -TimeoutSec 15
  if ([int64]$archivedTopic.topic.status -ne 4) {
    throw "Topic archive did not return ARCHIVED status"
  }
  $archivedTopicObjectCount = 0
  if (-not $SkipMinIOVerification) {
    $archivedTopicObjectCount = @(Get-MinIOTopicObjects -TopicID $topicID).Count
  }
  Invoke-MultipartApi -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Headers $author.Headers -FilePath $sourceFile -Filename "after-topic-archive.txt" -PriceCredits $priceCredits -ExpectedStatus 412 | Out-Null
  if (-not $SkipMinIOVerification -and @(Get-MinIOTopicObjects -TopicID $topicID).Count -ne $archivedTopicObjectCount) {
    throw "Archived topic attachment upload wrote an object"
  }
  $buyerBalanceBeforeArchivedTopicDownload = Get-CreditBalance -Headers $buyer.Headers
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$archivedTopicAttachmentID/download" -Headers $buyer.Headers -OutputFile (Join-Path $tempDirectory "archived-topic-download.body") -HeadersFile $downloadHeadersFile -ExpectedStatus 404
  $buyerBalanceAfterArchivedTopicDownload = Get-CreditBalance -Headers $buyer.Headers
  if ($buyerBalanceAfterArchivedTopicDownload -ne $buyerBalanceBeforeArchivedTopicDownload) {
    throw "Archived topic attachment download changed the buyer balance"
  }
  Invoke-JsonApi -Uri "$baseUrl/api/v1/attachments/$archivedTopicAttachmentID" -Headers $author.Headers -Body $updatePriceBody -ExpectedStatus 412 | Out-Null

  Write-Host "Attachment smoke passed: topic=$topicID attachment=$attachmentID buyer=$($buyer.Id) author_sale_ledger=$([int64]$authorSaleLedger.id) author_sale_credits=$priceCredits updated_price=$updatedPriceCredits membership_product=$membershipProductID membership_entitlement=$membershipEntitlementID email_verification_blocked=$emailVerificationAttachmentBlocked price_update_email_verification_blocked=$emailVerificationPriceUpdateBlocked"
} finally {
  if ($null -ne $author -and $null -ne $author.Headers) {
    foreach ($id in @($attachmentID, $missingObjectAttachmentID, $archivedTopicAttachmentID) | Where-Object { $_ -gt 0 -and $archivedAttachmentIDs -notcontains $_ }) {
      try {
        Invoke-Api -Uri "$baseUrl/api/v1/attachments/$id" -Method Delete -Headers $author.Headers -TimeoutSec 15 | Out-Null
      } catch {
        Write-Warning "Could not archive test attachment ${id}: $($_.Exception.Message)"
      }
    }
  }
  if (-not $SkipMinIOVerification -and $topicID -gt 0) {
    try {
      Remove-MinIOTopicObjects -TopicID $topicID
    } catch {
      Write-Warning "Could not clean MinIO test objects for topic ${topicID}: $($_.Exception.Message)"
    }
  }
  if (-not $SkipMinIOVerification -and -not [string]::IsNullOrWhiteSpace($MinIOContainer)) {
    & docker.exe exec $MinIOContainer rm -rf $minioMcConfigDirectory | Out-Null
  }
  if (Test-Path -LiteralPath $tempDirectory) {
    Remove-Item -LiteralPath $tempDirectory -Recurse -Force
  }
}
