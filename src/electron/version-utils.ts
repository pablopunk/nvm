export function versionParts(version: unknown) {
  return String(version || '')
    .split(/[.-]/)
    .map((part) => Number.parseInt(part, 10))
    .map((part) => Number.isFinite(part) ? part : 0)
}

export function isNewerVersion(version: unknown, current: unknown) {
  const nextParts = versionParts(version)
  const currentParts = versionParts(current)
  const length = Math.max(nextParts.length, currentParts.length)
  for (let index = 0; index < length; index += 1) {
    const next = nextParts[index] || 0
    const existing = currentParts[index] || 0
    if (next > existing) return true
    if (next < existing) return false
  }
  return false
}
