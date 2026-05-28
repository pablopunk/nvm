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

const astroBin = path.join(root, 'backend', 'node_modules', '.bin', isWindows ? 'astro.cmd' : 'astro')
const backend = spawn(astroBin, ['dev'], {
  cwd: path.join(root, 'backend'),
  stdio: 'inherit',
  shell: isWindows,
  detached: !isWindows,
  env: process.env,
})

const child = spawn(bin, ['dev', '--watch'], {
  cwd: root,
  stdio: 'inherit',
  shell: isWindows,
  detached: !isWindows,
  env: process.env,
})

let shuttingDown = false
let fallbackTimer

function collectDescendants(rootPid) {
  if (isWindows) return []
  const out = spawnSync('ps', ['-Ao', 'pid=,ppid='], { encoding: 'utf8' })
  if (out.status !== 0) return []
  const byParent = new Map()
  for (const line of out.stdout.split('\n')) {
    const [pidStr, ppidStr] = line.trim().split(/\s+/)
    const pid = Number(pidStr), ppid = Number(ppidStr)
    if (!pid || !ppid) continue
    if (!byParent.has(ppid)) byParent.set(ppid, [])
    byParent.get(ppid).push(pid)
  }
  const pids = []
  const stack = [rootPid]
  while (stack.length) {
    const pid = stack.pop()
    const children = byParent.get(pid) || []
    for (const c of children) { pids.push(c); stack.push(c) }
  }
  return pids
}

function killProcess(proc, signal) {
  if (!proc || proc.killed) return
  if (isWindows) {
    spawn('taskkill', ['/pid', String(proc.pid), '/t', '/f'], { stdio: 'ignore' })
    return
  }
  const descendants = collectDescendants(proc.pid)
  try { process.kill(-proc.pid, signal) } catch { try { proc.kill(signal) } catch {} }
  for (const pid of descendants) { try { process.kill(pid, signal) } catch {} }
}

function killChild(signal = 'SIGINT') {
  killProcess(backend, signal)
  killProcess(child, signal)
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
  killProcess(backend, 'SIGTERM')
  process.exit(code ?? (signal ? 1 : 0))
})

backend.on('exit', (code, signal) => {
  if (shuttingDown) return
  console.error(`backend exited (code=${code}, signal=${signal})`)
  killProcess(child, 'SIGTERM')
  process.exit(code ?? (signal ? 1 : 0))
})

child.on('error', (error) => {
  console.error(error)
  killProcess(backend, 'SIGTERM')
  process.exit(1)
})

backend.on('error', (error) => {
  console.error('backend:', error)
})
