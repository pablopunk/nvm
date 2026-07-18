[CmdletBinding()]
param([Parameter(Mandatory = $true)][string]$FixtureRoot)

$ErrorActionPreference = 'Stop'
$currentSid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$blockedIdentities = @('Everyone', 'BUILTIN\Users', 'NT AUTHORITY\Authenticated Users')
$allowedIdentities = @(
  $currentSid,
  'NT AUTHORITY\SYSTEM',
  'BUILTIN\Administrators'
)

foreach ($file in Get-ChildItem -LiteralPath $FixtureRoot -File) {
  $acl = Get-Acl -LiteralPath $file.FullName
  $ownerSid = $acl.Owner
  try {
    $ownerSid = ([System.Security.Principal.NTAccount]::new($acl.Owner)).Translate([System.Security.Principal.SecurityIdentifier]).Value
  } catch {}
  if ($ownerSid -ne $currentSid) { throw "$($file.Name) is not owned by the current Windows user." }
  foreach ($entry in $acl.Access) {
    if ($entry.AccessControlType -ne 'Allow') { continue }
    $identity = $entry.IdentityReference.Value
    $sid = $identity
    try {
      $sid = ([System.Security.Principal.NTAccount]::new($identity)).Translate([System.Security.Principal.SecurityIdentifier]).Value
    } catch {}
    if ($blockedIdentities -contains $identity) {
      throw "$($file.Name) grants access to blocked identity $identity."
    }
    if (($allowedIdentities -notcontains $identity) -and ($allowedIdentities -notcontains $sid)) {
      throw "$($file.Name) grants access to unexpected identity $identity."
    }
  }
  Write-Host "$($file.Name): owner=current-user; access identities redacted and restricted"
}
