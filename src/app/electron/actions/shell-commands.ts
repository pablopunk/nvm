import { execFile, spawn } from 'node:child_process';
import os from 'node:os';
import path from 'node:path';
import { expandUserPath } from '../files/file-utils';
import { limitedOutput } from '../platform/limited-output';

export function shellResultView(title, result) {
  return {
    type: 'preview',
    title,
    content: `Exit code: ${result.exitCode ?? 0}\n\nSTDOUT\n${result.stdout || '-'}\n\nSTDERR\n${result.stderr || '-'}`,
  };
}

export function createShellCommands() {
  let cachedUserShellPathPromise: Promise<string> | null = null;

  function resolveUserShellPath(): Promise<string> {
    if (cachedUserShellPathPromise) return cachedUserShellPathPromise;
    cachedUserShellPathPromise = new Promise((resolve) => {
      const userShell = process.platform === 'win32' ? '' : process.env.SHELL;
      if (!userShell) return resolve('');
      const delimiter = '__NVM_PATH_DELIM__';
      execFile(
        userShell,
        ['-ilc', `printf '%s%s%s' '${delimiter}' "$PATH" '${delimiter}'`],
        { timeout: 5000 },
        (_error, stdout) => {
          resolve(
            typeof stdout === 'string' ? stdout.split(delimiter)[1] || '' : '',
          );
        },
      );
    });
    return cachedUserShellPathPromise;
  }

  async function shellSpawnEnv(extraEnv?: Record<string, string>) {
    const userShellPath = await resolveUserShellPath();
    const mergedPath = [userShellPath, process.env.PATH]
      .filter(Boolean)
      .join(path.delimiter);
    return {
      ...process.env,
      ...(mergedPath ? { PATH: mergedPath } : {}),
      ...(extraEnv || {}),
    };
  }

  async function runShellCommand(command, args = [], options: any = {}) {
    const env = await shellSpawnEnv(options.env);
    const expandedCommand = expandUserPath(String(command));
    const expandedArgs = Array.isArray(args)
      ? args.map((arg) => expandUserPath(String(arg)))
      : [];
    const safeTimeoutMs = Math.max(
      1000,
      Math.min(Number(options.timeout || 30_000), 120_000),
    );

    return new Promise((resolve) => {
      const child = spawn(expandedCommand, expandedArgs, {
        cwd: options.cwd ? expandUserPath(options.cwd) : undefined,
        env,
        shell: Boolean(options.shell),
        timeout: safeTimeoutMs,
      });
      let stdout = '';
      let stderr = '';
      let settled = false;

      const settle = (result: {
        stdout: string;
        stderr: string;
        exitCode: number;
      }) => {
        if (settled) return;
        settled = true;
        resolve(result);
      };

      child.stdout?.on('data', (chunk) => {
        stdout = limitedOutput(stdout + chunk.toString(), options.outputLimit);
      });
      child.stderr?.on('data', (chunk) => {
        stderr = limitedOutput(stderr + chunk.toString(), options.outputLimit);
      });
      child.on('error', (error) =>
        settle({ stdout, stderr: stderr || error.message, exitCode: 1 }),
      );
      child.on('close', (exitCode) => settle({ stdout, stderr, exitCode }));

      // Hard kill after timeout + 5s grace period. Node's spawn timeout sends
      // SIGTERM; if the process ignores it we escalate to SIGKILL.
      const killer = setTimeout(() => {
        if (!(settled || child.killed)) {
          child.kill('SIGKILL');
          settle({
            stdout,
            stderr: `${stderr}\nKilled after ${safeTimeoutMs + 5000}ms`,
            exitCode: -1,
          });
        }
      }, safeTimeoutMs + 5000);
      killer.unref();
      child.on('close', () => clearTimeout(killer));
    });
  }

  function runShellScript(script, options: any = {}) {
    const home = os.homedir();
    const resolvedScript = String(script).replace(/~(?=\/|$)/g, home);
    return runShellCommand(
      options.shell || '/bin/bash',
      ['-lc', resolvedScript],
      { ...options, shell: false },
    );
  }

  return { runShellCommand, runShellScript, shellSpawnEnv };
}
