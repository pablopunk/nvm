import crypto from 'node:crypto'

export function normalize(value: unknown) {
  return String(value || '').toLowerCase().trim()
}

export function hashValue(value: unknown) {
  return crypto.createHash('sha1').update(String(value)).digest('hex')
}

export function scoreNormalized(value: unknown, q: string) {
  if (!q) return 0
  const v = normalize(value)
  if (v === q) return 100
  if (v.startsWith(q)) return 80
  if (v.includes(q)) return 50
  let pos = 0
  for (const ch of q) {
    pos = v.indexOf(ch, pos)
    if (pos === -1) return 0
    pos += 1
  }
  return 20
}

export function score(value: unknown, query: unknown) {
  return scoreNormalized(value, normalize(query))
}

export function isLikelyUrl(input: string) {
  const value = input.trim()
  if (!value || value.includes(' ')) return false
  if (/^https?:\/\//i.test(value)) return true
  return /^[\w-]+(\.[\w-]+)+([/:?#].*)?$/i.test(value)
}

export function getUrlFromQuery(query: string) {
  const trimmed = query.trim()
  const opened = trimmed.match(/^open\s+(.+)$/i)?.[1]?.trim() || trimmed
  if (!isLikelyUrl(opened)) return null
  return /^https?:\/\//i.test(opened) ? opened : `https://${opened}`
}

export function calculate(query: string) {
  const expression = query.trim().replace(/^=?\s*/, '').replace(/^calc(?:ulate)?\s+/i, '')
  if (!expression || !/[+\-*/%^()]/.test(expression)) return null
  if (!/^[\d\s.+\-*/%^(),]+$/.test(expression)) return null

  try {
    const jsExpression = expression.replace(/%/g, '/100')
    const result = Function(`"use strict"; return (${jsExpression})`)()
    if (typeof result !== 'number' || !Number.isFinite(result)) return null
    return Number.isInteger(result) ? String(result) : String(Number(result.toPrecision(12)))
  } catch {
    return null
  }
}
