import * as logger from '../platform/logger';
import {
  consumePortableUpdateResult,
  type PortableUpdateResult,
} from '../os/windows-portable-update';
import { portableUpdateWorkDirectory } from './windows-portable-updater';

type ReportableUpdateState = { status: string; errorMessage: string };

export function portableUpdateFailureMessage(result: PortableUpdateResult) {
  const reason = result.reason.replace(/^[a-z-]+:\s*/, '');
  if (result.outcome === 'rolled-back')
    return `Nevermind ${result.toVersion} did not start, so version ${result.fromVersion} was restored. ${reason}`;
  if (result.outcome === 'failed')
    return `Nevermind ${result.toVersion} was not installed. ${reason}`;
  return null;
}

/** Surfaces the detached helper's outcome from the previous launch in the Updates view. */
export async function reportPreviousPortableUpdate(
  state: ReportableUpdateState,
) {
  const result = await consumePortableUpdateResult(
    portableUpdateWorkDirectory(),
  );
  if (!result) return;
  const message = portableUpdateFailureMessage(result);
  const event = `updater.portable.${result.outcome}`;
  const scope = { source: 'host' as const, scope: 'updater' };
  if (!message) {
    logger.info(event, result, scope);
    return;
  }
  logger.error(event, new Error(message), scope);
  state.status = 'error';
  state.errorMessage = message;
}
