import { execFile } from 'node:child_process';

type AppleScriptResult = {
  stdout: string;
  stderr: string;
  exitCode: number;
};

export type SystemAudioMuteLease = {
  restore(): Promise<void>;
};

type SystemAudioMuteDependencies = {
  platform?: NodeJS.Platform;
  runAppleScript?: (script: string) => Promise<AppleScriptResult>;
};

function runAppleScript(script: string) {
  return new Promise<AppleScriptResult>((resolve) => {
    execFile(
      'osascript',
      ['-e', script],
      { timeout: 5_000 },
      (error, stdout, stderr) =>
        resolve({
          stdout,
          stderr: stderr || error?.message || '',
          exitCode: error ? 1 : 0,
        }),
    );
  });
}

export function createSystemAudioMuteCapability(
  dependencies: SystemAudioMuteDependencies = {},
) {
  const platform = dependencies.platform ?? process.platform;
  const executeAppleScript = dependencies.runAppleScript ?? runAppleScript;

  async function temporarilyMute(): Promise<SystemAudioMuteLease> {
    if (platform !== 'darwin') return { restore: async () => {} };
    const muted = await executeAppleScript(
      'set wasMuted to output muted of (get volume settings)\nset volume with output muted\nreturn wasMuted',
    );
    if (muted.exitCode !== 0)
      throw new Error(muted.stderr || 'Could not mute system audio');
    const wasMuted = muted.stdout.trim().toLowerCase() === 'true';
    let restored = false;
    let restoration: Promise<void> | null = null;

    return {
      async restore() {
        if (restored) return;
        if (!restoration)
          restoration = executeAppleScript(
            `set volume with output muted ${wasMuted}`,
          )
            .then((result) => {
              if (result.exitCode !== 0)
                throw new Error(
                  result.stderr || 'Could not restore system audio',
                );
              restored = true;
            })
            .finally(() => {
              restoration = null;
            });
        await restoration;
      },
    };
  }

  return { temporarilyMute };
}
