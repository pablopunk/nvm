#!/usr/bin/env node
const fs = require('node:fs')
const path = require('node:path')

const source = fs.readFileSync(path.join(process.cwd(), 'src', 'electron', 'main.ts'), 'utf8')

function fail(message) {
  console.error(`Clone-safety check failed: ${message}`)
  process.exit(1)
}

if (!/function\s+normalizeViewItems[\s\S]*const\s+\{\s*run,\s*__handler,\s*action,\s*\.\.\.safeItem\s*\}\s*=\s*item/.test(source)) {
  fail('normalizeViewItems must strip raw run/__handler/action fields before renderer IPC')
}

if (!/function\s+normalizeActionPanel[\s\S]*const\s+\{\s*lazyActions,\s*\.\.\.safeSection\s*\}\s*=\s*section/.test(source)) {
  fail('normalizeActionPanel must strip lazyActions after normalizing them')
}

if (!/async\s+function\s+searchActions[\s\S]*structuredClone\(sorted\)[\s\S]*return\s+sorted/.test(source)) {
  fail('searchActions must structuredClone-check results before returning through IPC')
}

if (!/async\s+function\s+executeActionForIpc[\s\S]*structuredClone\(result\)/.test(source)) {
  fail('executeActionForIpc must structuredClone-check action results before returning through IPC')
}

console.log('Clone-safety checks passed')
