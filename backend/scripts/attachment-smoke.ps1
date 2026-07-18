param(
  [string]$BaseUrl = "http://127.0.0.1:18080",
  [string]$AdminUsername = "admin",
  [string]$AdminPassword = "Admin123!",
  [string]$MinIOContainer = "bbs-local-minio",
  [string]$MinIOBucket = "bbs-local",
  [string]$MinIOAccessKey = "minioadmin",
  [string]$MinIOSecretKey = "minioadmin",
  [switch]$SkipMinIOVerification
)

$ErrorActionPreference = "Stop"

$baseUrl = $BaseUrl.TrimEnd("/")
$stamp = Get-Date -Format "yyMMddHHmmssfff"
$priceCredits = 5000
$buyerTopUp = 10000
$tempDirectory = Join-Path ([System.IO.Path]::GetTempPath()) "bbs-attachment-smoke-$stamp"
$sourceFile = Join-Path $tempDirectory "paid-attachment.txt"
$downloadFile = Join-Path $tempDirectory "downloaded-attachment.txt"
$downloadHeadersFile = Join-Path $tempDirectory "download.headers"
$attachmentID = 0
$missingObjectAttachmentID = 0
$archivedTopicAttachmentID = 0
$topicID = 0
$minioObjectKeys = @()
$archivedAttachmentIDs = @()
$author = $null

if ([string]::IsNullOrWhiteSpace($MinIOBucket) -or [string]::IsNullOrWhiteSpace($MinIOAccessKey) -or [string]::IsNullOrWhiteSpace($MinIOSecretKey)) {
  throw "MinIO bucket and credentials must not be empty"
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
  $lines = & docker.exe exec $MinIOContainer mc ls --json local
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

  $lines = & docker.exe exec $MinIOContainer mc ls --recursive --json "local/$MinIOBucket/topics/$TopicID/"
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

  & docker.exe exec $MinIOContainer mc rm --force "local/$MinIOBucket/$ObjectKey" | Out-Null
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
  & docker.exe exec $MinIOContainer mc rm --recursive --force "local/$MinIOBucket/topics/$TopicID/" | Out-Null
}

function Set-MinIOAlias {
  & docker.exe exec $MinIOContainer mc alias set local "http://127.0.0.1:9000" $MinIOAccessKey $MinIOSecretKey | Out-Null
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
      throw "MinIO container '$MinIOContainer' is not available. Use -SkipMinIOVerification only when external MinIO is intentionally used."
    }
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
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $buyer.Headers -OutputFile $downloadFile -HeadersFile $downloadHeadersFile -ExpectedStatus 200
  $buyerBalanceAfterSecondDownload = Get-CreditBalance -Headers $buyer.Headers
  if ($buyerBalanceAfterSecondDownload -ne $buyerBalanceAfterFirstDownload) {
    throw "Repeated attachment download charged credits more than once"
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
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $insufficientBuyer.Headers -OutputFile (Join-Path $tempDirectory "insufficient-download.body") -HeadersFile $downloadHeadersFile -ExpectedStatus 412
  $insufficientBalanceAfter = Get-CreditBalance -Headers $insufficientBuyer.Headers
  if ($insufficientBalanceAfter -ne $insufficientBalanceBefore) {
    throw "Insufficient-credit attachment download changed the buyer balance"
  }
  $insufficientBuyerDownloadHistory = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $insufficientBuyer.Headers -TimeoutSec 15
  if (@($insufficientBuyerDownloadHistory.items | Where-Object { $null -ne $_ }).Count -ne 0) {
    throw "Pending attachment download was exposed in download history"
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

  $archived = Invoke-Api -Uri "$baseUrl/api/v1/attachments/$attachmentID" -Method Delete -Headers $author.Headers -TimeoutSec 15
  if ($archived.status -ne "ARCHIVED") {
    throw "Attachment archive did not return ARCHIVED status"
  }
  $archivedAttachmentIDs += $attachmentID
  $buyerBalanceBeforeArchivedDownload = Get-CreditBalance -Headers $buyer.Headers
  Invoke-Download -Uri "$baseUrl/api/v1/attachments/$attachmentID/download" -Headers $buyer.Headers -OutputFile (Join-Path $tempDirectory "archived-download.body") -HeadersFile $downloadHeadersFile -ExpectedStatus 412
  $buyerBalanceAfterArchivedDownload = Get-CreditBalance -Headers $buyer.Headers
  if ($buyerBalanceAfterArchivedDownload -ne $buyerBalanceBeforeArchivedDownload) {
    throw "Archived attachment download changed the buyer balance"
  }
  $buyerDownloadHistoryAfterArchive = Invoke-Api -Uri "$baseUrl/api/v1/attachments/downloads?limit=10&offset=0" -Method Get -Headers $buyer.Headers -TimeoutSec 15
  $buyerArchivedDownloadRecord = @($buyerDownloadHistoryAfterArchive.items | Where-Object { [int64]$_.attachment.id -eq $attachmentID }) | Select-Object -First 1
  if (-not $buyerArchivedDownloadRecord -or $buyerArchivedDownloadRecord.attachment.status -ne "ARCHIVED") {
    throw "Archived attachment was not retained in buyer download history"
  }

  if ($missingObjectAttachmentID -gt 0) {
    Invoke-Api -Uri "$baseUrl/api/v1/attachments/$missingObjectAttachmentID" -Method Delete -Headers $author.Headers -TimeoutSec 15 | Out-Null
    $archivedAttachmentIDs += $missingObjectAttachmentID
  }

  $archivedTopicUpload = Invoke-MultipartApi -Uri "$baseUrl/api/v1/topics/$topicID/attachments" -Headers $author.Headers -FilePath $sourceFile -Filename "topic-archived.txt" -PriceCredits $priceCredits
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

  Write-Host "Attachment smoke passed: topic=$topicID attachment=$attachmentID buyer=$($buyer.Id) email_verification_blocked=$emailVerificationAttachmentBlocked"
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
  if (Test-Path -LiteralPath $tempDirectory) {
    Remove-Item -LiteralPath $tempDirectory -Recurse -Force
  }
}
