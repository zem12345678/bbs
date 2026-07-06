param(
  [int]$GatewayPort = 18080,
  [switch]$SkipBuild,
  [switch]$KeepRunning,
  [int]$ProjectionRetries = 60
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$ServicesRoot = Join-Path $RepoRoot "backend\services"
$Services = @(
  @{ Name = "user-service"; Port = 9102 },
  @{ Name = "content-service"; Port = 9103 },
  @{ Name = "comment-service"; Port = 9104 },
  @{ Name = "reaction-service"; Port = 9105 },
  @{ Name = "admin-service"; Port = 9114 },
  @{ Name = "search-service"; Port = 9106 },
  @{ Name = "credit-service"; Port = 9107 },
  @{ Name = "notification-service"; Port = 9108 },
  @{ Name = "feed-service"; Port = 9113 }
)

$Started = New-Object System.Collections.Generic.List[System.Diagnostics.Process]

if ($ProjectionRetries -lt 1) {
  throw "ProjectionRetries must be greater than 0"
}

function Test-PortListening {
  param([int]$Port)

  $client = New-Object System.Net.Sockets.TcpClient
  try {
    $async = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
    if ($async.AsyncWaitHandle.WaitOne(300)) {
      $client.EndConnect($async)
      return $true
    }
    return $false
  } catch {
    return $false
  } finally {
    $client.Close()
  }
}

function Get-AvailableTcpPort {
  $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
  try {
    $listener.Start()
    return ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
  } finally {
    $listener.Stop()
  }
}

function Invoke-GoBuild {
  param([string]$ServiceName)

  $serviceDir = Join-Path $ServicesRoot $ServiceName
  $buildTarget = ".\cmd\server"
  if ($ServiceName -eq "admin-service" -or $ServiceName -eq "content-service" -or $ServiceName -eq "reaction-service") {
    $buildTarget = ".\cmd"
  }
  Push-Location $serviceDir
  try {
    go build -o "bin\$ServiceName.exe" $buildTarget
  } finally {
    Pop-Location
  }
}

function Wait-Port {
  param(
    [int]$Port,
    [int]$TimeoutSeconds = 45
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    $client = New-Object System.Net.Sockets.TcpClient
    try {
      $async = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
      if ($async.AsyncWaitHandle.WaitOne(500)) {
        $client.EndConnect($async)
        return
      }
    } catch {
    } finally {
      $client.Close()
    }
    Start-Sleep -Milliseconds 300
  }
  throw "Timed out waiting for TCP port $Port"
}

function Wait-Http {
  param(
    [string]$Url,
    [int]$TimeoutSeconds = 45
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    try {
      Invoke-RestMethod -Uri $Url -Method Get -TimeoutSec 2 | Out-Null
      return
    } catch {
      Start-Sleep -Milliseconds 500
    }
  }
  throw "Timed out waiting for $Url"
}

function Invoke-Api {
  $result = Microsoft.PowerShell.Utility\Invoke-RestMethod @args
  if ($null -ne $result) {
    $propertyNames = @($result.PSObject.Properties | ForEach-Object { $_.Name })
    if ($propertyNames -contains "http_code" -and $propertyNames -contains "code" -and $propertyNames -contains "data") {
      if ([int64]$result.code -ne 0) {
        throw "API error $($result.code): $($result.message)"
      }
      return $result.data
    }
  }
  return $result
}

function Assert-ApiForbidden {
  try {
    Microsoft.PowerShell.Utility\Invoke-RestMethod @args | Out-Null
  } catch {
    $statusCode = $null
    if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
      $statusCode = [int]$_.Exception.Response.StatusCode
    }
    if ($statusCode -eq 403) {
      return
    }
    throw
  }
  throw "Expected API request to be forbidden"
}

function Assert-ApiStatus {
  param(
    [int]$ExpectedStatus
  )
  try {
    Microsoft.PowerShell.Utility\Invoke-RestMethod @args | Out-Null
  } catch {
    $statusCode = $null
    if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
      $statusCode = [int]$_.Exception.Response.StatusCode
    }
    if ($statusCode -eq $ExpectedStatus) {
      return
    }
    throw
  }
  throw "Expected API request to return HTTP $ExpectedStatus"
}

function Assert-ObjectProperty {
  param(
    [object]$Object,
    [string]$Property,
    [string]$Message
  )

  if ($null -eq $Object) {
    throw $Message
  }
  $propertyNames = @($Object.PSObject.Properties | ForEach-Object { $_.Name })
  if ($propertyNames -notcontains $Property) {
    throw $Message
  }
}

