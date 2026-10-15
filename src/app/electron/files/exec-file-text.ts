import { execFile } from 'node:child_process';

export function execFileText(
  command: string,
  args: string[] = [],
  options = {},
): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(command, args, options, (error, stdout) =>
      error ? reject(error) : resolve(stdout),
    );
  });
}
