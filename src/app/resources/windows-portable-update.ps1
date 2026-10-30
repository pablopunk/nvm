# Replaces a running Nevermind portable executable after the app exits.
# Started detached by the app; every outcome is written to -ResultPath.
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$TargetPath,
  [Parameter(Mandatory = $true)][string]$SourcePath,
  [Parameter(Mandatory = $true)][string]$ExpectedSha512,
  [Parameter(Mandatory = $true)][string]$ExpectedVersion,
  [Parameter(Mandatory = $true)][string]$CurrentVersion,
  [Parameter(Mandatory = $true)][string]$WorkDirectory,
  [Parameter(Mandatory = $true)][string]$ResultPath,
  [string]$WaitProcessIds = '',
  [int]$AppExitTimeoutSeconds = 300,
  [int]$TargetLockTimeoutSeconds = 120,
  [int]$HealthTimeoutSeconds = 240
)

$ErrorActionPreference = 'Stop'
# An inherited PowerShell 7 PSModulePath makes Windows PowerShell autoload an incompatible Security module.
Import-Module (Join-Path $PSHOME 'Modules\Microsoft.PowerShell.Security\Microsoft.PowerShell.Security.psd1') -ErrorAction SilentlyContinue
$LogPath = Join-Path $WorkDirectory 'helper.log'
$TargetDirectory = [IO.Path]::GetDirectoryName($TargetPath)
$TargetName = [IO.Path]::GetFileName($TargetPath)
$StagedPath = Join-Path $TargetDirectory ".$TargetName.$ExpectedVersion.update"
$BackupPath = Join-Path $TargetDirectory ".$TargetName.previous"
$LockPath = Join-Path $TargetDirectory ".$TargetName.update-lock"
$HealthPath = Join-Path $WorkDirectory "nevermind-update-health-$([guid]::NewGuid().ToString('N')).json"
$ProcessIdsToAwait = @($WaitProcessIds -split ',' | Where-Object { $_ -match '^\d+$' } | ForEach-Object { [int]$_ })

function Write-Log([string]$Message) {
  $line = "$([DateTime]::UtcNow.ToString('o')) $Message"
  Add-Content -LiteralPath $LogPath -Value $line -Encoding utf8
}

function Write-Result([string]$Outcome, [string]$Reason) {
  $result = [ordered]@{
    schemaVersion = 1
    outcome = $Outcome
    reason = $Reason
    fromVersion = $CurrentVersion
    toVersion = $ExpectedVersion
    targetPath = $TargetPath
    finishedAt = [DateTime]::UtcNow.ToString('o')
  }
  $temporary = "$ResultPath.$PID.tmp"
  [IO.File]::WriteAllText($temporary, ($result | ConvertTo-Json))
  Move-Item -LiteralPath $temporary -Destination $ResultPath -Force
  Write-Log "result $Outcome $Reason"
}

function Get-Sha512Base64([string]$Path) {
  $stream = [IO.File]::Open($Path, 'Open', 'Read', 'Read')
  $sha = [Security.Cryptography.SHA512]::Create()
  try {
    return [Convert]::ToBase64String($sha.ComputeHash($stream))
  } finally {
    $sha.Dispose()
    $stream.Dispose()
  }
}

function Test-ProcessAlive([int]$ProcessId) {
  return $null -ne (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)
}

function Wait-ForAppExit {
  $deadline = [DateTime]::UtcNow.AddSeconds($AppExitTimeoutSeconds)
  while (@($ProcessIdsToAwait | Where-Object { Test-ProcessAlive $_ }).Count -gt 0) {
    if ([DateTime]::UtcNow -gt $deadline) { return $false }
    Start-Sleep -Milliseconds 250
  }
  return $true
}

function Test-TargetExclusivelyWritable {
  try {
    $stream = [IO.File]::Open($TargetPath, 'Open', 'ReadWrite', 'None')
    $stream.Dispose()
    return $true
  } catch {
    return $false
  }
}

