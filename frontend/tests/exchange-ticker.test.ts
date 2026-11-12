import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import Module, { createRequire } from 'node:module'
import path from 'node:path'

import * as React from 'react'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'

import Ticker, { type TickerRate } from '../src/sections/Ticker'

// Test harness only: tsx reads the cwd tsconfig.json (no "jsx" option), so the imported
// component is compiled with the classic transform and resolves `React` from the global scope.
;(globalThis as unknown as { React: typeof React }).React = React

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the existing API tests do.
interface ModuleInternals {
  _resolveFilename: (
    request: string,
    parent?: unknown,
    isMain?: boolean,
    options?: unknown,
  ) => string
}

const moduleInternals = Module as unknown as ModuleInternals
const originalResolveFilename = moduleInternals._resolveFilename
// tsx runs this file as ESM, where __dirname is undefined; the compiled run keeps it.
const compiledSrcRoot =
  typeof __dirname === 'string'
    ? path.resolve(__dirname, '..', 'src')
    : path.resolve(process.cwd(), 'src')

// tsc emits no binary assets, so asset imports resolve to their would-be
// location under out/ and are stubbed by the require extension below.
const assetExtensions = new Set(['.jpg', '.jpeg', '.png', '.svg', '.webp'])

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  if (assetExtensions.has(path.extname(target)) && !existsSync(target)) {
    return target
  }

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

const assetRequire = createRequire(path.join(compiledSrcRoot, 'index.js'))
const originalJpgLoader = assetRequire.extensions['.jpg']
assetRequire.extensions['.jpg'] = (module, filename) => {
  module.exports = filename
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  if (originalJpgLoader) {
    assetRequire.extensions['.jpg'] = originalJpgLoader
  } else {
    delete assetRequire.extensions['.jpg']
  }
})

const HTML_ENTITIES: Record<string, string> = {
  '&amp;': '&',
  '&lt;': '<',
  '&gt;': '>',
  '&quot;': '"',
  '&#x27;': "'",
}

// React escapes '>' in text, so decode it before looking for the rate strings.
function decodeText(markup: string): string {
  return markup.replace(/&(?:amp|lt|gt|quot|#x27);/g, (entity) => HTML_ENTITIES[entity] ?? entity)
}

function renderTicker(items: TickerRate[]): string {
  return renderToStaticMarkup(createElement(MemoryRouter, null, createElement(Ticker, { items })))
}

test('ticker renders rich rate cards', () => {
  const markup = renderTicker([
    { code: 'EUR', name: 'Євро', flag: '🇪🇺', value: '51.08' },
    { code: 'USD', name: 'Долар США', flag: '🇺🇸', value: '45.50' },
  ])
  const text = decodeText(markup)

  // Every part of a rate card reaches the DOM: flag, name, code, value, unit.
  for (const part of ['Євро', 'EUR', '51.08', '🇪🇺', '₴', 'Долар США', 'USD', '45.50', '🇺🇸']) {
    assert.ok(text.includes(part), `expected the ticker markup to include ${part}:\n${markup}`)
  }

  // The accessible label carries the full sentence, not just the bare value.
  assert.ok(
    text.includes('aria-label="Євро (EUR): 51.08 гривень"'),
    `expected the ticker markup to expose the EUR rate aria-label:\n${markup}`,
  )
  assert.ok(
    text.includes('aria-label="Долар США (USD): 45.50 гривень"'),
    `expected the ticker markup to expose the USD rate aria-label:\n${markup}`,
  )

  // The old flat "UAH -> EUR = ..." strings are gone.
  assert.ok(
    !text.includes('UAH ->'),
    `expected the ticker to drop the literal "UAH ->" text:\n${markup}`,
  )
})

test('ticker renders nothing for empty rates', () => {
  assert.equal(renderTicker([]), '')
})

test('ticker renders unit after value', () => {
  const markup = renderTicker([
    { code: 'A95', name: 'А-95', flag: '⛽', value: '88.74', unit: '₴/л' },
  ])
  const text = decodeText(markup)
  const valueIndex = text.indexOf('88.74')
  const unitIndex = text.indexOf('₴/л')

  assert.ok(valueIndex !== -1, `expected the ticker markup to include 88.74:\n${markup}`)
  assert.ok(unitIndex !== -1, `expected the ticker markup to include the ₴/л unit:\n${markup}`)
  assert.ok(
    valueIndex < unitIndex,
    `expected the unit to follow the value (88.74 < ₴/л):\n${markup}`,
  )
})

test('ticker defaults unit to hryvnia sign', () => {
  const markup = renderTicker([{ code: 'EUR', name: 'Євро', flag: '🇪🇺', value: '51.08' }])
  const text = decodeText(markup)

  assert.ok(text.includes('₴'), `expected the default ₴ unit in the ticker markup:\n${markup}`)
  assert.ok(!text.includes('₴/л'), `expected no custom ₴/л unit by default:\n${markup}`)
})

test('ticker renders section label at group boundary', () => {
  const markup = renderTicker([
    { code: 'EUR', name: 'Євро', flag: '🇪🇺', value: '51.08', group: 'Курс валют' },
    { code: 'USD', name: 'Долар США', flag: '🇺🇸', value: '45.50', group: 'Курс валют' },
    { code: 'A95', name: 'А-95', flag: '⛽', value: '88.74', unit: '₴/л', group: 'Паливо' },
  ])
  const text = decodeText(markup)
  const currencyIndex = text.indexOf('Курс валют')
  const fuelIndex = text.indexOf('Паливо')

  assert.ok(currencyIndex !== -1, `expected the 'Курс валют' section label:\n${markup}`)
  assert.ok(fuelIndex !== -1, `expected the 'Паливо' section label:\n${markup}`)
  // The fuel group starts after the currency group, so its label comes later.
  assert.ok(
    currencyIndex < fuelIndex,
    `expected 'Паливо' to follow 'Курс валют' (${currencyIndex} < ${fuelIndex}):\n${markup}`,
  )
})

test('ticker renders no labels without groups', () => {
  const markup = renderTicker([{ code: 'EUR', name: 'Євро', flag: '🇪🇺', value: '51.08' }])
  const text = decodeText(markup)

  // Without any group metadata the ticker must stay label-free.
  assert.ok(!text.includes('Курс валют'), `expected no 'Курс валют' label without groups:\n${markup}`)
  assert.ok(!text.includes('Паливо'), `expected no 'Паливо' label without groups:\n${markup}`)
})
