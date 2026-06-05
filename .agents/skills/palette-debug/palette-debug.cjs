#!/usr/bin/env node
const { spawnSync } = require('node:child_process')
const path = require('node:path')

const args = process.argv.slice(2)
let query = ''
let execute = ''
let skipBuild = false

for (let i = 0; i < args.length; i += 1) {
  const arg = args[i]
  if (arg === '--query' || arg === '-q') query = args[++i] || ''
  else if (arg === '--execute' || arg === '-x') execute = args[++i] || ''
  else if (arg === '--no-build') skipBuild = true
  else if (!query) query = arg
}

function run(command, commandArgs, options = {}) {
  const result = spawnSync(command, commandArgs, { stdio: 'inherit', ...options })
  if (result.status !== 0) process.exit(result.status || 1)
}

if (!skipBuild) run('pnpm', ['build'])

const electronBin = path.join(process.cwd(), 'node_modules', '.bin', process.platform === 'win32' ? 'electron.cmd' : 'electron')
run(electronBin, ['.'], {
  env: {
    ...process.env,
    NVM_PALETTE_DEBUG: '1',
    NVM_PALETTE_QUERY: query,
    ...(execute ? { NVM_PALETTE_EXECUTE: execute } : {}),
  },
})
