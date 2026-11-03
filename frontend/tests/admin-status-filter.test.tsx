import { after, afterEach, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType, ReactNode } from 'react'

import { AuthContext } from '../src/context/auth-context'
import type { AdminNews } from '../src/types'

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the other frontend suites do.
interface ModuleInternals {
  _resolveFilename: (
    request: string,
    parent?: unknown,
    isMain?: boolean,
    options?: unknown,
  ) => string
  _load: (request: string, parent?: unknown, isMain?: boolean) => unknown
}

const moduleInternals = Module as unknown as ModuleInternals
const originalResolveFilename = moduleInternals._resolveFilename
const originalLoad = moduleInternals._load
const compiledSrcRoot = path.resolve(__dirname, '..', 'src')

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

// ---------------------------------------------------------------------------
// The Admin page loads its rows through useEffect, which the server renderer
// never runs. Supplying its own initial state keeps the real Admin tree
// rendering while making its filters, table and stats observable.
// ---------------------------------------------------------------------------

let realUseState: ((initial: unknown) => unknown) | undefined
let articlesForRender: AdminNews[] = []

function adminUseState(initial: unknown) {
  if (Array.isArray(initial)) {
    return [articlesForRender, () => {}]
  }
  if (initial === true) {
    return [false, () => {}]
  }
  if (!realUseState) throw new Error('react useState was not captured for Admin')

  return realUseState(initial)
}

// A closed Radix select renders its items into a DocumentFragment through a
// portal, which the server renderer drops. Exposing that fragment keeps the
// real option list observable, the same way the category suite exposes Radix
// dialog portals. Only the select internals are patched; every other module
// keeps the real hooks.
const selectDistSuffix = path.join('@radix-ui', 'react-select', 'dist', 'index.js')

moduleInternals._load = function loadWithAdminInitialState(request, parent, ...rest) {
  const loaded = originalLoad.call(moduleInternals, request, parent, ...rest) as Record<string, unknown>
  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''

  if (request === 'react-dom') {
    return { ...loaded, createPortal: (children: ReactNode) => children }
  }

  if (request === '@radix-ui/react-portal') {
    return { ...loaded, Portal: ({ children }: { children?: ReactNode }) => children }
  }

  if (request === 'react' && parentFilename.endsWith(selectDistSuffix)) {
    const selectUseState = loaded.useState as (initial?: unknown) => [unknown, (value: unknown) => void]

    return {
      ...loaded,
      // The closed-content branch keeps its DocumentFragment in a state
      // initialised without an argument; give it a value so the portal renders.
      useState: (initial?: unknown) =>
        initial === undefined ? [{}, () => undefined] : selectUseState(initial),
    }
  }

  if (request === 'react' && parentFilename.endsWith(path.join('pages', 'Admin.js'))) {
    realUseState = loaded.useState as typeof realUseState
    return { ...loaded, useState: adminUseState }
  }

  return loaded
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  moduleInternals._load = originalLoad
})

afterEach(() => {
  articlesForRender = []
})

// ---------------------------------------------------------------------------
// Fixtures and helpers
// ---------------------------------------------------------------------------

function makeArticle(overrides: Partial<AdminNews> = {}): AdminNews {
  return {
    id: 'news-1',
    slug: 'news-1',
    category: 'Новини',
    title: 'Тестова новина',
    description: 'Короткий опис новини',
    text: 'Повний текст новини',
    language: 'uk',
    status: 'published',
    featured: false,
    main_event: false,
    image_url: '',
    views: 10,
    published_at: '2026-10-25T11:35:00Z',
    created_at: '2026-10-24T09:00:00Z',
    updated_at: '2026-10-25T11:35:00Z',
    ...overrides,
  }
}

const authValue = {
  user: { id: 'staff-1', login: 'editor', role: 'admin' as const },
  loading: false,
  login: async () => {},
  logout: async () => {},
  clearUser: () => {},
}

function visibleText(html: string): string {
  return html
    .replace(/<[^>]*>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .toLocaleLowerCase('uk')
}

function headerCellTexts(html: string): string[] {
  return [...html.matchAll(/<th\b[^>]*>([\s\S]*?)<\/th>/gi)].map((match) => visibleText(match[1]))
}

function sectionByLabel(html: string, label: string): string | undefined {
  const pattern = new RegExp(`<section\\b[^>]*aria-label="${label}"[^>]*>([\\s\\S]*?)</section>`)
  return html.match(pattern)?.[1]
}

let adminPromise: Promise<ComponentType> | undefined

async function renderAdmin(): Promise<string> {
  adminPromise ??= import('../src/pages/Admin').then((module) => module.default)
  const Admin = await adminPromise
  articlesForRender = [makeArticle()]

  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/admin']}>
      <AuthContext.Provider value={authValue}>
        <Admin />
      </AuthContext.Provider>
    </MemoryRouter>,
  )
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test('Admin page no longer renders the status filter control', async () => {
  const html = await renderAdmin()
  const statusFilter = /<[^>]*\baria-label="Фільтр за статусом"[^>]*>/i.exec(html)

  assert.equal(
    Boolean(statusFilter),
    false,
    `Admin page: the status filter must be removed, but found ${statusFilter?.[0]}`,
  )
})

test('Admin page no longer offers the «Усі статуси» option', async () => {
  const html = await renderAdmin()
  const text = visibleText(html)

  assert.equal(
    text.includes('усі статуси'),
    false,
    `Admin page: the visible text must not offer «Усі статуси», got:\n${text}`,
  )
})

test('Admin page keeps the category filter control', async () => {
  const html = await renderAdmin()
  const trigger = /<[^>]*\baria-label="Фільтр за категорією"[^>]*>/i.exec(html)

  assert.ok(
    trigger,
    `Admin page: expected the category filter trigger with aria-label «Фільтр за категорією»:\n${html}`,
  )
})

test('Admin page keeps the «СТАТУС» table column', async () => {
  const html = await renderAdmin()
  const headers = headerCellTexts(html)

  assert.ok(
    headers.includes('статус'),
    `Admin page: expected the <th> «СТАТУС» to remain, got ${JSON.stringify(headers)}:\n${html}`,
  )
})

test('Admin page keeps the «Опубліковано на сторінці» stat', async () => {
  const html = await renderAdmin()
  const stats = sectionByLabel(html, 'Статистика')

  assert.ok(stats, `Admin page: expected the stats section in:\n${html}`)
  assert.ok(
    visibleText(stats).includes('опубліковано на сторінці'),
    `Admin page: expected the «Опубліковано на сторінці» stat to remain in:\n${stats}`,
  )
})
