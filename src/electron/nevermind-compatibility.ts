import * as logger from './logger'
import { nevermindDesktopHeaders } from './nevermind-api'

export type NevermindCompatibilityManifest = {
  client?: {
    compatible?: boolean
    unsupportedReason?: string | null
  }
  desktop?: {
    minimumSupportedVersion?: string
    latestVersion?: string | null
    updateUrl?: string
  }
}

export class NevermindCompatibilityError extends Error {
  updateUrl?: string
  minimumSupportedVersion?: string
  latestVersion?: string | null
  unsupportedReason?: string | null

  constructor(manifest: NevermindCompatibilityManifest) {
    const minimum = manifest.desktop?.minimumSupportedVersion
    super(minimum ? `This Nevermind version is no longer supported. Please update to ${minimum} or newer.` : 'This Nevermind version is no longer supported. Please update Nevermind.')
    this.name = 'NevermindCompatibilityError'
    this.updateUrl = manifest.desktop?.updateUrl
    this.minimumSupportedVersion = minimum
    this.latestVersion = manifest.desktop?.latestVersion
    this.unsupportedReason = manifest.client?.unsupportedReason
  }
}

export async function checkNevermindCompatibility(baseUrl: string) {
  const trimmed = baseUrl.replace(/\/$/, '')
  let res: Response
  try {
    res = await fetch(`${trimmed}/api/compatibility`, { headers: nevermindDesktopHeaders() })
  } catch (error) {
    logger.warn('nevermind.compatibility.fetch.failed', error as Error)
    return null
  }
  if (res.status === 404) return null
  if (!res.ok) {
    logger.warn('nevermind.compatibility.unavailable', { status: res.status })
    return null
  }
  const manifest = (await res.json().catch(() => null)) as NevermindCompatibilityManifest | null
  if (!manifest) return null
  if (manifest.client?.compatible === false) throw new NevermindCompatibilityError(manifest)
  return manifest
}
