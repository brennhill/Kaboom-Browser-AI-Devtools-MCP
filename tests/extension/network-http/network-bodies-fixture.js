// @ts-nocheck
/**
 * @fileoverview Canonical response and Headers fixtures for network-body unit tests.
 */

export const createMockResponse = (options = {}) => ({
  ok: options.ok !== undefined ? options.ok : true,
  status: options.status || 200,
  statusText: options.statusText || 'OK',
  headers: new Map([
    ['content-type', options.contentType || 'application/json'],
    ...(options.headers || []),
  ]),
  clone() {
    return {
      ...this,
      text: () => Promise.resolve(options.body || '{}'),
      blob: () =>
        Promise.resolve({
          size: (options.body || '{}').length,
          type: options.contentType || 'application/json',
        }),
    }
  },
})

export class MockHeaders {
  constructor(init) {
    this._map = new Map(Object.entries(init || {}))
  }

  get(name) {
    return this._map.get(name.toLowerCase()) || null
  }

  entries() {
    return this._map.entries()
  }

  forEach(fn) {
    this._map.forEach((value, key) => fn(value, key))
  }
}