function Invoke-ServiceMigrate {
  param([string]$ServiceName)

  $serviceDir = Join-Path $ServicesRoot $ServiceName
  Push-Location $serviceDir
  try {
    & (Join-Path $serviceDir "bin\$ServiceName.exe") "migrate" "-c" "configs/config.yaml"
    if ($LASTEXITCODE -ne 0) {
      throw "$ServiceName migrate failed with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }
}

function Invoke-ReactionCacheRebuild {
  $serviceDir = Join-Path $ServicesRoot "reaction-service"
  Push-Location $serviceDir
  try {
    $output = & (Join-Path $serviceDir "bin\reaction-service.exe") "rebuild-cache" "-c" "configs/config.yaml" "--verify"
    if ($LASTEXITCODE -ne 0) {
      throw "reaction-service rebuild-cache failed with exit code $LASTEXITCODE"
    }
    return ($output | ConvertFrom-Json)
  } finally {
    Pop-Location
  }
}

function Start-ServiceProcess {
  param(
    [string]$ServiceName,
    [int]$Port
  )

  if ($Port -gt 0 -and (Test-PortListening $Port)) {
    Write-Host "$ServiceName already listens on port $Port; reusing existing process."
    return
  }

  $serviceDir = Join-Path $ServicesRoot $ServiceName
  $logsDir = Join-Path $serviceDir "logs"
  New-Item -ItemType Directory -Force -Path $logsDir | Out-Null
  $argumentList = @("-config", "configs/config.yaml")
  if ($ServiceName -eq "admin-service" -or $ServiceName -eq "content-service" -or $ServiceName -eq "reaction-service") {
    $argumentList = @("server", "-c", "configs/config.yaml")
  }
  $process = Start-Process `
    -FilePath (Join-Path $serviceDir "bin\$ServiceName.exe") `
    -ArgumentList $argumentList `
    -WorkingDirectory $serviceDir `
    -RedirectStandardOutput (Join-Path $logsDir "smoke-out.log") `
    -RedirectStandardError (Join-Path $logsDir "smoke-err.log") `
    -WindowStyle Hidden `
    -PassThru
  $Started.Add($process)
}

function Stop-StartedProcesses {
  foreach ($process in $Started) {
    try {
      if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit(5000) | Out-Null
      }
    } catch {
    }
  }
}

try {
  if (Test-PortListening $GatewayPort) {
    $previousPort = $GatewayPort
    $GatewayPort = Get-AvailableTcpPort
    Write-Host "Gateway port $previousPort is in use; using $GatewayPort for smoke."
  }

  if (-not $SkipBuild) {
    foreach ($service in $Services) {
      Invoke-GoBuild $service.Name
    }
    Invoke-GoBuild "api-gateway"
  }

  Invoke-ServiceMigrate "user-service"
  Invoke-ServiceMigrate "admin-service"
  Invoke-ServiceMigrate "content-service"
  Invoke-ServiceMigrate "reaction-service"

  foreach ($service in $Services) {
    Start-ServiceProcess $service.Name $service.Port
  }
  foreach ($service in $Services) {
    Wait-Port $service.Port
  }

  $gatewayDir = Join-Path $ServicesRoot "api-gateway"
  $gatewayConfig = Join-Path $env:TEMP "bbs-api-gateway-smoke-$GatewayPort.yaml"
  $gatewaySource = Get-Content -Path (Join-Path $gatewayDir "configs\config.yaml") -Raw
  ($gatewaySource -replace "(?m)^  httpPort: .+$", "  httpPort: $GatewayPort") |
    Set-Content -Path $gatewayConfig -Encoding UTF8

  $gatewayLogsDir = Join-Path $gatewayDir "logs"
  New-Item -ItemType Directory -Force -Path $gatewayLogsDir | Out-Null
  $gateway = Start-Process `
    -FilePath (Join-Path $gatewayDir "bin\api-gateway.exe") `
    -ArgumentList @("-config", $gatewayConfig) `
    -WorkingDirectory $gatewayDir `
    -RedirectStandardOutput (Join-Path $gatewayLogsDir "smoke-out.log") `
    -RedirectStandardError (Join-Path $gatewayLogsDir "smoke-err.log") `
    -WindowStyle Hidden `
    -PassThru
  $Started.Add($gateway)

  $baseUrl = "http://127.0.0.1:$GatewayPort"
  Wait-Http "$baseUrl/healthz"

  $authConfig = Invoke-Api -Uri "$baseUrl/api/v1/auth/config" -Method Get -TimeoutSec 10
  foreach ($property in @("password_enabled", "register_enabled", "webmaster_enabled", "oauth_callback_hint", "providers")) {
    Assert-ObjectProperty $authConfig $property "Auth config did not include $property"
  }
  $authProviderNames = @($authConfig.providers | ForEach-Object { $_.provider })
  foreach ($providerName in @("github", "qq", "google")) {
    if ($authProviderNames -notcontains $providerName) {
      throw "Auth config did not include $providerName provider"
    }
    $provider = @($authConfig.providers | Where-Object { $_.provider -eq $providerName })[0]
    foreach ($property in @("enabled", "label", "start_url")) {
      Assert-ObjectProperty $provider $property "Auth config provider $providerName did not include $property"
    }
    if ([string]::IsNullOrWhiteSpace([string]$provider.start_url)) {
      throw "Auth config provider $providerName did not include start_url"
    }
  }
  $githubAuthProvider = @($authConfig.providers | Where-Object { $_.provider -eq "github" })[0]
  Assert-ObjectProperty $githubAuthProvider "min_account_years" "Auth config GitHub provider did not include min_account_years"

  $stamp = Get-Date -Format "yyyyMMddHHmmss"
  $username = "smoke$stamp"
  $password = "Password123!"
  $registerBody = @{
    username = $username
    email = "$username@example.com"
    password = $password
    nickname = "Smoke User"
  } | ConvertTo-Json
  $register = Invoke-Api -Uri "$baseUrl/api/v1/auth/register" -Method Post -ContentType "application/json" -Body $registerBody -TimeoutSec 10
  $token = $register.access_token
  if (-not $token) {
    throw "Register response did not include access_token"
  }

  $headers = @{ Authorization = "Bearer $token" }
  $me = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Get -Headers $headers -TimeoutSec 10
  if (-not $me.user.id) {
    throw "Current user response did not include user.id"
  }
  $userBadges = Invoke-Api -Uri "$baseUrl/api/v1/users/$($me.user.id)/badges?limit=20&offset=0" -Method Get -TimeoutSec 10
  $memberBadgeListed = $false
  foreach ($item in @($userBadges.items)) {
    if ($item.id -eq "community-member") {
      $memberBadgeListed = $true
    }
  }
  if (-not $memberBadgeListed) {
    throw "User badge list did not include community-member badge"
  }

  $profileBody = @{
    nickname = "Smoke Updated"
    avatar_url = ""
    bio = "Local smoke profile"
  } | ConvertTo-Json
  $profile = Invoke-Api -Uri "$baseUrl/api/v1/users/me" -Method Put -Headers $headers -ContentType "application/json" -Body $profileBody -TimeoutSec 10
  if ($profile.user.nickname -ne "Smoke Updated") {
    throw "Profile update did not persist nickname"
  }

  $followeeUsername = "target$stamp"
  $followeeBody = @{
    username = $followeeUsername
    email = "$followeeUsername@example.com"
    password = $password
    nickname = "Follow Target"
  } | ConvertTo-Json
  $followee = Invoke-Api -Uri "$baseUrl/api/v1/auth/register" -Method Post -ContentType "application/json" -Body $followeeBody -TimeoutSec 10
  $followeeId = $followee.user.id
  if (-not $followeeId) {
    throw "Follow target response did not include user.id"
  }
  $followeeToken = $followee.access_token
  if (-not $followeeToken) {
    throw "Follow target response did not include access_token"
  }
  $followeeHeaders = @{ Authorization = "Bearer $followeeToken" }

  $adminUsername = "admin"
  $adminDefaultPassword = "Admin123!"
  $adminLoginBody = @{
    account = $adminUsername
    password = $adminDefaultPassword
  } | ConvertTo-Json
  $admin = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/login" -Method Post -ContentType "application/json" -Body $adminLoginBody -TimeoutSec 10
  $adminToken = $admin.access_token
  if (-not $adminToken) {
    throw "Admin login response did not include access_token"
  }
  $adminHeaders = @{ Authorization = "Bearer $adminToken" }
  $adminProfile = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/profile" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminProfile.roles) -notcontains "admin") {
    throw "Admin profile did not include admin role"
  }
  $adminPerms = @($adminProfile.permissions)
  if (
    $adminPerms -notcontains "governance:list_articles" -or
    $adminPerms -notcontains "governance:archive_article" -or
    $adminPerms -notcontains "governance:list_topics" -or
    $adminPerms -notcontains "governance:hide_topic" -or
    $adminPerms -notcontains "governance:archive_topic" -or
    $adminPerms -notcontains "governance:list_users" -or
    $adminPerms -notcontains "governance:list_comments" -or
    $adminPerms -notcontains "governance:hide_comment" -or
    $adminPerms -notcontains "governance:list_admin_users" -or
    $adminPerms -notcontains "governance:create_admin_user" -or
    $adminPerms -notcontains "governance:list_roles" -or
    $adminPerms -notcontains "governance:assign_roles" -or
    $adminPerms -notcontains "governance:list_badges" -or
    $adminPerms -notcontains "governance:create_badge" -or
    $adminPerms -notcontains "governance:update_badge" -or
    $adminPerms -notcontains "governance:delete_badge" -or
    $adminPerms -notcontains "governance:list_levels" -or
    $adminPerms -notcontains "governance:create_level" -or
    $adminPerms -notcontains "governance:update_level" -or
    $adminPerms -notcontains "governance:delete_level" -or
    $adminPerms -notcontains "governance:list_forbidden_words" -or
    $adminPerms -notcontains "governance:create_forbidden_word" -or
    $adminPerms -notcontains "governance:update_forbidden_word" -or
    $adminPerms -notcontains "governance:delete_forbidden_word" -or
    $adminPerms -notcontains "governance:list_settings" -or
    $adminPerms -notcontains "governance:update_setting" -or
    $adminPerms -notcontains "governance:list_email_logs" -or
    $adminPerms -notcontains "governance:list_links" -or
    $adminPerms -notcontains "governance:create_link" -or
    $adminPerms -notcontains "governance:update_link" -or
    $adminPerms -notcontains "governance:delete_link" -or
    $adminPerms -notcontains "governance:list_tasks" -or
    $adminPerms -notcontains "governance:create_task" -or
    $adminPerms -notcontains "governance:update_task" -or
    $adminPerms -notcontains "governance:delete_task"
  ) {
    throw "Admin profile did not include expected governance permissions"
  }

  $governanceUsers = Invoke-Api -Uri "$baseUrl/api/v1/admin/users?query=$username&status=0&page=1&page_size=20" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $governanceUserListed = $false
  foreach ($item in @($governanceUsers.items)) {
    if ([string]$item.id -eq [string]$me.user.id) {
      $governanceUserListed = $true
    }
  }
  if (-not $governanceUserListed) {
    throw "Admin user governance list did not include registered smoke user"
  }

  $roles = Invoke-Api -Uri "$baseUrl/api/v1/admin/rbac/roles" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $roleKeys = @($roles.items | ForEach-Object { $_.key })
  if ($roleKeys -notcontains "admin" -or $roleKeys -notcontains "moderator") {
    throw "Admin RBAC role list did not include expected roles"
  }
  $adminUsers = Invoke-Api -Uri "$baseUrl/api/v1/admin/rbac/users?limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $bootstrapAdminListed = $false
  foreach ($item in @($adminUsers.items)) {
    if ($item.username -eq $adminUsername) {
      $bootstrapAdminListed = $true
    }
  }
  if (-not $bootstrapAdminListed) {
    throw "Admin RBAC user list did not include bootstrap admin"
  }
  $rbacUsername = "rbac$stamp"
  $rbacPassword = "Admin123!$stamp"
  $createAdminBody = @{
    username = $rbacUsername
    email = "$rbacUsername@admin.local"
    phone = ""
    password = $rbacPassword
    role_keys = @("moderator")
  } | ConvertTo-Json
  $createdAdmin = Invoke-Api -Uri "$baseUrl/api/v1/admin/rbac/users" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $createAdminBody -TimeoutSec 10
  if (-not $createdAdmin.user.id -or @($createdAdmin.user.roles) -notcontains "moderator") {
    throw "Admin RBAC create user did not return moderator admin user"
  }
  $assignAdminBody = @{
    role_keys = @("admin")
  } | ConvertTo-Json
  $assignedAdmin = Invoke-Api -Uri "$baseUrl/api/v1/admin/rbac/users/$($createdAdmin.user.id)/roles" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $assignAdminBody -TimeoutSec 10
  if (@($assignedAdmin.user.roles) -notcontains "admin") {
    throw "Admin RBAC assign roles did not return admin role"
  }
  $createdAdminLoginBody = @{
    account = $rbacUsername
    password = $rbacPassword
  } | ConvertTo-Json
  $createdAdminLogin = Invoke-Api -Uri "$baseUrl/api/v1/admin/auth/login" -Method Post -ContentType "application/json" -Body $createdAdminLoginBody -TimeoutSec 10
  if (-not $createdAdminLogin.access_token -or @($createdAdminLogin.roles) -notcontains "admin") {
    throw "Created admin could not login with assigned admin role"
  }

  Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/follow" -Method Post -Headers $headers -TimeoutSec 10 | Out-Null
  $followState = Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/following-state" -Method Get -Headers $headers -TimeoutSec 10
  if (-not $followState.following) {
    throw "Follow state was not true after follow"
  }
  $followers = Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/followers?page=1&page_size=10" -Method Get -TimeoutSec 10
  if (@($followers.items).Count -lt 1) {
    throw "Followers list did not include any follower after follow"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/follow" -Method Delete -Headers $headers -TimeoutSec 10 | Out-Null
  $unfollowState = Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/following-state" -Method Get -Headers $headers -TimeoutSec 10
  if ($unfollowState.following) {
    throw "Follow state was still true after unfollow"
  }
  Assert-ApiStatus 401 -Uri "$baseUrl/api/v1/feed?sort=follow&limit=10&offset=0" -Method Get -TimeoutSec 10
  Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/follow" -Method Post -Headers $headers -TimeoutSec 10 | Out-Null
  $refollowState = Invoke-Api -Uri "$baseUrl/api/v1/users/$followeeId/following-state" -Method Get -Headers $headers -TimeoutSec 10
  if (-not $refollowState.following) {
    throw "Follow state was not true after refollow for following feed"
  }

  $categories = Invoke-Api -Uri "$baseUrl/api/v1/categories?status=2&limit=20&offset=0" -Method Get -TimeoutSec 10
  $defaultCategory = $null
  foreach ($item in @($categories.items)) {
    if ($item.slug -eq "general") {
      $defaultCategory = $item
    }
  }
  if (-not $defaultCategory -or -not $defaultCategory.id) {
    throw "Category list did not include seeded general category"
  }
  $categoryId = [int64]$defaultCategory.id
  $categoryDetail = Invoke-Api -Uri "$baseUrl/api/v1/categories/$categoryId" -Method Get -TimeoutSec 10
  if ($categoryDetail.category.slug -ne "general") {
    throw "Category detail did not return seeded general category"
  }
  $links = Invoke-Api -Uri "$baseUrl/api/v1/links?limit=20&offset=0" -Method Get -TimeoutSec 10
  if (@($links.items).Count -lt 1) {
    throw "Links endpoint did not return persisted links"
  }
  $tasks = Invoke-Api -Uri "$baseUrl/api/v1/tasks?limit=20&offset=0" -Method Get -TimeoutSec 10
  if (@($tasks.items).Count -lt 1) {
    throw "Tasks endpoint did not return persisted tasks"
  }
  $adminCategories = Invoke-Api -Uri "$baseUrl/api/v1/admin/categories?status=0&limit=50&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $adminCategoryListed = $false
  foreach ($item in @($adminCategories.items)) {
    if ($item.slug -eq "general") {
      $adminCategoryListed = $true
    }
  }
  if (-not $adminCategoryListed) {
    throw "Admin category list did not include seeded general category"
  }
  $adminBadges = Invoke-Api -Uri "$baseUrl/api/v1/admin/badges?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminBadges.items).Count -lt 1) {
    throw "Admin badges endpoint did not return persisted badges"
  }
  $badgeBody = @{
    key = "smoke-badge-$stamp"
    name = "Smoke Badge $stamp"
    description = "Created by local smoke test."
    rule_type = "manual"
    rule_value = 0
    status = 2
    sort = 90
  } | ConvertTo-Json
  $createdBadge = Invoke-Api -Uri "$baseUrl/api/v1/admin/badges" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $badgeBody -TimeoutSec 10
  $createdBadgeId = $createdBadge.badge.id
  if (-not $createdBadgeId) {
    throw "Admin badge create did not return badge.id"
  }
  $updateBadgeBody = @{
    name = "Smoke Badge Updated $stamp"
    description = "Updated by local smoke test."
    rule_type = "manual"
    rule_value = 0
    status = 2
    sort = 91
  } | ConvertTo-Json
  $updatedBadge = Invoke-Api -Uri "$baseUrl/api/v1/admin/badges/$createdBadgeId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateBadgeBody -TimeoutSec 10
  if ($updatedBadge.badge.name -ne "Smoke Badge Updated $stamp") {
    throw "Admin badge update did not persist name"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/badges/$createdBadgeId" -Method Delete -Headers $adminHeaders -TimeoutSec 10 | Out-Null

  $adminLevels = Invoke-Api -Uri "$baseUrl/api/v1/admin/levels?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminLevels.items).Count -lt 1) {
    throw "Admin levels endpoint did not return persisted levels"
  }
  $levelBody = @{
    key = "smoke-level-$stamp"
    name = "Smoke Level $stamp"
    description = "Created by local smoke test."
    min_score = 777
    max_score = 888
    status = 2
    sort = 90
  } | ConvertTo-Json
  $createdLevel = Invoke-Api -Uri "$baseUrl/api/v1/admin/levels" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $levelBody -TimeoutSec 10
  $createdLevelId = $createdLevel.level.id
  if (-not $createdLevelId) {
    throw "Admin level create did not return level.id"
  }
  $updateLevelBody = @{
    name = "Smoke Level Updated $stamp"
    description = "Updated by local smoke test."
    min_score = 778
    max_score = 889
    status = 2
    sort = 91
  } | ConvertTo-Json
  $updatedLevel = Invoke-Api -Uri "$baseUrl/api/v1/admin/levels/$createdLevelId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateLevelBody -TimeoutSec 10
  if ($updatedLevel.level.min_score -ne 778) {
    throw "Admin level update did not persist min_score"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/levels/$createdLevelId" -Method Delete -Headers $adminHeaders -TimeoutSec 10 | Out-Null

  $adminForbiddenWords = Invoke-Api -Uri "$baseUrl/api/v1/admin/forbidden-words?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminForbiddenWords.items).Count -lt 1) {
    throw "Admin forbidden words endpoint did not return persisted rows"
  }
  $forbiddenWordBody = @{
    word = "smoke-word-$stamp"
    scene = "content"
    action = "review"
    replacement = ""
    description = "Created by local smoke test."
    status = 2
  } | ConvertTo-Json
  $createdForbiddenWord = Invoke-Api -Uri "$baseUrl/api/v1/admin/forbidden-words" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $forbiddenWordBody -TimeoutSec 10
  $createdForbiddenWordId = $createdForbiddenWord.word.id
  if (-not $createdForbiddenWordId) {
    throw "Admin forbidden word create did not return word.id"
  }
  $updateForbiddenWordBody = @{
    word = "smoke-word-updated-$stamp"
    scene = "comment"
    action = "replace"
    replacement = "***"
    description = "Updated by local smoke test."
    status = 2
  } | ConvertTo-Json
  $updatedForbiddenWord = Invoke-Api -Uri "$baseUrl/api/v1/admin/forbidden-words/$createdForbiddenWordId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateForbiddenWordBody -TimeoutSec 10
  if ($updatedForbiddenWord.word.action -ne "replace") {
    throw "Admin forbidden word update did not persist action"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/forbidden-words/$createdForbiddenWordId" -Method Delete -Headers $adminHeaders -TimeoutSec 10 | Out-Null

  $adminSettings = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings?limit=100&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminSettings.items).Count -lt 1 -or -not $adminSettings.settings.site_name) {
    throw "Admin settings endpoint did not return seeded site settings"
  }
  $settingBody = @{
    key = "smoke_setting"
    value = "enabled"
    group = "smoke"
    value_type = "string"
    description = "Created by local smoke test."
    status = 2
  } | ConvertTo-Json
  $updatedSetting = Invoke-Api -Uri "$baseUrl/api/v1/admin/settings/smoke_setting" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $settingBody -TimeoutSec 10
  if ($updatedSetting.setting.value -ne "enabled") {
    throw "Admin setting update did not persist value"
  }

  $adminEmailLogs = Invoke-Api -Uri "$baseUrl/api/v1/admin/email-logs?limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if ($null -eq $adminEmailLogs.items -or $null -eq $adminEmailLogs.total) {
    throw "Admin email logs endpoint did not return a list response"
  }

  $adminLinks = Invoke-Api -Uri "$baseUrl/api/v1/admin/links?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminLinks.items).Count -lt 1) {
    throw "Admin links endpoint did not return persisted links"
  }
  $linkBody = @{
    key = "smoke-link-$stamp"
    title = "Smoke Link $stamp"
    url = "https://example.com/$stamp"
    description = "Created by local smoke test."
    status = 2
    sort = 90
  } | ConvertTo-Json
  $createdLink = Invoke-Api -Uri "$baseUrl/api/v1/admin/links" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $linkBody -TimeoutSec 10
  $createdLinkId = $createdLink.link.id
  if (-not $createdLinkId) {
    throw "Admin link create did not return link.id"
  }
  $updateLinkBody = @{
    title = "Smoke Link Updated $stamp"
    url = "https://example.com/$stamp/updated"
    description = "Updated by local smoke test."
    status = 2
    sort = 91
  } | ConvertTo-Json
  $updatedLink = Invoke-Api -Uri "$baseUrl/api/v1/admin/links/$createdLinkId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateLinkBody -TimeoutSec 10
  if ($updatedLink.link.title -ne "Smoke Link Updated $stamp") {
    throw "Admin link update did not persist title"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/links/$createdLinkId" -Method Delete -Headers $adminHeaders -TimeoutSec 10 | Out-Null

  $adminTasks = Invoke-Api -Uri "$baseUrl/api/v1/admin/tasks?status=0&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  if (@($adminTasks.items).Count -lt 1) {
    throw "Admin tasks endpoint did not return persisted tasks"
  }
  $taskBody = @{
    key = "smoke-task-$stamp"
    title = "Smoke Task $stamp"
    description = "Created by local smoke test."
    reward_points = 7
    status = 2
    sort = 90
  } | ConvertTo-Json
  $createdTask = Invoke-Api -Uri "$baseUrl/api/v1/admin/tasks" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $taskBody -TimeoutSec 10
  $createdTaskId = $createdTask.task.id
  if (-not $createdTaskId) {
    throw "Admin task create did not return task.id"
  }
  $updateTaskBody = @{
    title = "Smoke Task Updated $stamp"
    description = "Updated by local smoke test."
    reward_points = 9
    status = 2
    sort = 91
  } | ConvertTo-Json
  $updatedTask = Invoke-Api -Uri "$baseUrl/api/v1/admin/tasks/$createdTaskId" -Method Put -Headers $adminHeaders -ContentType "application/json" -Body $updateTaskBody -TimeoutSec 10
  if ($updatedTask.task.reward_points -ne 9) {
    throw "Admin task update did not persist reward_points"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/tasks/$createdTaskId" -Method Delete -Headers $adminHeaders -TimeoutSec 10 | Out-Null

  $permissionTopicBody = @{
    slug = "permission-topic-$stamp"
    type = "topic"
    title = "Permission topic $stamp"
    body = "This topic validates author-only updates and deletes."
    tags = @("permission", "topic")
    category_id = $categoryId
    publish = $true
  } | ConvertTo-Json
  $permissionTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $permissionTopicBody -TimeoutSec 10
  $permissionTopicId = $permissionTopic.topic.id
  if (-not $permissionTopicId) {
    throw "Permission topic create response did not include topic.id"
  }
  $permissionTopicUpdateBody = @{
    title = "Permission topic updated $stamp"
    body = "Updated by the owner."
    tags = @("permission", "updated")
    category_id = $categoryId
  } | ConvertTo-Json
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics/$permissionTopicId" -Method Put -Headers $followeeHeaders -ContentType "application/json" -Body $permissionTopicUpdateBody -TimeoutSec 10
  $updatedPermissionTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics/$permissionTopicId" -Method Put -Headers $headers -ContentType "application/json" -Body $permissionTopicUpdateBody -TimeoutSec 10
  if ($updatedPermissionTopic.topic.title -ne "Permission topic updated $stamp") {
    throw "Owner topic update did not persist title"
  }
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics/$permissionTopicId" -Method Delete -Headers $followeeHeaders -TimeoutSec 10
  $deletedPermissionTopic = Invoke-Api -Uri "$baseUrl/api/v1/topics/$permissionTopicId" -Method Delete -Headers $headers -TimeoutSec 10
  if ([int64]$deletedPermissionTopic.topic.status -ne 4) {
    throw "Owner topic delete did not archive topic"
  }

  $permissionArticleBody = @{
    slug = "permission-article-$stamp"
    title = "Permission article $stamp"
    summary = "Permission smoke"
    body = "This article validates author-only updates and deletes."
    cover_url = ""
    tags = @("permission", "article")
    publish = $true
  } | ConvertTo-Json
  $permissionArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles" -Method Post -Headers $headers -ContentType "application/json" -Body $permissionArticleBody -TimeoutSec 10
  $permissionArticleId = $permissionArticle.article.id
  if (-not $permissionArticleId) {
    throw "Permission article create response did not include article.id"
  }
  $permissionArticleUpdateBody = @{
    title = "Permission article updated $stamp"
    summary = "Updated by owner"
    body = "Updated by the owner."
    cover_url = ""
    tags = @("permission", "updated")
  } | ConvertTo-Json
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/articles/$permissionArticleId" -Method Put -Headers $followeeHeaders -ContentType "application/json" -Body $permissionArticleUpdateBody -TimeoutSec 10
  $updatedPermissionArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles/$permissionArticleId" -Method Put -Headers $headers -ContentType "application/json" -Body $permissionArticleUpdateBody -TimeoutSec 10
  if ($updatedPermissionArticle.article.title -ne "Permission article updated $stamp") {
    throw "Owner article update did not persist title"
  }
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/articles/$permissionArticleId" -Method Delete -Headers $followeeHeaders -TimeoutSec 10
  $deletedPermissionArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles/$permissionArticleId" -Method Delete -Headers $headers -TimeoutSec 10
  if ([int64]$deletedPermissionArticle.article.status -ne 4) {
    throw "Owner article delete did not archive article"
  }

  $topicTitle = "Topic smoke $stamp"
  $topicBody = @{
    slug = "topic-smoke-$stamp"
    type = "topic"
    title = $topicTitle
    body = "This topic validates the forum topic publishing path."
    tags = @("topic", "smoke")
    category_id = $categoryId
    publish = $true
  } | ConvertTo-Json
  $topic = Invoke-Api -Uri "$baseUrl/api/v1/topics" -Method Post -Headers $headers -ContentType "application/json" -Body $topicBody -TimeoutSec 10
  $topicId = $topic.topic.id
  if (-not $topicId -or [int64]$topic.topic.status -ne 2) {
    throw "Topic create response did not include a published topic"
  }
  $topicDetail = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId" -Method Get -TimeoutSec 10
  if ($topicDetail.topic.title -ne $topicTitle) {
    throw "Topic detail did not return created topic"
  }
  if ([int64]$topicDetail.topic.category_id -ne $categoryId) {
    throw "Topic detail did not include selected category_id"
  }
  $topicList = Invoke-Api -Uri "$baseUrl/api/v1/topics?status=2&type=topic&limit=20&offset=0" -Method Get -TimeoutSec 10
  $topicListed = $false
  foreach ($item in @($topicList.items)) {
    if ([string]$item.id -eq [string]$topicId) {
      $topicListed = $true
    }
  }
  if (-not $topicListed) {
    throw "Topic list did not include created topic"
  }
  $categoryTopics = Invoke-Api -Uri "$baseUrl/api/v1/topics?status=2&type=topic&category_id=$categoryId&limit=20&offset=0" -Method Get -TimeoutSec 10
  $categoryTopicListed = $false
  foreach ($item in @($categoryTopics.items)) {
    if ([string]$item.id -eq [string]$topicId -and [int64]$item.category_id -eq $categoryId) {
      $categoryTopicListed = $true
    }
  }
  if (-not $categoryTopicListed) {
    throw "Category topic list did not include created topic"
  }
  $topicCommentBody = @{
    content = "Topic smoke comment"
    parent_id = 0
  } | ConvertTo-Json
  $topicComment = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $topicCommentBody -TimeoutSec 10
  $topicCommentId = $topicComment.comment.id
  if (-not $topicCommentId) {
    throw "Topic comment response did not include comment.id"
  }
  $topicComments = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/comments?page=1&page_size=20" -Method Get -TimeoutSec 10
  if (@($topicComments.items).Count -lt 1) {
    throw "Topic comments did not include created comment"
  }
  $permissionCommentBody = @{
    content = "Permission smoke comment"
    parent_id = 0
  } | ConvertTo-Json
  $permissionComment = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $permissionCommentBody -TimeoutSec 10
  $permissionCommentId = $permissionComment.comment.id
  if (-not $permissionCommentId) {
    throw "Permission comment response did not include comment.id"
  }
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/comments/$permissionCommentId" -Method Delete -Headers $followeeHeaders -TimeoutSec 10
  Invoke-Api -Uri "$baseUrl/api/v1/comments/$permissionCommentId" -Method Delete -Headers $headers -TimeoutSec 10 | Out-Null
  $permissionHiddenComments = Invoke-Api -Uri "$baseUrl/api/v1/admin/comments?entity_type=topic&entity_id=$topicId&status=0&page=1&page_size=20" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $permissionCommentHidden = $false
  foreach ($item in @($permissionHiddenComments.items)) {
    if ([string]$item.id -eq [string]$permissionCommentId -and [int64]$item.status -eq 0) {
      $permissionCommentHidden = $true
    }
  }
  if (-not $permissionCommentHidden) {
    throw "Owner comment delete did not hide permission comment"
  }
  $topicLike = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/like" -Method Post -Headers $headers -TimeoutSec 10
  if ([int64]$topicLike.count -lt 1) {
    throw "Topic like did not increment count"
  }
  $topicFavorite = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/favorite" -Method Post -Headers $headers -TimeoutSec 10
  if ([int64]$topicFavorite.count -lt 1) {
    throw "Topic favorite did not increment count"
  }
  $topicReactionCounts = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/reactions" -Method Get -TimeoutSec 10
  if ([int64]$topicReactionCounts.like_count -lt 1 -or [int64]$topicReactionCounts.favorite_count -lt 1) {
    throw "Topic reaction counts did not include like and favorite"
  }

  $topicFeed = $null
  $topicFeedItem = $null
  $topicFeedProjected = $false
  $topicFeedCountersProjected = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $topicFeed = Invoke-Api -Uri "$baseUrl/api/v1/feed?limit=20&offset=0" -Method Get -TimeoutSec 10
      foreach ($item in @($topicFeed.items)) {
        if ([string]$item.id -eq [string]$topicId -and $item.entity_type -eq "topic") {
          $topicFeedItem = $item
        }
      }
      if ($topicFeedItem) {
        $topicFeedProjected = $true
      }
      if ($topicFeedItem -and [int64]$topicFeedItem.comment_count -ge 1 -and [int64]$topicFeedItem.like_count -ge 1 -and [int64]$topicFeedItem.favorite_count -ge 1) {
        $topicFeedCountersProjected = $true
        break
      }
    } catch {
    }
  }
  if (-not $topicFeedProjected) {
    throw "Created topic was not projected into feed-service within timeout"
  }
  if (-not $topicFeedCountersProjected) {
    throw "Topic feed item counters were not projected from comment/reaction events"
  }

  $topicSearchIndexed = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $topicSearchUrl = "$baseUrl/api/v1/search/topics?q=$([uri]::EscapeDataString($topicTitle))"
      $topicSearch = Invoke-Api -Uri $topicSearchUrl -Method Get -TimeoutSec 10
      foreach ($item in @($topicSearch.items)) {
        if ([string]$item.topic.id -eq [string]$topicId) {
          $topicSearchIndexed = $true
        }
      }
      if ($topicSearchIndexed) {
        break
      }
    } catch {
    }
  }
  if (-not $topicSearchIndexed) {
    throw "Created topic was not indexed by search-service within timeout"
  }

  $adminTopics = Invoke-Api -Uri "$baseUrl/api/v1/admin/topics?status=2&type=topic&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $adminTopicListed = $false
  foreach ($item in @($adminTopics.items)) {
    if ([string]$item.id -eq [string]$topicId) {
      $adminTopicListed = $true
    }
  }
  if (-not $adminTopicListed) {
    throw "Admin topic list did not include created topic"
  }
  $hiddenTopic = Invoke-Api -Uri "$baseUrl/api/v1/admin/topics/$topicId/hide" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$hiddenTopic.topic.status -ne 3) {
    throw "Admin hide topic did not mark topic as hidden"
  }
  $archivedTopic = Invoke-Api -Uri "$baseUrl/api/v1/admin/topics/$topicId/archive" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$archivedTopic.topic.status -ne 4) {
    throw "Admin archive topic did not mark topic as archived"
  }

  $title = "Frontend smoke post $stamp"
  $articleBody = @{
    slug = "frontend-smoke-$stamp"
    title = $title
    summary = "Gateway smoke"
    body = "This post was created through api-gateway during local smoke testing."
    cover_url = ""
    tags = @("smoke", "frontend")
    publish = $true
  } | ConvertTo-Json
  $article = Invoke-Api -Uri "$baseUrl/api/v1/articles" -Method Post -Headers $followeeHeaders -ContentType "application/json" -Body $articleBody -TimeoutSec 10
  $articleId = $article.article.id
  if (-not $articleId) {
    throw "Create article response did not include article.id"
  }
  $articleDetail = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId" -Method Get -TimeoutSec 10
  if ($articleDetail.article.title -ne $title) {
    throw "Article detail response did not match created article"
  }

  $moderationTitle = "Admin moderation smoke $stamp"
  $moderationBody = @{
    slug = "admin-moderation-smoke-$stamp"
    title = $moderationTitle
    summary = "Admin moderation smoke"
    body = "This post is used to validate admin article moderation."
    cover_url = ""
    tags = @("moderation", "smoke")
    publish = $true
  } | ConvertTo-Json
  $moderationArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles" -Method Post -Headers $followeeHeaders -ContentType "application/json" -Body $moderationBody -TimeoutSec 10
  $moderationArticleId = $moderationArticle.article.id
  if (-not $moderationArticleId) {
    throw "Moderation article response did not include article.id"
  }
  $adminArticles = Invoke-Api -Uri "$baseUrl/api/v1/admin/articles?status=2&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $moderationListed = $false
  foreach ($item in @($adminArticles.items)) {
    if ([string]$item.id -eq [string]$moderationArticleId) {
      $moderationListed = $true
    }
  }
  if (-not $moderationListed) {
    throw "Admin article list did not include moderation article"
  }
  $hiddenArticle = Invoke-Api -Uri "$baseUrl/api/v1/admin/articles/$moderationArticleId/hide" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$hiddenArticle.article.status -ne 3) {
    throw "Admin hide article did not mark article as hidden"
  }
  $archivedArticle = Invoke-Api -Uri "$baseUrl/api/v1/admin/articles/$moderationArticleId/archive" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$archivedArticle.article.status -ne 4) {
    throw "Admin archive article did not mark article as archived"
  }

  $tags = Invoke-Api -Uri "$baseUrl/api/v1/tags?limit=20" -Method Get -TimeoutSec 10
  $tagNames = @($tags.items | ForEach-Object { $_.name })
  if ($tagNames -notcontains "smoke" -or $tagNames -notcontains "frontend") {
    throw "Tag list did not include tags from published article"
  }
  $tagSuggestBody = @{
    query = "front"
    limit = 10
  } | ConvertTo-Json
  $tagSuggest = Invoke-Api -Uri "$baseUrl/api/v1/tags/autocomplete" -Method Post -ContentType "application/json" -Body $tagSuggestBody -TimeoutSec 10
  $suggestNames = @($tagSuggest.items | ForEach-Object { $_.name })
  if ($suggestNames -notcontains "frontend") {
    throw "Tag autocomplete did not include frontend"
  }

  $reportBody = @{
    reason = "content_violation"
    description = "Smoke report"
  } | ConvertTo-Json
  $report = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/report" -Method Post -Headers $headers -ContentType "application/json" -Body $reportBody -TimeoutSec 10
  if (-not $report.report.id) {
    throw "Report response did not include report.id"
  }
  Assert-ApiStatus 401 -Uri "$baseUrl/api/v1/admin/reports?status=1&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $reports = Invoke-Api -Uri "$baseUrl/api/v1/admin/reports?status=1&limit=20&offset=0" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $reportedItem = $null
  foreach ($item in @($reports.items)) {
    if ([string]$item.id -eq [string]$report.report.id) {
      $reportedItem = $item
    }
  }
  if (-not $reportedItem) {
    throw "Admin report list did not include submitted report"
  }
  $auditBody = @{
    status = 2
  } | ConvertTo-Json
  $auditedReport = Invoke-Api -Uri "$baseUrl/api/v1/admin/reports/$($report.report.id)/audit" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $auditBody -TimeoutSec 10
  if ([int64]$auditedReport.report.status -ne 2) {
    throw "Audit report response did not mark report as resolved"
  }

  $commentBody = @{
    content = "Smoke comment"
    parent_id = 0
  } | ConvertTo-Json
  $mutedUser = Invoke-Api -Uri "$baseUrl/api/v1/admin/users/$($me.user.id)/mute" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$mutedUser.user.status -ne 2) {
    throw "Mute user response did not mark user as muted"
  }
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/topics/$topicId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $topicCommentBody -TimeoutSec 10
  Assert-ApiForbidden -Uri "$baseUrl/api/v1/articles/$articleId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $commentBody -TimeoutSec 10
  $unmutedUser = Invoke-Api -Uri "$baseUrl/api/v1/admin/users/$($me.user.id)/unmute" -Method Post -Headers $adminHeaders -TimeoutSec 10
  if ([int64]$unmutedUser.user.status -ne 1) {
    throw "Unmute user response did not mark user as active"
  }
  $moderationCommentBody = @{
    content = "Moderation smoke comment"
    parent_id = 0
  } | ConvertTo-Json
  $moderationComment = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $moderationCommentBody -TimeoutSec 10
  $moderationCommentId = $moderationComment.comment.id
  if (-not $moderationCommentId) {
    throw "Moderation comment response did not include comment.id"
  }
  $adminComments = Invoke-Api -Uri "$baseUrl/api/v1/admin/comments?entity_type=article&entity_id=$articleId&status=-1&page=1&page_size=20" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $moderationCommentListed = $false
  foreach ($item in @($adminComments.items)) {
    if ([string]$item.id -eq [string]$moderationCommentId) {
      $moderationCommentListed = $true
    }
  }
  if (-not $moderationCommentListed) {
    throw "Admin comment list did not include moderation comment"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/admin/comments/$moderationCommentId/hide" -Method Post -Headers $adminHeaders -TimeoutSec 10 | Out-Null
  $hiddenComments = Invoke-Api -Uri "$baseUrl/api/v1/admin/comments?entity_type=article&entity_id=$articleId&status=0&page=1&page_size=20" -Method Get -Headers $adminHeaders -TimeoutSec 10
  $moderationCommentHidden = $false
  foreach ($item in @($hiddenComments.items)) {
    if ([string]$item.id -eq [string]$moderationCommentId -and [int64]$item.status -eq 0) {
      $moderationCommentHidden = $true
    }
  }
  if (-not $moderationCommentHidden) {
    throw "Admin hide comment did not mark moderation comment as hidden"
  }
  $comment = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/comments" -Method Post -Headers $headers -ContentType "application/json" -Body $commentBody -TimeoutSec 10
  $like = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/like" -Method Post -Headers $headers -TimeoutSec 10
  $favorite = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/favorite" -Method Post -Headers $headers -TimeoutSec 10
  $likes = Invoke-Api -Uri "$baseUrl/api/v1/users/current/likes?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $likeTopicListed = $false
  $likeArticleListed = $false
  foreach ($item in @($likes.items)) {
    if ($item.entity.entity_type -eq "topic" -and [string]$item.entity.entity_id -eq [string]$topicId) {
      $likeTopicListed = $true
    }
    if ($item.entity.entity_type -eq "article" -and [string]$item.entity.entity_id -eq [string]$articleId) {
      $likeArticleListed = $true
    }
  }
  if (-not $likeTopicListed -or -not $likeArticleListed) {
    throw "Current user likes did not include liked topic and article"
  }
  $favorites = Invoke-Api -Uri "$baseUrl/api/v1/users/current/favorites?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $favoriteTopicListed = $false
  $favoriteArticleListed = $false
  foreach ($item in @($favorites.items)) {
    if ($item.entity.entity_type -eq "topic" -and [string]$item.entity.entity_id -eq [string]$topicId) {
      $favoriteTopicListed = $true
    }
    if ($item.entity.entity_type -eq "article" -and [string]$item.entity.entity_id -eq [string]$articleId) {
      $favoriteArticleListed = $true
    }
  }
  if (-not $favoriteTopicListed -or -not $favoriteArticleListed) {
    throw "Current user favorites did not include favorited topic and article"
  }
  $reactionCacheRebuild = Invoke-ReactionCacheRebuild
  if (-not $reactionCacheRebuild.verified -or [int64]$reactionCacheRebuild.likes_loaded -lt 2 -or [int64]$reactionCacheRebuild.favorites_loaded -lt 2) {
    throw "Reaction cache rebuild did not verify loaded like and favorite caches"
  }
  $postRebuildTopicReactions = Invoke-Api -Uri "$baseUrl/api/v1/topics/$topicId/reactions" -Method Get -TimeoutSec 10
  $postRebuildArticleReactions = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/reactions" -Method Get -TimeoutSec 10
  if ([int64]$postRebuildTopicReactions.like_count -lt 1 -or [int64]$postRebuildTopicReactions.favorite_count -lt 1) {
    throw "Topic reaction counts changed after cache rebuild"
  }
  if ([int64]$postRebuildArticleReactions.like_count -lt 1 -or [int64]$postRebuildArticleReactions.favorite_count -lt 1) {
    throw "Article reaction counts changed after cache rebuild"
  }
  $comments = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/comments?page=1&page_size=10" -Method Get -TimeoutSec 10

  $feed = $null
  $feedItem = $null
  $feedCountersProjected = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $feed = Invoke-Api -Uri "$baseUrl/api/v1/feed?limit=10&offset=0" -Method Get -TimeoutSec 10
      foreach ($item in @($feed.items)) {
        if ([string]$item.id -eq [string]$articleId) {
          $feedItem = $item
        }
      }
      if ($feedItem -and [int64]$feedItem.comment_count -ge 1 -and [int64]$feedItem.like_count -ge 1 -and [int64]$feedItem.favorite_count -ge 1) {
        $feedCountersProjected = $true
        break
      }
    } catch {
    }
  }
  if (-not $feedItem) {
    throw "Created article was not projected into feed-service within timeout"
  }
  if (-not $feedCountersProjected) {
    throw "Feed item counters were not projected from comment/reaction events"
  }

  $followingFeed = Invoke-Api -Uri "$baseUrl/api/v1/feed?sort=follow&limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $followingFeedArticleListed = $false
  $followingFeedOwnTopicExcluded = $true
  foreach ($item in @($followingFeed.items)) {
    if ([string]$item.id -eq [string]$articleId -and $item.entity_type -eq "article") {
      $followingFeedArticleListed = $true
    }
    if ([string]$item.id -eq [string]$topicId -and $item.entity_type -eq "topic") {
      $followingFeedOwnTopicExcluded = $false
    }
  }
  if (-not $followingFeedArticleListed) {
    throw "Following feed did not include article from followed author"
  }
  if (-not $followingFeedOwnTopicExcluded) {
    throw "Following feed included topic from an unfollowed author"
  }

  $hotFeed = Invoke-Api -Uri "$baseUrl/api/v1/feed?sort=hot&limit=10&offset=0" -Method Get -TimeoutSec 10
  $foundHot = $false
  foreach ($item in @($hotFeed.items)) {
    if ([string]$item.id -eq [string]$articleId) {
      $foundHot = $true
    }
  }
  if (-not $foundHot) {
    throw "Created article was not available in hot feed"
  }

  $searchIndexed = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $searchUrl = "$baseUrl/api/v1/search/articles?q=$([uri]::EscapeDataString($title))"
      $search = Invoke-Api -Uri $searchUrl -Method Get -TimeoutSec 10
      foreach ($item in @($search.items)) {
        if ([string]$item.article.id -eq [string]$articleId) {
          $searchIndexed = $true
        }
      }
      if ($searchIndexed) {
        break
      }
    } catch {
    }
  }

  if (-not $searchIndexed) {
    throw "Created article was not indexed by search-service within timeout"
  }

  $notifications = $null
  $notificationTypes = @()
  $notificationsReady = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $notifications = Invoke-Api -Uri "$baseUrl/api/v1/notifications?limit=20&offset=0" -Method Get -Headers $followeeHeaders -TimeoutSec 10
      $notificationTypes = @($notifications.items | ForEach-Object { $_.type })
      if ($notificationTypes -contains "comment" -and $notificationTypes -contains "like" -and $notificationTypes -contains "favorite") {
        $notificationsReady = $true
        break
      }
    } catch {
    }
  }
  if (-not $notificationsReady) {
    throw "Author notifications were not projected from comment/reaction events"
  }
  $unread = Invoke-Api -Uri "$baseUrl/api/v1/notifications/unread-count" -Method Get -Headers $followeeHeaders -TimeoutSec 10
  if ([int64]$unread.count -lt 3) {
    throw "Unread notification count was lower than expected"
  }
  $firstNotification = @($notifications.items)[0]
  Invoke-Api -Uri "$baseUrl/api/v1/notifications/$($firstNotification.id)/read" -Method Post -Headers $followeeHeaders -TimeoutSec 10 | Out-Null
  $afterRead = Invoke-Api -Uri "$baseUrl/api/v1/notifications/unread-count" -Method Get -Headers $followeeHeaders -TimeoutSec 10
  if ([int64]$afterRead.count -ge [int64]$unread.count) {
    throw "Unread notification count did not decrease after mark read"
  }
  Invoke-Api -Uri "$baseUrl/api/v1/notifications/read-all" -Method Post -Headers $followeeHeaders -TimeoutSec 10 | Out-Null
  $afterReadAll = Invoke-Api -Uri "$baseUrl/api/v1/notifications/unread-count" -Method Get -Headers $followeeHeaders -TimeoutSec 10
  $afterReadAllCount = [int64]$afterReadAll.count
  if ($afterReadAllCount -ne 0) {
    throw "Unread notification count was not zero after mark all read"
  }

  $actorCredit = $null
  $authorCredit = $null
  $actorLedger = $null
  $authorLedger = $null
  $creditsReady = $false
  for ($i = 0; $i -lt $ProjectionRetries; $i++) {
    Start-Sleep -Seconds 1
    try {
      $actorCredit = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $headers -TimeoutSec 10
      $authorCredit = Invoke-Api -Uri "$baseUrl/api/v1/credits/balance" -Method Get -Headers $followeeHeaders -TimeoutSec 10
      if ([int64]$actorCredit.balance.total -ge 27 -and [int64]$authorCredit.balance.total -ge 34) {
        $creditsReady = $true
        break
      }
    } catch {
    }
  }
  if (-not $creditsReady) {
    throw "Credit balances were not projected from user/article/comment/reaction events"
  }
  $actorLedger = Invoke-Api -Uri "$baseUrl/api/v1/credits/ledger?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $authorLedger = Invoke-Api -Uri "$baseUrl/api/v1/credits/ledger?limit=20&offset=0" -Method Get -Headers $followeeHeaders -TimeoutSec 10
  if (@($actorLedger.items).Count -lt 5) {
    throw "Actor credit ledger did not include expected events"
  }
  if (@($authorLedger.items).Count -lt 5) {
    throw "Author credit ledger did not include expected events"
  }
  $unfavoriteArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/favorite" -Method Delete -Headers $headers -TimeoutSec 10
  if ([int64]$unfavoriteArticle.count -ne 0) {
    throw "Article unfavorite did not decrement count"
  }
  $favoritesAfterUnfavorite = Invoke-Api -Uri "$baseUrl/api/v1/users/current/favorites?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $favoriteArticleRemoved = $true
  $favoriteTopicStillListed = $false
  foreach ($item in @($favoritesAfterUnfavorite.items)) {
    if ($item.entity.entity_type -eq "article" -and [string]$item.entity.entity_id -eq [string]$articleId) {
      $favoriteArticleRemoved = $false
    }
    if ($item.entity.entity_type -eq "topic" -and [string]$item.entity.entity_id -eq [string]$topicId) {
      $favoriteTopicStillListed = $true
    }
  }
  if (-not $favoriteArticleRemoved -or -not $favoriteTopicStillListed) {
    throw "Current user favorites did not reflect article unfavorite"
  }
  $unlikeArticle = Invoke-Api -Uri "$baseUrl/api/v1/articles/$articleId/like" -Method Delete -Headers $headers -TimeoutSec 10
  if ([int64]$unlikeArticle.count -ne 0) {
    throw "Article unlike did not decrement count"
  }
  $likesAfterUnlike = Invoke-Api -Uri "$baseUrl/api/v1/users/current/likes?limit=20&offset=0" -Method Get -Headers $headers -TimeoutSec 10
  $likeArticleRemoved = $true
  $likeTopicStillListed = $false
  foreach ($item in @($likesAfterUnlike.items)) {
    if ($item.entity.entity_type -eq "article" -and [string]$item.entity.entity_id -eq [string]$articleId) {
      $likeArticleRemoved = $false
    }
    if ($item.entity.entity_type -eq "topic" -and [string]$item.entity.entity_id -eq [string]$topicId) {
      $likeTopicStillListed = $true
    }
  }
  if (-not $likeArticleRemoved -or -not $likeTopicStillListed) {
    throw "Current user likes did not reflect article unlike"
  }

  [pscustomobject]@{
    gateway = $baseUrl
    authProviders = $authProviderNames
    oauthGithubMinYears = $githubAuthProvider.min_account_years
    webmasterEnabled = $authConfig.webmaster_enabled
    username = $username
    adminUsername = $adminUsername
    rbacRoleKeys = $roleKeys
    createdAdminId = $createdAdmin.user.id
    createdAdminRoles = $assignedAdmin.user.roles
    userId = $me.user.id
    governanceUserListed = $governanceUserListed
    followeeId = $followeeId
    followRoundTrip = -not $unfollowState.following
    refollowedForFeed = $refollowState.following
    categoryId = $categoryId
    categoryName = $categoryDetail.category.name
    links = @($links.items).Count
    tasks = @($tasks.items).Count
    adminBadges = @($adminBadges.items).Count
    adminLevels = @($adminLevels.items).Count
    adminForbiddenWords = @($adminForbiddenWords.items).Count
    adminSettings = @($adminSettings.items).Count
    adminEmailLogs = @($adminEmailLogs.items).Count
    adminLinks = @($adminLinks.items).Count
    adminTasks = @($adminTasks.items).Count
    permissionTopicArchivedStatus = $deletedPermissionTopic.topic.status
    permissionArticleArchivedStatus = $deletedPermissionArticle.article.status
    topicId = $topicId
    topicListed = $topicListed
    categoryTopicListed = $categoryTopicListed
    topicCommentId = $topicCommentId
    permissionCommentHidden = $permissionCommentHidden
    topicLikeCount = $topicReactionCounts.like_count
    topicFavoriteCount = $topicReactionCounts.favorite_count
    topicFeedProjected = $topicFeedProjected
    topicFeedLikeCount = $topicFeedItem.like_count
    topicFeedFavoriteCount = $topicFeedItem.favorite_count
    topicFeedCommentCount = $topicFeedItem.comment_count
    topicSearchIndexed = $topicSearchIndexed
    adminTopicListed = $adminTopicListed
    hiddenTopicStatus = $hiddenTopic.topic.status
    archivedTopicStatus = $archivedTopic.topic.status
    articleId = $articleId
    articleTitle = $articleDetail.article.title
    tags = $tagNames
    reportId = $report.report.id
    reportStatus = $auditedReport.report.status
    moderationArticleId = $moderationArticleId
    moderationArticleHiddenStatus = $hiddenArticle.article.status
    moderationArticleArchivedStatus = $archivedArticle.article.status
    moderationCommentId = $moderationCommentId
    moderationCommentHidden = $moderationCommentHidden
    mutedUserStatus = $mutedUser.user.status
    unmutedUserStatus = $unmutedUser.user.status
    commentId = $comment.comment.id
    likeCount = $like.count
    favoriteCount = $favorite.count
    currentUserLikes = @($likes.items).Count
    likeTopicListed = $likeTopicListed
    likeArticleListed = $likeArticleListed
    currentUserFavorites = @($favorites.items).Count
    favoriteTopicListed = $favoriteTopicListed
    favoriteArticleListed = $favoriteArticleListed
    reactionCacheVerified = $reactionCacheRebuild.verified
    reactionCacheLikesLoaded = $reactionCacheRebuild.likes_loaded
    reactionCacheFavoritesLoaded = $reactionCacheRebuild.favorites_loaded
    listedComments = @($comments.items).Count
    feedItems = @($feed.items).Count
    feedLikeCount = $feedItem.like_count
    feedFavoriteCount = $feedItem.favorite_count
    feedCommentCount = $feedItem.comment_count
    followingFeedItems = @($followingFeed.items).Count
    followingFeedArticleListed = $followingFeedArticleListed
    followingFeedOwnTopicExcluded = $followingFeedOwnTopicExcluded
    searchIndexed = $searchIndexed
    notificationTypes = $notificationTypes
    unreadBeforeRead = $unread.count
    unreadAfterRead = $afterRead.count
    unreadAfterReadAll = $afterReadAllCount
    actorCreditTotal = $actorCredit.balance.total
    authorCreditTotal = $authorCredit.balance.total
    actorCreditLedger = @($actorLedger.items).Count
    authorCreditLedger = @($authorLedger.items).Count
    favoriteArticleRemoved = $favoriteArticleRemoved
    favoriteTopicStillListed = $favoriteTopicStillListed
    likeArticleRemoved = $likeArticleRemoved
    likeTopicStillListed = $likeTopicStillListed
  } | ConvertTo-Json -Depth 4
} finally {
  if (-not $KeepRunning) {
    Stop-StartedProcesses
  }
}
