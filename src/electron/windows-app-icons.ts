import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';

export type AppIconSource = string | { path: string; resourceIndex: number };

function visualElementsLogoPath(targetPath: string, manifest: string) {
  const logo = ['Square44x44Logo', 'Square70x70Logo', 'Square150x150Logo']
    .map(
      (attribute) =>
        manifest.match(
          new RegExp(`\\b${attribute}\\s*=\\s*["']([^"']+)["']`, 'i'),
        )?.[1],
    )
    .find(Boolean);
  if (!logo) return null;

  const appDirectory = path.win32.dirname(targetPath);
  const logoPath = path.win32.resolve(appDirectory, logo);
  const relativeLogoPath = path.win32.relative(appDirectory, logoPath);
  if (
    relativeLogoPath.startsWith('..') ||
    path.win32.isAbsolute(relativeLogoPath)
  ) {
    return null;
  }
  return logoPath;
}

export async function windowsShortcutIconSources(
  appPath: string,
  readShortcutLink: (shortcutPath: string) => {
    icon?: string;
    iconIndex?: number;
    target: string;
  },
  readTextFile: (filePath: string) => Promise<string> = (filePath) =>
    fs.readFile(filePath, 'utf8'),
) {
  try {
    const shortcut = readShortcutLink(appPath);
    const executableName = path.win32.basename(
      shortcut.target,
      path.win32.extname(shortcut.target),
    );
    const manifestPath = path.win32.join(
      path.win32.dirname(shortcut.target),
      `${executableName}.VisualElementsManifest.xml`,
    );
    const manifest = await readTextFile(manifestPath).catch(() => '');
    const visualElementsLogo = visualElementsLogoPath(
      shortcut.target,
      manifest,
    );
    const candidates: AppIconSource[] = [];
    if (shortcut.icon) {
      candidates.push({
        path: shortcut.icon,
        resourceIndex: shortcut.iconIndex ?? 0,
      });
    }
    for (const candidate of [visualElementsLogo, shortcut.target, appPath]) {
      if (candidate && !candidates.some((item) => item === candidate)) {
        candidates.push(candidate);
      }
    }
    return candidates;
  } catch {
    return [appPath];
  }
}

const WINDOWS_ICON_RESOURCE_SCRIPT = `
$null = Add-Type -AssemblyName System.Drawing
$null = Add-Type -Name IconExtractor -Namespace Nevermind -MemberDefinition @'
[System.Runtime.InteropServices.DllImport("shell32.dll", CharSet=System.Runtime.InteropServices.CharSet.Unicode)]
public static extern uint ExtractIconEx(string file, int index, System.IntPtr[] large, System.IntPtr[] small, uint count);
[System.Runtime.InteropServices.DllImport("user32.dll")]
public static extern bool DestroyIcon(System.IntPtr icon);
'@
$large = New-Object System.IntPtr[] 1
$small = New-Object System.IntPtr[] 1
$count = [Nevermind.IconExtractor]::ExtractIconEx($env:NVM_ICON_PATH, [int]$env:NVM_ICON_INDEX, $large, $small, 1)
if ($count -eq 0) { exit 1 }
$handle = if ($large[0] -ne [System.IntPtr]::Zero) { $large[0] } else { $small[0] }
$icon = [System.Drawing.Icon]::FromHandle($handle).Clone()
$bitmap = $icon.ToBitmap()
$stream = New-Object System.IO.MemoryStream
$bitmap.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
[Console]::Out.Write([Convert]::ToBase64String($stream.ToArray()))
$stream.Dispose()
$bitmap.Dispose()
$icon.Dispose()
if ($large[0] -ne [System.IntPtr]::Zero) { $null = [Nevermind.IconExtractor]::DestroyIcon($large[0]) }
if ($small[0] -ne [System.IntPtr]::Zero) { $null = [Nevermind.IconExtractor]::DestroyIcon($small[0]) }
`;

export function readWindowsIconResourcePng(
  iconPath: string,
  resourceIndex: number,
) {
  return new Promise<Buffer | null>((resolve) => {
    execFile(
      'powershell.exe',
      [
        '-NoLogo',
        '-NoProfile',
        '-NonInteractive',
        '-Command',
        WINDOWS_ICON_RESOURCE_SCRIPT,
      ],
      {
        encoding: 'utf8',
        env: {
          ...process.env,
          NVM_ICON_INDEX: String(resourceIndex),
          NVM_ICON_PATH: iconPath,
        },
        maxBuffer: 2 * 1024 * 1024,
        timeout: 5000,
        windowsHide: true,
      },
      (error, stdout) => {
        if (error || !stdout.trim()) {
          resolve(null);
          return;
        }
        resolve(Buffer.from(stdout.trim(), 'base64'));
      },
    );
  });
}
