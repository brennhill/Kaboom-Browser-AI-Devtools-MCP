// upload-handler-execute.test.js — executeUpload error-path coverage.
// Verifies upload commands return async error results when script injection fails,
// instead of being left pending forever.
//
// Run: node --test extension/background/__tests__/upload-handler-execute.test.js

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

const mockExecuteScript = mock.fn()
const mockFetch = mock.fn()

globalThis.chrome = {
  scripting: { executeScript: mockExecuteScript }
}
globalThis.fetch = mockFetch

import { executeUpload } from '../upload-handler.js'

describe('executeUpload', () => {
  beforeEach(() => {
    mockExecuteScript.mock.resetCalls()
    mockFetch.mock.resetCalls()
  })

  test('sends async error result when executeScript fails after Stage 1 read', async () => {
    mockFetch.mock.mockImplementation(async (url) => {
      if (typeof url === 'string' && url.includes('/api/file/read')) {
        return {
          ok: true,
          status: 200,
          async json() {
            return {
              success: true,
              data_base64: Buffer.from('hello').toString('base64'),
              file_name: 'upload.txt',
              mime_type: 'text/plain'
            }
          }
        }
      }
      throw new Error(`unexpected fetch url: ${url}`)
    })

    mockExecuteScript.mock.mockImplementation(async () => {
      throw new Error(
        'The page keeping the extension port is moved into back/forward cache, so the message channel is closed.'
      )
    })

    const asyncResults = []
    const sendAsyncResult = (_syncClient, queryId, correlationId, status, result, error) => {
      asyncResults.push({ queryId, correlationId, status, result, error })
    }

    await executeUpload(
      {
        id: 'q-upload-1',
        correlation_id: 'upload_corr_1',
        params: JSON.stringify({
          selector: '#file-input',
          file_path: '/tmp/upload.txt',
          file_name: 'upload.txt',
          mime_type: 'text/plain'
        })
      },
      1,
      {},
      sendAsyncResult,
      () => {}
    )

    assert.strictEqual(asyncResults.length, 1, 'expected one async result to be sent')
    assert.strictEqual(asyncResults[0].status, 'error')
    assert.ok(
      typeof asyncResults[0].error === 'string' && asyncResults[0].error.includes('message channel is closed'),
      `expected executeScript channel-closed error, got: ${asyncResults[0].error}`
    )
  })
})
