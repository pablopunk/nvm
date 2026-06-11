export function sortFoundFiles(files: any[], options: any = {}) {
  const sortBy = options.sortBy || options.sort || null
  if (!sortBy) return files
  const direction = options.order === 'asc' ? 1 : -1
  const field = sortBy === 'recent' || sortBy === 'modified' ? 'mtimeMs'
    : sortBy === 'added' ? 'dateAddedMs'
      : sortBy === 'created' ? 'birthtimeMs'
        : sortBy === 'name' ? 'name'
          : sortBy === 'size' ? 'size'
            : sortBy
  return [...files].sort((a, b) => {
    const av = a[field] || 0
    const bv = b[field] || 0
    if (typeof av === 'string' || typeof bv === 'string') return direction * String(av).localeCompare(String(bv))
    return direction * (av - bv)
  })
}

export function findFilesNeedsStats(sortBy: unknown) {
  return ['added', 'created', 'recent', 'modified', 'size'].includes(String(sortBy || ''))
}

export function applyDateAdded(files: any[], dates: Map<string, number>) {
  for (const file of files) file.dateAddedMs = dates.get(file.path) || file.birthtimeMs || 0
  return files
}

export function selectFindFiles(files: any[], options: any = {}, limit = 100) {
  return sortFoundFiles(files, options).slice(0, limit)
}

export function includeDimensionsForFindOptions(options: any = {}) {
  return Boolean(options.includeDimensions)
}
