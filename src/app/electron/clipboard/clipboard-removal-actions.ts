import { clipboardItemTitle } from './clipboard-utils';

export function clipboardHistoryRemovalAction(
  range: string,
  title: string,
  message: string,
  itemId = '',
) {
  return {
    type: 'removeClipboardHistory',
    title,
    clipboardHistoryRange: range,
    clipboardHistoryItemId: itemId,
    style: 'destructive',
    requiresConfirmation: true,
    confirmLabel: title,
    confirmMessage: message,
  };
}

let clipboardHistoryRemovalByTimeActionCache: any;

function clipboardHistoryRemovalByTimeAction() {
  if (clipboardHistoryRemovalByTimeActionCache)
    return clipboardHistoryRemovalByTimeActionCache;
  const ranges = [
    {
      range: 'last-5-minutes',
      title: '5 Minutes',
      period: 'the last 5 minutes',
    },
    {
      range: 'last-hour',
      title: 'Hour',
      period: 'the last hour',
    },
    {
      range: 'last-day',
      title: 'Day',
      period: 'the last day',
    },
    {
      range: 'last-week',
      title: 'Week',
      period: 'the last week',
    },
  ];
  clipboardHistoryRemovalByTimeActionCache = {
    type: 'submenu',
    title: 'Remove Last...',
    submenu: {
      title: 'Remove Last...',
      sections: [
        {
          actions: ranges.map(({ range, title, period }) => ({
            ...clipboardHistoryRemovalAction(
              range,
              title,
              `Remove clipboard history entries copied in ${period}?`,
            ),
            confirmLabel: 'Remove',
          })),
        },
      ],
    },
  };
  return clipboardHistoryRemovalByTimeActionCache;
}

export function clipboardHistoryRemovalActions(
  historyLength: number,
  item: any = null,
) {
  if (historyLength === 0) return [];
  if (item?.id)
    return [
      clipboardHistoryRemovalAction(
        'item',
        'Remove Item',
        `Remove “${clipboardItemTitle(item)}” from clipboard history?`,
        item.id,
      ),
      clipboardHistoryRemovalByTimeAction(),
    ];
  return [
    clipboardHistoryRemovalAction(
      'last-hour',
      'Remove Entries from Last Hour',
      'Remove clipboard history entries copied in the last hour?',
    ),
    clipboardHistoryRemovalAction(
      'last-day',
      'Remove Entries from Last Day',
      'Remove clipboard history entries copied in the last day?',
    ),
    clipboardHistoryRemovalAction(
      'all',
      'Remove All Entries',
      'Remove all clipboard history entries?',
    ),
  ];
}