function Wait-ForTargetRelease {
  $deadline = [DateTime]::UtcNow.AddSeconds($TargetLockTimeoutSeconds)
  while (-not (Test-TargetExclusivelyWritable)) {
    if ([DateTime]::UtcNow -gt $deadline) { return $false }
    Start-Sleep -Milliseconds 500
  }
  return $true
}

function Enter-UpdateLock {
  for ($attempt = 0; $attempt -lt 2; $attempt++) {
    try {
      $stream = [IO.File]::Open($LockPath, 'CreateNew', 'Write', 'None')
      $bytes = [Text.Encoding]::UTF8.GetBytes([string]$PID)
      $stream.Write($bytes, 0, $bytes.Length)
      $stream.Dispose()
      return $true
    } catch [IO.IOException] {
      $owner = 0
      [void][int]::TryParse((Get-Content -LiteralPath $LockPath -Raw -ErrorAction SilentlyContinue), [ref]$owner)
      if ($owner -gt 0 -and (Test-ProcessAlive $owner)) { return $false }
      Write-Log "removing stale update lock owned by $owner"
      Remove-Item -LiteralPath $LockPath -Force -ErrorAction SilentlyContinue
    }
  }
  return $false
}

function Get-SignerSubject([string]$Path) {
  $signature = Get-AuthenticodeSignature -LiteralPath $Path
  if ($signature.Status -ne 'Valid') { return $null }
  return $signature.SignerCertificate.Subject
}

function Assert-TrustedReplacement {
  $currentSigner = Get-SignerSubject $TargetPath
  if ($null -eq $currentSigner) { return }
  $stagedSigner = Get-SignerSubject $StagedPath
  if ($stagedSigner -ne $currentSigner) {
    throw "untrusted-signer: the update is not signed by $currentSigner"
  }
}

function Copy-ExplicitAccessRules([string]$From, [string]$To) {
  try {
    $acl = (Get-Item -LiteralPath $From).GetAccessControl('Access')
    $explicitRules = @($acl.GetAccessRules($true, $false, [Security.Principal.SecurityIdentifier]))
    if (-not $acl.AreAccessRulesProtected -and $explicitRules.Count -eq 0) { return }
    (Get-Item -LiteralPath $To).SetAccessControl($acl)
  } catch {
    Write-Log "could not copy access rules: $($_.Exception.Message)"
  }
}

function Stage-Update {
  Copy-Item -LiteralPath $SourcePath -Destination $StagedPath -Force
  $actual = Get-Sha512Base64 $StagedPath
  if ($actual -ne $ExpectedSha512) { throw 'integrity: the downloaded update does not match its release checksum' }
  Assert-TrustedReplacement
}

function Swap-InStagedUpdate {
  if (Test-Path -LiteralPath $BackupPath) { Remove-Item -LiteralPath $BackupPath -Force }
  [IO.File]::Move($TargetPath, $BackupPath)
  try {
    [IO.File]::Move($StagedPath, $TargetPath)
  } catch {
    [IO.File]::Move($BackupPath, $TargetPath)
    throw
  }
  Copy-ExplicitAccessRules $BackupPath $TargetPath
  if ((Get-Sha512Base64 $TargetPath) -ne $ExpectedSha512) {
    Restore-Backup
    throw 'integrity: the replaced executable does not match its release checksum'
  }
}

function Restore-Backup {
  if (-not (Test-Path -LiteralPath $BackupPath)) { return }
  if (Test-Path -LiteralPath $TargetPath) { Remove-Item -LiteralPath $TargetPath -Force }
  [IO.File]::Move($BackupPath, $TargetPath)
  Write-Log 'restored previous executable'
}

function Start-Target([string]$HealthFile) {
  $info = New-Object Diagnostics.ProcessStartInfo
  $info.FileName = $TargetPath
  $info.WorkingDirectory = $TargetDirectory
  $info.UseShellExecute = $false
  foreach ($name in @('PORTABLE_EXECUTABLE_FILE', 'PORTABLE_EXECUTABLE_DIR', 'PORTABLE_EXECUTABLE_APP_FILENAME', 'NVM_PORTABLE_UPDATE_HEALTH_FILE')) {
    [void]$info.EnvironmentVariables.Remove($name)
  }
  if ($HealthFile) { $info.EnvironmentVariables['NVM_PORTABLE_UPDATE_HEALTH_FILE'] = $HealthFile }
  return [Diagnostics.Process]::Start($info)
}

