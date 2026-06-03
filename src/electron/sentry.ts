import { app } from 'electron'
import * as Sentry from '@sentry/electron/main'

let initialized = false

export function initSentry() {
  if (initialized) return
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
  if (!initialized) return
  Sentry.withScope((scope) => {
    if (context) scope.setContext('extra', context)
    Sentry.captureException(err)
  })
}
