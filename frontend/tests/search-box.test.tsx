import { test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

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
const compiledSrcRoot = path.resolve(__dirname, '..', 'src')

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

interface SearchBoxProps {
  autoFocus?: boolean
  onNavigate?: () => void
}

async function loadSearchBox(): Promise<ComponentType<SearchBoxProps>> {
  // Loaded dynamically so the alias hook above is installed before Node
  // resolves SearchBox's '@/api/news' and '@/lib/utils' imports.
  const searchBoxModule = (await import('../src/components/SearchBox')) as {
    default: ComponentType<SearchBoxProps>
  }

  return searchBoxModule.default
}

test('SearchBox renders a search input with placeholder Пошук новин… and no listbox when empty', async () => {
  const SearchBox = await loadSearchBox()

  const html = renderToStaticMarkup(
    <MemoryRouter>
      <SearchBox />
    </MemoryRouter>,
  )

  assert.ok(html.includes('type="search"'), `expected a search input in:\n${html}`)
  assert.ok(
    html.includes('placeholder="Пошук новин…"'),
    `expected the search placeholder in:\n${html}`,
  )
  assert.ok(
    !html.includes('role="listbox"'),
    `expected no suggestions listbox while the query is empty, got:\n${html}`,
  )
})
