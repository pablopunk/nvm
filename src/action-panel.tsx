import { Search } from 'lucide-react'
import { ActionPanelView, EmptyState, type ActionPanelRow } from './ui'

export function ActionPanel({ rows, emptyMessage = 'No actions found', emptySubtitle }: { rows: ActionPanelRow[]; emptyMessage?: string; emptySubtitle?: string }) {
  return <ActionPanelView rows={rows} renderEmpty={() => <EmptyState icon={<Search size={24} />} title={emptyMessage} subtitle={emptySubtitle} />} />
}
