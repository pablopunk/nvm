#!/usr/bin/env node
const path = require('node:path')
const { spawn, spawnSync } = require('node:child_process')

const root = path.resolve(__dirname, '..')
const isWindows = process.platform === 'win32'
const bin = path.join(root, 'node_modules', '.bin', isWindows ? 'electron-vite.cmd' : 'electron-vite')

const prepare = spawnSync(process.execPath, [path.join(__dirname, 'prepare-dev.cjs')], {
  cwd: root,
  stdio: 'inherit',
})

if (prepare.status !== 0) process.exit(prepare.status || 1)

const child = spawn(bin, ['dev', '--watch'], {
  cwd: root,
  stdio: 'inherit',
  shell: isWindows,
  detached: !isWindows,
  env: process.env,
})

let shuttingDown = false
let fallbackTimer

function killChild(signal = 'SIGINT') {
  if (child.killed) return
  if (isWindows) {
    spawn('taskkill', ['/pid', String(child.pid), '/t', '/f'], { stdio: 'ignore' })
    return
  }
  try {
    process.kill(-child.pid, signal)
  } catch {
    try { child.kill(signal) } catch {}
  }
}

function shutdown(signal = 'SIGINT') {
  if (shuttingDown) return
  shuttingDown = true
  killChild(signal)
  fallbackTimer = setTimeout(() => killChild('SIGKILL'), 1500)
  fallbackTimer.unref?.()
}

process.on('SIGINT', () => shutdown('SIGINT'))
process.on('SIGTERM', () => shutdown('SIGTERM'))
process.on('SIGHUP', () => shutdown('SIGHUP'))
process.on('exit', () => {
  if (!shuttingDown) killChild('SIGTERM')
})

child.on('exit', (code, signal) => {
  if (fallbackTimer) clearTimeout(fallbackTimer)
  process.exit(code ?? (signal ? 1 : 0))
})

child.on('error', (error) => {
  console.error(error)
  process.exit(1)
})
