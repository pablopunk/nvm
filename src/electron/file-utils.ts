import os from 'node:os'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

export const IMAGE_EXTENSIONS = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'tiff', 'tif', 'heic'])
export const VIDEO_EXTENSIONS = new Set(['mp4', 'mov', 'avi', 'mkv', 'webm', 'wmv', 'flv', 'm4v'])
export const LOCAL_FILE_PROTOCOL = 'nvm-file'
export const LOCAL_THUMB_PROTOCOL = 'nvm-thumb'

export function expandUserPath(value: string) {
  if (!value) return value
  if (value === '~') return os.homedir()
  if (value.startsWith('~/')) return path.join(os.homedir(), value.slice(2))
  return value
}

export function fileUrlForPath(filePath: string) {
  return `${LOCAL_FILE_PROTOCOL}:${pathToFileURL(filePath).href.slice('file:'.length)}`
}

export function thumbnailUrlForPath(filePath: string) {
  return `${LOCAL_THUMB_PROTOCOL}://thumb?path=${encodeURIComponent(filePath)}`
}

export function extensionForPath(filePath: string) {
  return path.extname(filePath).toLowerCase().replace(/^\./, '')
}

export function isImagePath(filePath: string) {
  return IMAGE_EXTENSIONS.has(extensionForPath(filePath))
}

export function isVideoPath(filePath: string) {
  return VIDEO_EXTENSIONS.has(extensionForPath(filePath))
}
