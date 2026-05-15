import { useEffect, useState } from 'react'

export function useSearchResults<T>(search: (query: string) => Promise<T[]>, query: string, refreshNonce: number) {
  const [results, setResults] = useState<T[]>([])

  useEffect(() => {
    let cancelled = false
    const timer = window.setTimeout(async () => {
      const next = await search(query)
      if (!cancelled) setResults(next)
    }, 20)

    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [search, query, refreshNonce])

  return [results, setResults] as const
}
