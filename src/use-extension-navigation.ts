import { useState } from 'react'
import type { CommandView } from './model'

export type NavigationMode = 'root' | 'push' | 'replace' | 'pop'

export function useExtensionNavigation() {
  const [view, setView] = useState<CommandView | null>(null)
  const [backStack, setBackStack] = useState<CommandView[]>([])

  function showView(nextView: CommandView, navigation: Exclude<NavigationMode, 'pop'> = 'replace') {
    if (navigation === 'root') setBackStack([])
    if (navigation === 'push' && view) setBackStack((stack) => [...stack, view])
    setView(nextView)
  }

  function popView() {
    let didPop = false
    setBackStack((stack) => {
      const next = [...stack]
      const previous = next.pop() || null
      setView(previous)
      didPop = Boolean(previous)
      return next
    })
    return didPop
  }

  function clearView() {
    setView(null)
    setBackStack([])
  }

  return { view, setView, backStack, setBackStack, showView, popView, clearView }
}
