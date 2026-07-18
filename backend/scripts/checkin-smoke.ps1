param(
  [int]$GatewayPort = 18080
)

$ErrorActionPreference = "Stop"

function Invoke-BbsApi {
  param(
    [string]$Uri,
    [string]$Method = "GET",
    [hashtable]$Headers,
    [string]$Body
  )

  $params = @{ Uri = $Uri; Method = $Method; TimeoutSec = 15 }
  if ($null -ne $Headers) {
    $params.Headers = $Headers
  }
  if (-not [string]::IsNullOrWhiteSpace($Body)) {
    $params.ContentType = "application/json"
    $params.Body = $Body
  }
  $response = Invoke-RestMethod @params
  if ([int]$response.code -ne 0) {
    throw "API error $($response.code): $($response.message)"
  }
  return $response.data
}

$baseUrl = "http://127.0.0.1:$GatewayPort/api/v1"
$stamp = Get-Date -Format "yyyyMMddHHmmssfff"
$username = "checkin$stamp"
$registrationBody = @{
  username = $username
  email = "$username@example.com"
  password = "Password123!"
  nickname = "Check-in Smoke"
} | ConvertTo-Json -Compress

$registration = Invoke-BbsApi -Uri "$baseUrl/auth/register" -Method "POST" -Body $registrationBody
$token = [string]$registration.access_token
if ([string]::IsNullOrWhiteSpace($token)) {
  throw "Registration did not return an access token"
}

$headers = @{ Authorization = "Bearer $token" }
$userID = [string]$registration.user.id
$before = Invoke-BbsApi -Uri "$baseUrl/credits/check-in" -Headers $headers
if ([bool]$before.checked_in) {
  throw "Fresh account was already checked in"
}

$first = Invoke-BbsApi -Uri "$baseUrl/credits/check-in" -Method "POST" -Headers $headers
$second = Invoke-BbsApi -Uri "$baseUrl/credits/check-in" -Method "POST" -Headers $headers
$after = Invoke-BbsApi -Uri "$baseUrl/credits/check-in" -Headers $headers
$ledger = Invoke-BbsApi -Uri "$baseUrl/credits/ledger?limit=50&offset=0" -Headers $headers
$dailyEntries = @($ledger.items | Where-Object { $_.reason -eq "daily_check_in" })
$day = [string]$first.check_in.latest_day
$expectedEventID = "credit.checkin:${userID}:$day"

if ([bool]$first.duplicate) {
  throw "First check-in was marked duplicate"
}
if (-not [bool]$second.duplicate) {
  throw "Second same-day check-in was not marked duplicate"
}
if (-not [bool]$after.checked_in) {
  throw "Check-in status remained incomplete after successful check-in"
}
if ([int64]$first.ledger.delta -ne 5 -or [string]$first.ledger.reason -ne "daily_check_in") {
  throw "First check-in ledger did not contain the expected reward"
}
if ([string]$first.ledger.source_event_id -ne $expectedEventID) {
  throw "First check-in source event id was not deterministic"
}
if ($dailyEntries.Count -ne 1) {
  throw "Daily check-in ledger count = $($dailyEntries.Count), want 1"
}
if ([string]$second.ledger.id -ne [string]$first.ledger.id) {
  throw "Duplicate check-in did not return the original ledger entry"
}

[pscustomobject]@{
  user_id            = $userID
  day                = $day
  consecutive_days   = $first.check_in.consecutive_days
  reward_credits     = $first.reward_credits
  first_duplicate    = [bool]$first.duplicate
  second_duplicate   = [bool]$second.duplicate
  checked_in_after   = [bool]$after.checked_in
  daily_ledger_count = $dailyEntries.Count
  source_event_id    = $first.ledger.source_event_id
} | ConvertTo-Json -Compress
