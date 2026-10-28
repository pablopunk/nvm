import {
  FAILURE_MESSAGES,
  type FailedResult,
  failure,
  type Settled,
} from './window-management-failures';
import type {
  HelperCode,
  HelperEnvelope,
  SetFrameData,
  WindowApplyFailureReason,
} from './window-management-types';

const HELPER_FAILURE_REASONS: Partial<
  Record<HelperCode, WindowApplyFailureReason>
> = {
  denied: 'denied',
  'no-window': 'no-window',
  'target-lost': 'target-lost',
  unsupported: 'unsupported',
};

export function helperFailure(envelope: HelperEnvelope<unknown>): FailedResult {
  const reason = HELPER_FAILURE_REASONS[envelope.code] ?? 'unapplied';
  return failure(reason, envelope.message ?? FAILURE_MESSAGES[reason]);
}

export function unwrapEnvelope<Data>(
  envelope: HelperEnvelope<Data>,
): Settled<Data> {
  if (envelope.ok && envelope.data !== undefined) {
    return { ok: true, value: envelope.data };
  }
  return { ok: false, failure: helperFailure(envelope) };
}

export function settleWrite(
  envelope: HelperEnvelope<SetFrameData>,
): Settled<SetFrameData> {
  const settled = unwrapEnvelope(envelope);
  if (!settled.ok || settled.value.outcome !== 'unapplied') {
    return settled;
  }
  return {
    ok: false,
    failure: failure(
      'unapplied',
      settled.value.reason ?? FAILURE_MESSAGES.unapplied,
    ),
  };
}
