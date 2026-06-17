import { Search } from 'lucide-react';
import {
  type ActionPanelRow,
  ActionPanelView,
  EMPTY_ACTIONS_TITLE,
  EmptyState,
} from './ui';

export function ActionPanel({
  rows,
  emptyMessage = EMPTY_ACTIONS_TITLE,
  emptySubtitle,
}: {
  rows: ActionPanelRow[];
  emptyMessage?: string;
  emptySubtitle?: string;
}) {
  return (
    <ActionPanelView
      rows={rows}
      renderEmpty={() => (
        <EmptyState
          icon={<Search size={24} />}
          title={emptyMessage}
          subtitle={emptySubtitle}
        />
      )}
    />
  );
}