function Read-HealthVersion {
  try {
    return (Get-Content -LiteralPath $HealthPath -Raw | ConvertFrom-Json).version
  } catch {
    return $null
  }
}

function Wait-ForHealthyLaunch([Diagnostics.Process]$Process) {
  $deadline = [DateTime]::UtcNow.AddSeconds($HealthTimeoutSeconds)
  while ([DateTime]::UtcNow -lt $deadline) {
    $version = Read-HealthVersion
    if ($version -eq $ExpectedVersion) { return 'healthy' }
    if ($version) { return "wrong-version: the relaunched app reported $version" }
    if ($Process.HasExited) { return "startup-failed: the updated app exited with code $($Process.ExitCode) before it was ready" }
    Start-Sleep -Milliseconds 250
  }
  return 'unconfirmed'
}

function Stop-ProcessTree([Diagnostics.Process]$Process) {
  if ($null -eq $Process -or $Process.HasExited) { return }
  & taskkill.exe /PID $Process.Id /T /F | Out-Null
  [void]$Process.WaitForExit(10000)
}

function Relaunch-Previous([string]$Outcome, [string]$Reason) {
  Write-Result $Outcome $Reason
  try {
    [void](Start-Target $null)
  } catch {
    Write-Log "could not relaunch previous version: $($_.Exception.Message)"
  }
}

function Install-Update {
  try {
    Stage-Update
    Swap-InStagedUpdate
  } catch {
    Remove-Item -LiteralPath $StagedPath -Force -ErrorAction SilentlyContinue
    Relaunch-Previous 'failed' $_.Exception.Message
    return 1
  }
  Write-Log 'swapped executable; launching updated app'
  $launch = $null
  try {
    $launch = Start-Target $HealthPath
    $health = Wait-ForHealthyLaunch $launch
  } catch {
    $health = "startup-failed: $($_.Exception.Message)"
  }
  if ($health -eq 'healthy') {
    Remove-Item -LiteralPath $BackupPath -Force -ErrorAction SilentlyContinue
    Write-Result 'installed' 'healthy'
    return 0
  }
  if ($health -eq 'unconfirmed') {
    Write-Result 'unconfirmed' 'the updated app is running but did not confirm readiness; the previous executable was kept beside it'
    return 0
  }
  Stop-ProcessTree $launch
  [void](Wait-ForTargetRelease)
  Restore-Backup
  Relaunch-Previous 'rolled-back' $health
  return 1
}

New-Item -ItemType Directory -Path $WorkDirectory -Force | Out-Null
Write-Log "updating $TargetPath from $CurrentVersion to $ExpectedVersion"
if (-not (Wait-ForAppExit)) {
  Write-Result 'failed' 'app-still-running: Nevermind did not exit, so the update was not installed'
  exit 1
}
$ownsLock = $false
$exitCode = 1
try {
  $ownsLock = Enter-UpdateLock
  if (-not $ownsLock) {
    Relaunch-Previous 'failed' 'busy: another Nevermind update is already being installed'
  } elseif (-not (Wait-ForTargetRelease)) {
    Relaunch-Previous 'failed' 'target-in-use: another process is still using the Nevermind executable'
  } else {
    $exitCode = Install-Update
  }
} catch {
  Write-Log "unexpected failure: $($_.Exception.Message)"
  Relaunch-Previous 'failed' "unexpected: $($_.Exception.Message)"
} finally {
  if ($ownsLock) { Remove-Item -LiteralPath $LockPath -Force -ErrorAction SilentlyContinue }
  Remove-Item -LiteralPath $HealthPath -Force -ErrorAction SilentlyContinue
}
exit $exitCode
