// @ts-nocheck
/**
 * @fileoverview data-table.test.js - Regression tests for structured table extraction.
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

class MockCell {
  constructor(tagName, text) {
    this.tagName = tagName
    this.textContent = text
  }
}

class MockRow {
  constructor(cells) {
    this._cells = cells
  }

  querySelectorAll(selector) {
    if (selector === 'th,td') return this._cells
    return []
  }
}

class MockTableElement {}

function makeTable({
  id = '',
  className = '',
  caption = '',
  headerCells = [],
  rows = [],
  name = ''
}) {
  const table = {
    id,
    className,
    caption: caption ? { textContent: caption } : null,
    _name: name,
    _headerCells: headerCells.map((text) => new MockCell('TH', text)),
    _rows: rows.map((row) =>
      new MockRow(row.map((cell, idx) => new MockCell(idx === 0 && headerCells.length === 0 ? 'TH' : 'TD', cell)))
    ),
    getAttribute(attr) {
      if (attr === 'name') return this._name || null
      return null
    },
    querySelectorAll(selector) {
      if (selector === 'thead th') return this._headerCells
      if (selector === 'tbody tr, tr') return this._rows
      return []
    },
    querySelector(selector) {
      if (selector === 'tr') return this._rows[0] || null
      return null
    }
  }

  Object.setPrototypeOf(table, MockTableElement.prototype)
  return table
}

describe('extractDataTables', () => {
  let extractDataTables
  let reportTable
  let metricsTable

  beforeEach(async () => {
    ;({ extractDataTables } = await import('../../../extension/inject/data-table.js'))
    globalThis.HTMLTableElement = MockTableElement

    reportTable = makeTable({
      id: 'report',
      caption: 'Monthly Report',
      headerCells: ['Name', 'Role'],
      rows: [
        ['Alice', 'Admin'],
        ['Bob', 'Editor']
      ]
    })

    metricsTable = makeTable({
      className: 'metrics',
      headerCells: ['Metric', 'Value', 'Unit'],
      rows: [
        ['Latency', '120', 'ms'],
        ['Errors', '0.2', '%']
      ]
    })

    globalThis.document = {
      querySelectorAll: (selector) => {
        if (selector === 'table' || selector === 'table.report') return [reportTable]
        if (selector === 'table.metrics') return [metricsTable]
        if (selector === 'table.all') return [reportTable, metricsTable]
        return []
      }
    }
  })

  test('extracts headers, rows, and caption from table elements', () => {
    const result = extractDataTables({ selector: 'table' })

    assert.strictEqual(result.count, 1)
    assert.strictEqual(result.tables.length, 1)
    assert.strictEqual(result.tables[0].selector, 'table#report')
    assert.strictEqual(result.tables[0].caption, 'Monthly Report')
    assert.deepStrictEqual(result.tables[0].headers, ['Name', 'Role'])
    assert.deepStrictEqual(result.tables[0].rows, [
      { Name: 'Alice', Role: 'Admin' },
      { Name: 'Bob', Role: 'Editor' }
    ])
  })

  test('honors max_rows and max_cols limits', () => {
    const result = extractDataTables({
      selector: 'table.metrics',
      max_rows: 1,
      max_cols: 2
    })

    assert.strictEqual(result.count, 1)
    assert.deepStrictEqual(result.tables[0].headers, ['Metric', 'Value'])
    assert.strictEqual(result.tables[0].row_count, 1)
    assert.deepStrictEqual(result.tables[0].rows, [{ Metric: 'Latency', Value: '120' }])
  })
})

