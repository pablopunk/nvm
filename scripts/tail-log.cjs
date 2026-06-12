#!/usr/bin/env node
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

const names = ['Nevermind', 'nvm'];
const candidates =
  process.platform === 'darwin'
    ? names.map((name) =>
        path.join(os.homedir(), 'Library', 'Logs', name, 'nevermind.log'),
      )
    : process.platform === 'win32'
      ? names.map((name) =>
          path.join(
            process.env.APPDATA ||
              path.join(os.homedir(), 'AppData', 'Roaming'),
            name,
            'logs',
            'nevermind.log',
          ),
        )
      : names.map((name) =>
          path.join(
            process.env.XDG_CONFIG_HOME || path.join(os.homedir(), '.config'),
            name,
            'logs',
            'nevermind.log',
          ),
        );

const logPath =
  candidates.find((candidate) => fs.existsSync(candidate)) || candidates[0];
fs.mkdirSync(path.dirname(logPath), { recursive: true });
fs.closeSync(fs.openSync(logPath, 'a'));
console.log(`Tailing ${logPath}`);

const tail = spawn('tail', ['-n', '200', '-F', logPath], { stdio: 'inherit' });
tail.on('exit', (code, signal) => process.exit(code || (signal ? 1 : 0)));
