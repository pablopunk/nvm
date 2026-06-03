#!/usr/bin/env node
const fs = require('node:fs')
const path = require('node:path')

const configPath = path.join(process.cwd(), 'electron-builder.yml')
const config = fs.readFileSync(configPath, 'utf8')

function fail(message) {
  console.error(`Packaged resource check failed: ${message}`)
  process.exit(1)
}

if (!fs.existsSync(path.join(process.cwd(), 'src', 'resources', 'nevermind-extension-api.d.ts'))) {
  fail('missing src/resources/nevermind-extension-api.d.ts')
}

if (/^\s*-\s+src\/resources\/\*\*\s*$/m.test(config)) {
  fail('src/resources must be packaged with a FileSet; electron-builder default app-root ignores drop .d.ts files')
}

if (!/from:\s*src\/resources[\s\S]*to:\s*src\/resources[\s\S]*filter:[\s\S]*['"]?\*\*\/\*['"]?/m.test(config)) {
  fail('electron-builder.yml must include src/resources as a FileSet so the extension API declaration is packaged')
}

console.log('Packaged resource checks passed')
