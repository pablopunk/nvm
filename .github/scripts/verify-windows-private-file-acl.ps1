[CmdletBinding()]
param([Parameter(Mandatory = $true)][string]$FixtureRoot)

$ErrorActionPreference = 'Stop'
$currentSid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$blockedSids = @('S-1-1-0', 'S-1-5-11', 'S-1-5-32-545')
$allowedSids = @(
  $currentSid,
  'S-1-5-18',
  'S-1-5-32-544'
)
$allowedOwnerSids = @($currentSid, 'S-1-5-32-544')

function Resolve-Sid([string]$Identity) {
  if ($Identity -match '^S-1-') { return $Identity }
  return ([System.Security.Principal.NTAccount]::new($Identity)).Translate(
    [System.Security.Principal.SecurityIdentifier]
  ).Value
}

foreach ($file in Get-ChildItem -LiteralPath $FixtureRoot -File) {
  $acl = Get-Acl -LiteralPath $file.FullName
  try {
    $ownerSid = Resolve-Sid $acl.Owner
  } catch {
    throw "$($file.Name) owner could not be resolved to a Windows SID."
  }
  if ($allowedOwnerSids -notcontains $ownerSid) { throw "$($file.Name) has an unexpected owner." }
  $currentUserAllowed = $false
  foreach ($entry in $acl.Access) {
    if ($entry.AccessControlType -ne 'Allow') { continue }
    try {
      $sid = Resolve-Sid $entry.IdentityReference.Value
    } catch {
      throw "$($file.Name) has an access identity that could not be resolved to a Windows SID."
    }
    if ($blockedSids -contains $sid) {
      throw "$($file.Name) grants access to a blocked broad Windows identity."
    }
    if ($allowedSids -notcontains $sid) { throw "$($file.Name) grants access to an unexpected identity." }
    if ($sid -eq $currentSid) { $currentUserAllowed = $true }
  }
  if (-not $currentUserAllowed) { throw "$($file.Name) does not grant the current Windows user access." }
  Write-Host "$($file.Name): owner=private-principal; access identities redacted and restricted"
}
