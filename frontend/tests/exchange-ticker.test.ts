import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import Module, { createRequire } from 'node:module'
import path from 'node:path'

import * as React from 'react'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'

import Ticker from '../src/sections/Ticker'

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

function renderTicker(items: string[]): string {
  return renderToStaticMarkup(createElement(MemoryRouter, null, createElement(Ticker, { items })))
}

test('ticker renders each exchange rate string', () => {
  const markup = renderTicker(['UAH -> EUR = 50.48', 'UAH -> USD = 45.32'])
  const text = decodeText(markup)

  for (const rate of ['UAH -> EUR = 50.48', 'UAH -> USD = 45.32']) {
    assert.ok(text.includes(rate), `expected the ticker markup to include ${rate}:\n${markup}`)
  }
})

test('ticker renders nothing for empty rates', () => {
  assert.equal(renderTicker([]), '')
})
