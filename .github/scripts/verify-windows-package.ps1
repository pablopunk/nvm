[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$PackageRoot,
  [Parameter(Mandatory = $true)][string]$ExpectedVersion,
  [Parameter(Mandatory = $true)][ValidateSet('x64')][string]$ExpectedArch,
  [Parameter(Mandatory = $true)][string]$AsarExecutable,
  [Parameter(Mandatory = $true)][ValidateSet('Absent', 'Nsis')][string]$UpdaterMetadataPolicy
)

$ErrorActionPreference = 'Stop'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$ResolvedPackageRoot = (Resolve-Path $PackageRoot).Path
$ResolvedAsarExecutable = (Resolve-Path $AsarExecutable).Path
$ExtractionRoot = Join-Path $env:RUNNER_TEMP "windows-package-verify-$([guid]::NewGuid())"

function Assert-Condition([bool]$Condition, [string]$Message) {
  if (-not $Condition) { throw $Message }
}

function Resolve-UniqueArtifact([string]$Pattern, [string]$ExpectedName) {
  $matches = @(Get-ChildItem -LiteralPath $ResolvedPackageRoot -File | Where-Object { $_.Name -like $Pattern })
  Assert-Condition ($matches.Count -eq 1) "Expected exactly one $Pattern artifact; found $($matches.Count)."
  Assert-Condition ($matches[0].Name -ceq $ExpectedName) "Expected $ExpectedName; found $($matches[0].Name)."
  Assert-Condition ($matches[0].Length -gt 0) "$ExpectedName is empty."
  return $matches[0]
}

function Get-Sha512Base64([string]$Path) {
  $sha = [System.Security.Cryptography.SHA512]::Create()
  try {
    return [Convert]::ToBase64String($sha.ComputeHash([IO.File]::ReadAllBytes($Path)))
  } finally {
    $sha.Dispose()
  }
}

