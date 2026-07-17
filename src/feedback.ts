import type { CommandAction, CommandItem, CommandView } from './model';

export type FeedbackTone = 'info' | 'success' | 'error';

type FeedbackViewInput = {
  id?: string;
  title: string;
  message: string;
  tone?: FeedbackTone;
  details?: Array<{ title: string; subtitle?: string }>;
  actions?: CommandAction[];
};

const feedbackAppearance = {
  info: { foreground: 'blue' },
  success: { foreground: 'green' },
  error: { foreground: 'red' },
} as const;

const feedbackIcon = {
  info: 'info',
  success: 'circle-check',
  error: 'circle-alert',
} as const;

export function feedbackView({
  id = 'feedback',
  title,
  message,
  tone = 'error',
  details = [],
  actions = [],
}: FeedbackViewInput): CommandView {
  const navigationActions = actions.length
    ? actions
    : [{ type: 'popView', title: 'Back' } as CommandAction];
  const items: CommandItem[] = [
    {
      id: `${id}:message`,
      title,
      subtitle: message,
      icon: feedbackIcon[tone],
      appearance: feedbackAppearance[tone],
      disabled: true,
    },
    ...details.map((detail, index) => ({
      id: `${id}:detail:${index}`,
      title: detail.title,
      subtitle: detail.subtitle,
      icon: 'minus',
      disabled: true,
    })),
    ...navigationActions.map((action, index) => ({
      id: `${id}:action:${index}`,
      title: action.title || 'Continue',
      subtitle: action.subtitle || action.description,
      icon: action.style === 'destructive' ? 'triangle-alert' : 'arrow-left',
      appearance:
        action.style === 'destructive'
          ? { foreground: 'red' as const }
          : undefined,
      primaryAction: action,
    })),
  ];
  return {
    id,
    type: 'list',
    title,
    subtitle: message,
    items,
    selectedItemId: `${id}:action:0`,
  };
}
