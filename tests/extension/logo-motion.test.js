// @ts-nocheck
/**
 * @fileoverview logo-motion.test.js — Tests popup logo asset contracts.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')

let logoEl
let logoListeners
let importCounter = 0

function resetPopupLogoDom() {
  logoListeners = {}
  logoEl = {
    src: '',
    addEventListener: mock.fn((type, handler) => {
      logoListeners[type] = handler
    })
  }

  globalThis.document = {
    querySelector: mock.fn((selector) => {
      if (selector === '.logo') return logoEl
      return null
    })
  }

  globalThis.chrome = {
    runtime: {
      getURL: mock.fn((assetPath) => `chrome-extension://test/${assetPath}`)
    }
  }
}

describe('popup logo motion', () => {
  beforeEach(() => {
    mock.reset()
    resetPopupLogoDom()
  })

  test('keeps icon.svg in the popup without hover swapping', async () => {
    const { initPopupLogoMotion } = await import(`../../extension/popup/logo-motion.js?v=${++importCounter}`)

    initPopupLogoMotion()

    assert.ok(String(logoEl.src).includes('icons/icon.svg'), 'popup logo should start on idle icon.svg')
    assert.equal(logoListeners.mouseenter, undefined, 'popup logo should not install hover swap handlers')
    assert.equal(logoListeners.mouseleave, undefined, 'popup logo should not install hover swap handlers')
  })
})

describe('logo asset contracts', () => {
  test('manifest uses Kaboom branding for the extension shell', () => {
    const manifestJson = fs.readFileSync(path.join(REPO_ROOT, 'extension/manifest.json'), 'utf8')
    const manifest = JSON.parse(manifestJson)
    assert.equal(manifest.name, 'KaBOOM! Devtools')
    assert.equal(manifest.action.default_title, 'KaBOOM!: Agentic Browser Devtools')
  })

  test('popup html uses icon.svg as the idle asset', () => {
    const popupHtml = fs.readFileSync(path.join(REPO_ROOT, 'extension/popup.html'), 'utf8')
    assert.match(popupHtml, /src="icons\/icon\.svg"/)
  })

  test('icon.svg restores the original flame mark', () => {
    const iconSvg = fs.readFileSync(path.join(REPO_ROOT, 'extension/icons/icon.svg'), 'utf8')
    assert.match(iconSvg, /linearGradient id="flame"/)
    assert.match(iconSvg, /linearGradient id="innerFlame"/)
    assert.match(iconSvg, /Outer flame/)
    assert.doesNotMatch(iconSvg, /strum-path|harmonic-osc|energy-speed/)
  })

  test('logo-animated.svg also uses the flame mark instead of the string logo', () => {
    const animatedSvg = fs.readFileSync(path.join(REPO_ROOT, 'extension/icons/logo-animated.svg'), 'utf8')
    assert.match(animatedSvg, /linearGradient id="flame"/)
    assert.match(animatedSvg, /linearGradient id="innerFlame"/)
    assert.doesNotMatch(animatedSvg, /strum-path|ghost-1|harmonic-osc|energy-speed/)
  })
})
