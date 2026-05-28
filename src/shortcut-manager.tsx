import { Keyboard, RotateCcw, Trash2 } from 'lucide-react'
import { RootCommandList } from './command-list'
import type { CommandItem } from './model'
import type { ActionPanelRow } from './ui'
import { EMPTY_SHORTCUTS_TITLE, shortcutLabel } from './ui'

type ShortcutAction = { id: string; title: string; subtitle?: string }
export type ShortcutRecordLike = { actionId: string; accelerator: string; action: ShortcutAction }

export function shortcutItems(records: ShortcutRecordLike[], matches: (...values: Array<string | undefined>) => boolean): CommandItem[] {
  return records.map((record): CommandItem => ({
    id: `shortcut:${record.actionId}`,
    title: record.action.title,
    subtitle: shortcutLabel(record.accelerator),
    icon: 'keyboard',
    primaryAction: { type: 'nativeAction', title: 'Change shortcut', nativeAction: record.action },
  })).filter((item) => matches(item.title, item.subtitle, recordAcceleratorsForSearch(records, item.id)))
}

function recordAcceleratorsForSearch(records: ShortcutRecordLike[], itemId: string) {
  return records.find((record) => `shortcut:${record.actionId}` === itemId)?.accelerator
}

export function ShortcutManagerView({ records, matches, onSelect }: { records: ShortcutRecordLike[]; matches: (...values: Array<string | undefined>) => boolean; onSelect: (record: ShortcutRecordLike) => void }) {
  const items = shortcutItems(records, matches)
  return <RootCommandList
    items={items}
    iconForItem={() => <Keyboard size={18} />}
    onSelect={(item) => {
      const record = records.find((candidate) => `shortcut:${candidate.actionId}` === item.id)
      if (record) onSelect(record)
    }}
    emptyTitle={EMPTY_SHORTCUTS_TITLE}
  />
}

export function shortcutOptionRows(record: ShortcutRecordLike | null, startRecorder: (action: ShortcutAction) => void, removeShortcut: (record: ShortcutRecordLike) => void, matches: (...values: Array<string | undefined>) => boolean): ActionPanelRow[] {
  if (!record) return []
  return [
    {
      value: 'shortcut-option:change',
      icon: <Keyboard size={18} />,
      title: 'Change shortcut',
      subtitle: shortcutLabel(record.accelerator),
      onSelect: () => startRecorder(record.action),
      className: 'result',
    },
    {
      value: 'shortcut-option:remove',
      icon: <Trash2 size={18} />,
      title: 'Remove shortcut',
      subtitle: record.action.title,
      onSelect: () => removeShortcut(record),
      className: 'result dangerResult',
    },
  ].filter((row) => matches(row.title, row.subtitle))
}

export function shortcutRecorderRows(recordedShortcut: string, action: ShortcutAction | null, saveShortcut: () => void, cancel: () => void): ActionPanelRow[] {
  return [
    {
      value: 'shortcut:save',
      icon: <Keyboard size={18} />,
      title: recordedShortcut ? shortcutLabel(recordedShortcut) : 'Press a keyboard shortcut',
      subtitle: recordedShortcut ? `Save shortcut for “${action?.title}”` : 'Use at least one modifier, then press Enter',
      onSelect: saveShortcut,
      className: 'result',
    },
    {
      value: 'shortcut:cancel',
      icon: <RotateCcw size={18} />,
      title: 'Cancel',
      subtitle: 'Keep the current shortcut settings',
      onSelect: cancel,
      className: 'result',
    },
  ]
}
