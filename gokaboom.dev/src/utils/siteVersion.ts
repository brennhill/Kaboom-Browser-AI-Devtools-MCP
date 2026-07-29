import { readFileSync } from 'node:fs'

function loadVersionFromRepoRoot() {
  try {
    const raw = readFileSync(new URL('../../../VERSION', import.meta.url), 'utf8').trim()
    return raw.length > 0 ? raw : '0.0.0'
  } catch {
    return '0.0.0'
  }
}

const versionRaw = loadVersionFromRepoRoot().replace(/^v/i, '')

export const siteVersion = versionRaw
export const siteVersionLabel = `v${versionRaw}`
export const siteReleaseChannel = 'latest'
