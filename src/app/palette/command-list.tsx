import { Search, Sparkles } from 'lucide-react';
import React, { type DragEvent, type ReactNode } from 'react';
import {
  actionsFromPanel,
  type CommandAction,
  type CommandItem,
} from './model';
import {
  CommandRow,
  EMPTY_ROOT_SUBTITLE,
  EMPTY_ROOT_TITLE,
  EmptyState,
} from './ui';

export type RootCommandListProps = {
  items: CommandItem[];
  iconForItem: (item: CommandItem) => ReactNode;
  onSelect: (item: CommandItem) => void;
  emptyTitle?: string;
  emptySubtitle?: string;
  isLoading?: boolean;
  extraForItem?: (item: CommandItem) => string[];
  dragPathForItem?: (item: CommandItem) => string | null | undefined;
  startItemDrag?: (event: DragEvent, item: CommandItem) => void;
};

function isGlobalShortcut(action?: CommandAction) {
  return action?.shortcutScope === 'global' || action?.type === 'nativeAction';
}

function itemShortcut(item: CommandItem, primaryAction?: CommandAction) {
  const persistentAction = item.persistentAction as CommandAction | undefined;
  const global =
    item.shortcutScope === 'global' ||
    isGlobalShortcut(primaryAction) ||
    isGlobalShortcut(persistentAction);
  const shortcut =
    item.shortcut || primaryAction?.shortcut || persistentAction?.shortcut;
  return { shortcut, selectedOnly: Boolean(shortcut && !global) };
}

export function RootCommandList({
  items,
  iconForItem,
  onSelect,
  emptyTitle = EMPTY_ROOT_TITLE,
  emptySubtitle = EMPTY_ROOT_SUBTITLE,
  isLoading = false,
  extraForItem,
  dragPathForItem,
  startItemDrag,
}: RootCommandListProps) {
  if (items.length === 0 && isLoading) return null;
  if (items.length === 0)
    return (
      <EmptyState
        icon={<Search size={24} />}
        title={emptyTitle}
        subtitle={emptySubtitle}
      />
    );
  return (
    <>
      {items.map((item) => {
        const primaryAction =
          item.primaryAction ||
          actionsFromPanel(item.actionPanel, item.actions || [])[0];
        const { shortcut, selectedOnly } = itemShortcut(item, primaryAction);
        const dragPath = dragPathForItem?.(item);
        return (
          <CommandRow
            key={item.id}
            value={item.id}
            icon={iconForItem(item) || <Sparkles size={18} />}
            title={item.title}
            subtitle={item.subtitle || item.text}
            accessories={item.accessories}
            className={item.className}
            shortcut={shortcut}
            appearance={item.appearance}
            extras={extraForItem?.(item)}
            selectedOnlyShortcut={selectedOnly}
            draggable={Boolean(dragPath && startItemDrag)}
            onDragStart={
              dragPath && startItemDrag
                ? (event) => startItemDrag(event, item)
                : undefined
            }
            onSelect={() => onSelect(item)}
          />
        );
      })}
    </>
  );
}
