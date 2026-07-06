param(
  [string[]]$Services = @("admin-service", "api-gateway"),
  [switch]$All,
  [switch]$Restart,
  [switch]$Build,
  [switch]$NoWait,
  [int]$WaitSeconds = 30
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$ServicesRoot = Join-Path $RepoRoot "backend\services"

$ServiceSpecs = [ordered]@{
  "user-service" = @{
    Port = 9102
    Args = @("-config", "configs/config.yaml")
    BuildTarget = ".\cmd\server"
  }
  "content-service" = @{
    Port = 9103
    Args = @("server", "-c", "configs/config.yaml")
    BuildTarget = ".\cmd"
  }
  "comment-service" = @{
    Port = 9104
    Args = @("-config", "configs/config.yaml")
    BuildTarget = ".\cmd\server"
  }
  "reaction-service" = @{
    Port = 9105
    Args = @("server", "-c", "configs/config.yaml")
    BuildTarget = ".\cmd"
  }
  "search-service" = @{
    Port = 9106
    Args = @("-config", "configs/config.yaml")
    BuildTarget = ".\cmd\server"
  }
  "credit-service" = @{
    Port = 9107
    Args = @("-config", "configs/config.yaml")
    BuildTarget = ".\cmd\server"
  }
  "notification-service" = @{
    Port = 9108
    Args = @("-config", "configs/config.yaml")
    BuildTarget = ".\cmd\server"
  }
  "feed-service" = @{
    Port = 9113
    Args = @("-config", "configs/config.yaml")
    BuildTarget = ".\cmd\server"
  }
  "admin-service" = @{
    Port = 9114
    Args = @("server", "-c", "configs/config.yaml")
    BuildTarget = ".\cmd"
  }
  "api-gateway" = @{
    Port = 18080
    Args = @("-config", "configs/config.local.yaml")
    BuildTarget = ".\cmd\server"
  }
}

function Test-PortListening {
  param([int]$Port)

  $client = [System.Net.Sockets.TcpClient]::new()
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

function Wait-Port {
  param(
    [string]$ServiceName,
    [int]$Port,
    [int]$TimeoutSeconds
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    if (Test-PortListening $Port) {
      return $true
    }
    Start-Sleep -Milliseconds 300
  }

  Write-Warning "$ServiceName did not listen on port $Port within $TimeoutSeconds seconds."
  return $false
}

function Get-ServiceProcess {
  param([string]$ServiceName)

  $serviceDir = Join-Path $ServicesRoot $ServiceName
  $expectedExe = Join-Path $serviceDir "bin\$ServiceName.exe"
  Get-CimInstance Win32_Process -Filter "name='$ServiceName.exe'" |
    Where-Object { $_.ExecutablePath -eq $expectedExe -or $_.CommandLine -like "*$ServiceName.exe*" }
}

function Stop-ServiceProcess {
  param([string]$ServiceName)

  $processes = @(Get-ServiceProcess $ServiceName)
  $parentProcessIds = @()
  foreach ($process in $processes) {
    $parentProcessIds += [int]$process.ParentProcessId
    Stop-Process -Id $process.ProcessId -Force -ErrorAction SilentlyContinue
    Write-Host "Stopped $ServiceName process $($process.ProcessId)."
  }

  Start-Sleep -Milliseconds 500

  foreach ($parentProcessId in ($parentProcessIds | Sort-Object -Unique)) {
    $parent = Get-CimInstance Win32_Process -Filter "ProcessId=$parentProcessId" -ErrorAction SilentlyContinue
    if ($null -eq $parent) {
      continue
    }
    if (($parent.Name -in @("cmd.exe", "powershell.exe", "pwsh.exe")) -and ($parent.CommandLine -like "*$ServiceName*")) {
      Stop-Process -Id $parentProcessId -Force -ErrorAction SilentlyContinue
      Write-Host "Closed $ServiceName launcher window $parentProcessId."
    }
  }
}

function Invoke-ServiceBuild {
  param(
    [string]$ServiceName,
    [hashtable]$Spec
  )

  $serviceDir = Join-Path $ServicesRoot $ServiceName
  Push-Location $serviceDir
  try {
    Write-Host "Building $ServiceName..."
    go build -o "bin\$ServiceName.exe" $Spec.BuildTarget
  } finally {
    Pop-Location
  }
}

function Start-ServiceWindow {
  param(
    [string]$ServiceName,
    [hashtable]$Spec
  )

  $serviceDir = Join-Path $ServicesRoot $ServiceName
  $exePath = Join-Path $serviceDir "bin\$ServiceName.exe"
  if (-not (Test-Path $exePath)) {
    throw "$ServiceName binary not found: $exePath. Re-run with -Build."
  }

  $argsText = ($Spec.Args | ForEach-Object {
    if ($_ -match "\s") { '"' + $_ + '"' } else { $_ }
  }) -join " "
  $command = "title BBS $ServiceName && `"$exePath`" $argsText"
  Start-Process cmd.exe -WorkingDirectory $serviceDir -WindowStyle Normal -ArgumentList @("/k", $command)
  Write-Host "Started $ServiceName in a visible window on port $($Spec.Port)."
}

if ($All) {
  $Services = @($ServiceSpecs.Keys)
} else {
  $Services = @($Services | ForEach-Object { $_ -split "," } | ForEach-Object { $_.Trim() } | Where-Object { $_ })
}

foreach ($serviceName in $Services) {
  if (-not $ServiceSpecs.Contains($serviceName)) {
    throw "Unknown service '$serviceName'. Available: $($ServiceSpecs.Keys -join ', ')"
  }
}

foreach ($serviceName in $Services) {
  $spec = $ServiceSpecs[$serviceName]
  if ($Restart) {
    Stop-ServiceProcess $serviceName
  } elseif (Test-PortListening $spec.Port) {
    Write-Warning "$serviceName port $($spec.Port) is already listening. Use -Restart to replace the current process."
    continue
  }

  if ($Build) {
    Invoke-ServiceBuild $serviceName $spec
  }

  Start-ServiceWindow $serviceName $spec
}

if (-not $NoWait) {
  foreach ($serviceName in $Services) {
    $spec = $ServiceSpecs[$serviceName]
    [void](Wait-Port $serviceName $spec.Port $WaitSeconds)
  }
}

Write-Host "Local backend windows requested: $($Services -join ', ')"
