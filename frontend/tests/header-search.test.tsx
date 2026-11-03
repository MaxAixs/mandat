import { test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the other frontend tests do.
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
const compiledSrcRoot = path.resolve(__dirname, '..', 'src')

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

async function loadHeader(): Promise<ComponentType> {
  // Loaded dynamically so the alias hook above is installed before Node
  // resolves Header's '@/components/SearchBox' and '@/constants' imports.
  const headerModule = (await import('../src/sections/Header')) as {
    default: ComponentType
  }

  return headerModule.default
}

test('Header keeps live search on desktop and mobile without legacy #news navigation', async () => {
  const Header = await loadHeader()

  const html = renderToStaticMarkup(
    <MemoryRouter>
      <Header />
    </MemoryRouter>,
  )

  assert.ok(
    html.includes('placeholder="Пошук новин…"'),
    `expected the desktop search field in the header markup:\n${html}`,
  )
  assert.ok(
    html.includes('aria-label="Пошук новин"'),
    `expected the mobile search toggle in the header markup:\n${html}`,
  )
  assert.ok(
    !html.includes('href="#news"'),
    `expected the header navigation to keep real routes, not the legacy #news anchor:\n${html}`,
  )
})
