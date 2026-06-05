import { app } from 'electron'
import { createRequire } from 'node:module'

type SentryMain = typeof import('@sentry/electron/main')

const requireSentryModule = createRequire(import.meta.url)
let initialized = false
let sentry: SentryMain | undefined
let didTryLoadSentry = false

function loadSentry() {
  if (sentry || didTryLoadSentry) return sentry
  didTryLoadSentry = true
  try {
    sentry = requireSentryModule('@sentry/electron/main') as SentryMain
  } catch (error) {
    console.warn('Sentry disabled because @sentry/electron/main could not be loaded.', error)
  }
  return sentry
}

export function initSentry() {
  if (initialized) return
  const Sentry = loadSentry()
  if (!Sentry) return
  const DEFAULT_DSN = 'https://42084c8627e67df78e0de6816e67df66@o1175778.ingest.us.sentry.io/4511502032502784'
  const dsn = process.env.SENTRY_DSN_DESKTOP || process.env.NEVERMIND_SENTRY_DSN || DEFAULT_DSN
  if (!dsn) return
  Sentry.init({
    dsn,
    release: app.getVersion(),
    environment: app.isPackaged ? 'production' : 'development',
    tracesSampleRate: 0,
  })
  initialized = true
}

export function captureException(err: unknown, context?: Record<string, unknown>) {
  if (!initialized || !sentry) return
  sentry.withScope((scope) => {
    if (context) scope.setContext('extra', context)
    sentry?.captureException(err)
  })
}
