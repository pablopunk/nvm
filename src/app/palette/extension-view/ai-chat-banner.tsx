import { CreditCard, ShieldAlert } from 'lucide-react';
import type { ReactNode } from 'react';
import type { AiLimitState } from '../ai-chat/use-ai-chat';
import type { CommandAction } from '../model';
import { EmptyState } from '../ui';

const AI_DATA_USE_NOTICE =
  'Nevermind AI sends your messages to third-party model providers. Some providers may use prompts and responses to improve their models.';

function limitGateAction(
  limit: AiLimitState,
  runAction: (action: CommandAction) => void,
) {
  const action = limit.action;
  if (!action) {
    return;
  }
  return {
    value: 'run-limit-action',
    icon: <CreditCard size={16} />,
    title: limit.actionTitle || action.title,
    onSelect: () => runAction(action),
  };
}

function NevermindLimitGate({
  limit,
  runAction,
}: {
  limit: AiLimitState;
  runAction: (action: CommandAction) => void;
}) {
  return (
    <EmptyState
      icon={<CreditCard size={24} />}
      title={limit.title}
      subtitle={limit.message}
      action={limitGateAction(limit, runAction)}
    />
  );
}

function AiChatNotice({ icon, text }: { icon: ReactNode; text: string }) {
  return (
    <div className="creditNoticeBanner">
      {icon}
      <span>{text}</span>
    </div>
  );
}

export function AiChatBanner({
  limit,
  creditNotice,
  conversationIsEmpty,
  runAction,
}: {
  limit: AiLimitState | null;
  creditNotice: string | null;
  conversationIsEmpty: boolean;
  runAction: (action: CommandAction) => void;
}) {
  if (limit) {
    return <NevermindLimitGate limit={limit} runAction={runAction} />;
  }
  if (creditNotice) {
    return <AiChatNotice icon={<CreditCard size={14} />} text={creditNotice} />;
  }
  if (conversationIsEmpty) {
    return (
      <AiChatNotice
        icon={<ShieldAlert size={14} />}
        text={AI_DATA_USE_NOTICE}
      />
    );
  }
  return null;
}
