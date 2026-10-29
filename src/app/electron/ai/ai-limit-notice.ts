import { freeSpendCapNoticeFromText } from '../backend/free-spend-cap-notice';
import { getNevermindDashboardUrl } from '../backend/nevermind-auth';
import type { CommandAction } from '../../palette/model';
import { pricingUnavailableNoticeFromText } from './ai-pricing-notice';

export type AiLimitNotice = {
  kind:
    | 'insufficient_credits'
    | 'free_spend_cap_reached'
    | 'rate_limited'
    | 'prompt_too_large'
    | 'request_too_large'
    | 'unsupported_request_shape'
    | 'pricing_unavailable'
    | 'unsupported_client';
  title: string;
  message: string;
  actionTitle?: string;
  action?: CommandAction;
  retryAfterSec?: number;
};

export function aiLimitNoticeFromError(error: unknown): AiLimitNotice | null {
  const text = searchableErrorText(error);
  const retryAfterSec =
    Number(text.match(/retry[_ -]?after[^0-9]*(\d+)/i)?.[1] || 0) || undefined;
  const freeSpendNotice = freeSpendCapNoticeFromText(text);
  if (freeSpendNotice) {
    return {
      ...freeSpendNotice,
      actionTitle: 'Open Dashboard',
      action: {
        type: 'openUrl',
        title: 'Open Dashboard',
        url: getNevermindDashboardUrl(),
      },
    };
  }
  const pricingNotice = pricingUnavailableNoticeFromText(text);
  if (pricingNotice) return pricingNotice;
  if (/insufficient[_ -]?credits|no credits remaining|402\b/i.test(text)) {
    return {
      kind: 'insufficient_credits',
      title: 'Credits needed',
      message:
        'You’ve reached your Nevermind AI credit limit. Open your dashboard to review your account.',
      actionTitle: 'Open Dashboard',
      action: {
        type: 'openUrl',
        title: 'Open Dashboard',
        url: getNevermindDashboardUrl(),
      },
    };
  }
  if (/rate[_ -]?limited|rate limit exceeded|429\b/i.test(text)) {
    return {
      kind: 'rate_limited',
      title: 'AI limit reached',
      message: retryAfterSec
        ? `You’re sending messages too quickly. Try again in about ${retryAfterSec} seconds, or open your dashboard for account details.`
        : 'You’ve reached a Nevermind AI usage limit. Open your dashboard for account details.',
      actionTitle: 'Open Dashboard',
      action: {
        type: 'openUrl',
        title: 'Open Dashboard',
        url: getNevermindDashboardUrl(),
      },
      retryAfterSec,
    };
  }
  if (/request[_ -]?too[_ -]?large/i.test(text)) {
    return {
      kind: 'request_too_large',
      title: 'AI request too large',
      message:
        'Remove some images or start a new chat to make this request smaller.',
    };
  }
  if (/unsupported[_ -]?request[_ -]?shape/i.test(text)) {
    return {
      kind: 'unsupported_request_shape',
      title: 'Unsupported AI request',
      message:
        'This AI request uses an input or tool that the selected route cannot safely process. Remove it or switch to a supported model.',
    };
  }
  if (/prompt[_ -]?too[_ -]?large|prompt exceeds|413\b/i.test(text)) {
    return {
      kind: 'prompt_too_large',
      title: 'Prompt too large',
      message:
        'This chat history is too large for one request. Start a new chat, or send a shorter message without pasting large outputs.',
    };
  }
  if (
    /unsupported[_ -]?client|unsupported[_ -]?desktop|unsupported[_ -]?api|no longer supported|426\b/i.test(
      text,
    )
  ) {
    return {
      kind: 'unsupported_client',
      title: 'Update Nevermind',
      message:
        'This version of Nevermind is no longer supported by the backend. Install the latest version to keep using AI features.',
      actionTitle: 'Check for Update',
      action: { type: 'checkForUpdates', title: 'Check for Update' },
    };
  }
  return null;
}

function searchableErrorText(error: unknown) {
  if (!error) return '';
  if (!(error instanceof Error)) return stringifyError(error);
  const own = Object.fromEntries(
    Object.getOwnPropertyNames(error).map((key) => [key, (error as any)[key]]),
  );
  return [error.name, error.message, error.stack, stringifyError(own)]
    .filter(Boolean)
    .join('\n');
}

function stringifyError(value: unknown) {
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}
