import { app } from 'electron';
import { portableExecutableFromEnvironment } from './windows-portable-update';

type ProtocolRegistrar = Pick<typeof app, 'setAsDefaultProtocolClient'>;

/** Portable builds run from a temporary extraction, so links must reopen the user's launcher, which updates keep at the same path. */
export function registerDeepLinkProtocol(
  scheme: string,
  environment: NodeJS.ProcessEnv = process.env,
  registrar: ProtocolRegistrar = app,
) {
  const portable = portableExecutableFromEnvironment(environment);
  return portable
    ? registrar.setAsDefaultProtocolClient(scheme, portable.executablePath, [])
    : registrar.setAsDefaultProtocolClient(scheme);
}
