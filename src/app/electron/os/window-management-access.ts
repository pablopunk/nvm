import { type FailedResult, failure } from './window-management-failures';
import type { MacosWindowManager } from './window-management-types';

export async function windowAccessFailure(
  manager: Pick<MacosWindowManager, 'permissionState'>,
): Promise<FailedResult | null> {
  return (await manager.permissionState()) === 'allowed'
    ? null
    : failure('denied');
}
