const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

if (process.platform !== 'win32') process.exit(0);

const root = path.resolve(__dirname, '..');
const framework = path.join(
  process.env.SystemRoot || 'C:\\Windows',
  'Microsoft.NET',
  'Framework64',
  'v4.0.30319',
);
const outputDirectory = path.join(root, 'build/native');
fs.mkdirSync(outputDirectory, { recursive: true });
execFileSync(
  path.join(framework, 'csc.exe'),
  [
    '/nologo',
    '/target:exe',
    '/platform:anycpu',
    '/optimize+',
    `/out:${path.join(outputDirectory, 'windows-desktop-text.exe')}`,
    ...[
      'UIAutomationClient.dll',
      'UIAutomationTypes.dll',
      'WindowsBase.dll',
    ].map((name) => `/reference:${path.join(framework, 'WPF', name)}`),
    path.join(root, 'src/app/resources/windows-desktop-text.cs'),
  ],
  { stdio: 'inherit', windowsHide: true },
);
