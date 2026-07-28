// @ts-nocheck
/**
 * @fileoverview Canonical browser-environment fixtures for AI-context tests.
 */

import { mock } from 'node:test'

// Mock browser environment
export const createMockWindow = () => ({
  postMessage: mock.fn(),
  addEventListener: mock.fn(),
  fetch: mock.fn(() => Promise.resolve({ ok: false })),
  performance: { now: () => Date.now() }
})

export const createMockDocument = () => ({
  activeElement: null,
  querySelectorAll: mock.fn(() => []),
  body: {}
})
