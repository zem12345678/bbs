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

function Assert-BbsApiStatus {
  param(
    [int]$ExpectedStatus,
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

  try {
    [void](Invoke-RestMethod @params)
    throw "Expected HTTP $ExpectedStatus from $Uri"
  } catch {
    $response = $_.Exception.Response
    if ($null -eq $response) {
      throw
    }
    $status = [int]$response.StatusCode
    if ($status -ne $ExpectedStatus) {
      throw "Expected HTTP $ExpectedStatus from $Uri, got $status"
    }
    $raw = ""
    try {
      $reader = New-Object System.IO.StreamReader($response.GetResponseStream())
      $raw = $reader.ReadToEnd()
      $reader.Dispose()
    } catch {}
    return [pscustomobject]@{ Status = $status; Body = $raw }
  }
}

$baseUrl = "http://127.0.0.1:$GatewayPort/api/v1"
$stamp = Get-Date -Format "yyyyMMddHHmmssfff"
$admin = Invoke-BbsApi -Uri "$baseUrl/admin/auth/login" -Method "POST" -Body (@{
  account = "admin"
  password = "Admin123!"
} | ConvertTo-Json -Compress)
$adminToken = [string]$admin.access_token
if ([string]::IsNullOrWhiteSpace($adminToken)) {
  throw "Admin login did not return an access token"
}
$adminHeaders = @{ Authorization = "Bearer $adminToken" }

$category = Invoke-BbsApi -Uri "$baseUrl/admin/mall/categories" -Method "POST" -Headers $adminHeaders -Body (@{
  slug = "cart-replay-$stamp"
  name = "Cart Replay $stamp"
  description = "Cart replay smoke category"
  status = 2
  sort = 999
} | ConvertTo-Json -Compress)
$categorySlug = [string]$category.category.slug
if ([string]::IsNullOrWhiteSpace($categorySlug)) {
  throw "Category creation did not return a slug"
}

$product = Invoke-BbsApi -Uri "$baseUrl/admin/mall/products" -Method "POST" -Headers $adminHeaders -Body (@{
  sku = "CART-REPLAY-$stamp"
  title = "Cart Replay $stamp"
  description = "Cart replay smoke product"
  category = $categorySlug
  price_credits = 10
  stock = 5
  status = 2
  sort = 999
} | ConvertTo-Json -Compress)
$productID = [string]$product.product.id
if ([string]::IsNullOrWhiteSpace($productID)) {
  throw "Product creation did not return an id"
}
$expectedOriginalCredits = [int64]$product.product.price_credits

$username = "cartreplay$stamp"
$registration = Invoke-BbsApi -Uri "$baseUrl/auth/register" -Method "POST" -Body (@{
  username = $username
  email = "$username@example.com"
  password = "Password123!"
  nickname = "Cart Replay Smoke"
} | ConvertTo-Json -Compress)
$token = [string]$registration.access_token
if ([string]::IsNullOrWhiteSpace($token)) {
  throw "Registration did not return an access token"
}
$headers = @{ Authorization = "Bearer $token" }

$requestKey = "cart-replay-$stamp"
$originalAddress = "Shanghai Zhangjiang Road 1"
$changedAddress = "Shanghai Zhangjiang Road 2"
$checkoutBody = @{
  idempotency_key = $requestKey
  expected_original_credits = $expectedOriginalCredits
  receiver = "Cart Replay"
  phone = "13800000000"
  address = $originalAddress
} | ConvertTo-Json -Compress
$orderID = ""

try {
  [void](Invoke-BbsApi -Uri "$baseUrl/mall/cart/items/$productID" -Method "PUT" -Headers $headers -Body (@{ quantity = 1 } | ConvertTo-Json -Compress))
  $created = Invoke-BbsApi -Uri "$baseUrl/mall/cart/checkout" -Method "POST" -Headers $headers -Body $checkoutBody
  $orderID = [string]$created.order.id
  if ([string]::IsNullOrWhiteSpace($orderID)) {
    throw "Cart checkout did not return an order id"
  }

  $replay = Invoke-BbsApi -Uri "$baseUrl/mall/cart/checkout" -Method "POST" -Headers $headers -Body $checkoutBody
  if (-not [bool]$replay.duplicate -or [string]$replay.order.id -ne $orderID) {
    throw "Matching cart replay did not return the original order"
  }

  $conflictBody = @{
    idempotency_key = $requestKey
    expected_original_credits = $expectedOriginalCredits
    receiver = "Cart Replay"
    phone = "13800000000"
    address = $changedAddress
  } | ConvertTo-Json -Compress
  $conflict = Assert-BbsApiStatus -ExpectedStatus 409 -Uri "$baseUrl/mall/cart/checkout" -Method "POST" -Headers $headers -Body $conflictBody
  $order = Invoke-BbsApi -Uri "$baseUrl/mall/orders/$orderID" -Headers $headers
  if ([string]$order.order.address -ne $originalAddress) {
    throw "Conflicting cart replay changed the original order address"
  }

  [pscustomobject]@{
    order_id = $orderID
    product_id = $productID
    duplicate_replay = [bool]$replay.duplicate
    conflict_status = $conflict.Status
    original_address = $order.order.address
  } | ConvertTo-Json -Compress
} finally {
  if (-not [string]::IsNullOrWhiteSpace($orderID)) {
    try {
      [void](Invoke-BbsApi -Uri "$baseUrl/mall/orders/$orderID/cancel" -Method "POST" -Headers $headers)
    } catch {}
  }
}
