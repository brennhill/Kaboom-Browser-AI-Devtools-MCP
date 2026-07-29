// @ts-nocheck
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

describe('version check branding', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.chrome = {
      runtime: {
        getManifest: () => ({ version: '1.0.0' })
      },
      action: {
        setBadgeText: mock.fn(),
        setBadgeBackgroundColor: mock.fn(),
        setTitle: mock.fn()
      }
    }
  })

  test('update badge uses Kaboom copy and Kaboom repo URL', async () => {
    const versionCheck = await import('../../../extension/background/sync/version-check.js')

    versionCheck.resetVersionCheck()
    versionCheck.updateVersionFromHealth({ version: '1.0.0', available_version: '1.1.0' })

    const title = globalThis.chrome.action.setTitle.mock.calls.at(-1).arguments[0].title
    assert.match(title, /KaBOOM!: New version available \(1.1.0\)/)
    assert.strictEqual(
      versionCheck.getUpdateInfo().downloadUrl,
      'https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/latest'
    )
  })
})