function Get-Sha256Hex([string]$Path) {
  return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Assert-Unsigned([IO.FileInfo]$Artifact) {
  $signature = Get-AuthenticodeSignature -LiteralPath $Artifact.FullName
  Assert-Condition ($signature.Status -eq 'NotSigned') "$($Artifact.Name) must be unsigned; status was $($signature.Status)."
  Assert-Condition ($null -eq $signature.SignerCertificate) "$($Artifact.Name) unexpectedly has a signer certificate."
  return $signature.Status.ToString()
}

function Read-PngDimension([string]$Path, [int]$Offset) {
  $bytes = [IO.File]::ReadAllBytes($Path)
  Assert-Condition ($bytes.Length -ge 24) "Configured icon is not a complete PNG."
  return [uint32](
    ([uint64]$bytes[$Offset] * 16777216) +
    ([uint64]$bytes[$Offset + 1] * 65536) +
    ([uint64]$bytes[$Offset + 2] * 256) +
    [uint64]$bytes[$Offset + 3]
  )
}

if (-not ('NativeResourceCounter' -as [type])) {
  Add-Type @'
using System;
using System.Runtime.InteropServices;

public static class NativeResourceCounter {
  private delegate bool EnumResNameProc(IntPtr module, IntPtr type, IntPtr name, IntPtr parameter);
  [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
  private static extern IntPtr LoadLibraryEx(string fileName, IntPtr file, uint flags);
  [DllImport("kernel32.dll", SetLastError = true)]
  private static extern bool FreeLibrary(IntPtr module);
  [DllImport("kernel32.dll", SetLastError = true)]
  private static extern bool EnumResourceNames(IntPtr module, IntPtr type, EnumResNameProc callback, IntPtr parameter);

  public static int Count(string fileName, int resourceType) {
    const uint LOAD_LIBRARY_AS_DATAFILE = 0x00000002;
    IntPtr module = LoadLibraryEx(fileName, IntPtr.Zero, LOAD_LIBRARY_AS_DATAFILE);
    if (module == IntPtr.Zero) throw new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error());
    int count = 0;
    EnumResNameProc callback = delegate { count++; return true; };
    try {
      bool success = EnumResourceNames(module, new IntPtr(resourceType), callback, IntPtr.Zero);
      int error = Marshal.GetLastWin32Error();
      if (!success && error != 0 && error != 1813 && error != 1814) throw new System.ComponentModel.Win32Exception(error);
      return count;
    } finally {
      FreeLibrary(module);
      GC.KeepAlive(callback);
    }
  }
}
'@
}

try {
  $setupName = "Nevermind-$ExpectedVersion-win-$ExpectedArch-setup.exe"
  $portableName = "Nevermind-$ExpectedVersion-win-$ExpectedArch-portable.exe"
  $setup = Resolve-UniqueArtifact 'Nevermind-*-win-*-setup.exe' $setupName
  $portable = Resolve-UniqueArtifact 'Nevermind-*-win-*-portable.exe' $portableName
  $blockmap = Resolve-UniqueArtifact 'Nevermind-*-win-*-setup.exe.blockmap' "$setupName.blockmap"
  $unpacked = Get-Item -LiteralPath (Join-Path $ResolvedPackageRoot 'win-unpacked/Nevermind.exe')
  $asar = Get-Item -LiteralPath (Join-Path $ResolvedPackageRoot 'win-unpacked/resources/app.asar')
  Assert-Condition ($unpacked.Length -gt 0) 'win-unpacked/Nevermind.exe is empty.'
  Assert-Condition ($asar.Length -gt 0) 'win-unpacked/resources/app.asar is empty.'

  $setupSignature = Assert-Unsigned $setup
  $portableSignature = Assert-Unsigned $portable
  $unpackedSignature = Assert-Unsigned $unpacked

  New-Item -ItemType Directory -Path $ExtractionRoot | Out-Null
  & $ResolvedAsarExecutable extract $asar.FullName $ExtractionRoot
  Assert-Condition ($LASTEXITCODE -eq 0) 'Pinned @electron/asar extraction failed.'
  foreach ($requiredPath in @(
    'dist/main/main.js',
    'dist/preload/preload.cjs',
    'dist/renderer/index.html',
    'src/resources/nevermind-extension-api.d.ts',
    'node_modules/typescript/lib/lib.es2022.full.d.ts'
  )) {
    Assert-Condition (Test-Path -LiteralPath (Join-Path $ExtractionRoot $requiredPath)) "Packaged ASAR is missing $requiredPath."
  }
  & node (Join-Path $RepoRoot 'scripts/check-packaged-resources.cjs') $ExtractionRoot
  Assert-Condition ($LASTEXITCODE -eq 0) 'Packaged resource check failed.'
  & node (Join-Path $RepoRoot 'scripts/check-packaged-runtime-imports.cjs') $ExtractionRoot
  Assert-Condition ($LASTEXITCODE -eq 0) 'Packaged runtime import check failed.'

  $latest = Join-Path $ResolvedPackageRoot 'latest.yml'
  $updaterMetadata = [ordered]@{ policy = $UpdaterMetadataPolicy; file = $null; sha256 = $null }
  if ($UpdaterMetadataPolicy -eq 'Absent') {
    Assert-Condition (-not (Test-Path -LiteralPath $latest)) 'Unsigned smoke must not contain stale latest.yml updater metadata.'
  } else {
    Assert-Condition (Test-Path -LiteralPath $latest) 'NSIS updater policy requires latest.yml.'
    & node (Join-Path $RepoRoot 'scripts/validate-windows-updater-metadata.cjs') $latest $ResolvedPackageRoot $ExpectedVersion $ExpectedArch
    Assert-Condition ($LASTEXITCODE -eq 0) 'Windows updater metadata check failed.'
    $updaterMetadata.file = 'latest.yml'
    $updaterMetadata.sha256 = Get-Sha256Hex $latest
  }

  $iconSource = Join-Path $RepoRoot 'build/Icon.icon/Assets/icon.png'
  $builderConfig = Get-Content -LiteralPath (Join-Path $RepoRoot 'electron-builder.yml') -Raw
  Assert-Condition ($builderConfig -match 'win:\s*[\s\S]*?icon:\s*build/Icon\.icon/Assets/icon\.png') 'win.icon must identify the configured source.'
  $iconWidth = Read-PngDimension $iconSource 16
  $iconHeight = Read-PngDimension $iconSource 20
  Assert-Condition ($iconWidth -eq 1254 -and $iconHeight -eq 1254) 'Configured Windows icon must be 1254x1254.'
  $unpackedGroupIcons = [NativeResourceCounter]::Count($unpacked.FullName, 14)
  $unpackedIcons = [NativeResourceCounter]::Count($unpacked.FullName, 3)
  $portableGroupIcons = [NativeResourceCounter]::Count($portable.FullName, 14)
  $portableIcons = [NativeResourceCounter]::Count($portable.FullName, 3)
  Assert-Condition ($unpackedGroupIcons -gt 0 -and $unpackedIcons -gt 0) 'Unpacked executable has no icon resources.'
  Assert-Condition ($portableGroupIcons -gt 0 -and $portableIcons -gt 0) 'Portable executable has no icon resources.'

  $manifest = [ordered]@{
    schemaVersion = 1
    build = [ordered]@{
      version = $ExpectedVersion
      arch = $ExpectedArch
      commitSha = if ($env:GITHUB_SHA) { $env:GITHUB_SHA } else { 'local' }
      signing = 'unsigned'
    }
    artifacts = [ordered]@{
      nsis = [ordered]@{
        file = $setup.Name
        size = $setup.Length
        sha512 = Get-Sha512Base64 $setup.FullName
        signatureStatus = $setupSignature
        blockmap = [ordered]@{ file = $blockmap.Name; size = $blockmap.Length; sha512 = Get-Sha512Base64 $blockmap.FullName }
        updaterMetadata = $updaterMetadata
      }
      portable = [ordered]@{
        file = $portable.Name
        size = $portable.Length
        sha512 = Get-Sha512Base64 $portable.FullName
        signatureStatus = $portableSignature
      }
      unpacked = [ordered]@{
        executable = 'win-unpacked/Nevermind.exe'
        size = $unpacked.Length
        sha512 = Get-Sha512Base64 $unpacked.FullName
        signatureStatus = $unpackedSignature
        asar = [ordered]@{ file = 'win-unpacked/resources/app.asar'; size = $asar.Length; sha512 = Get-Sha512Base64 $asar.FullName }
      }
    }
    iconEvidence = [ordered]@{
      source = 'build/Icon.icon/Assets/icon.png'
      sourceSha256 = Get-Sha256Hex $iconSource
      sourceWidth = $iconWidth
      sourceHeight = $iconHeight
      builderConfigMatchesSource = $true
      unpackedGroupIconResources = $unpackedGroupIcons
      unpackedIconResources = $unpackedIcons
      portableGroupIconResources = $portableGroupIcons
      portableIconResources = $portableIcons
      claim = 'configured-source-and-PE-resource-presence-only'
    }
  }
  $manifestPath = Join-Path $ResolvedPackageRoot 'windows-smoke-manifest.json'
  $manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $manifestPath -Encoding utf8
  Write-Host "Windows package verification passed: $manifestPath"
} finally {
  if (Test-Path -LiteralPath $ExtractionRoot) {
    Remove-Item -LiteralPath $ExtractionRoot -Recurse -Force
  }
}
