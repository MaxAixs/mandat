import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

import { AuthContext } from '../src/context/auth-context'
import type { AdminNews } from '../src/types'

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

// Admin loads its rows from the API inside useEffect, which the server renderer
// never runs. Supplying the component's own initial state keeps the real Admin
// tree rendering while making its table observable; only Admin's `react`
// import is patched, so every other module keeps the real hooks.
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

moduleInternals._load = function loadWithAdminInitialState(request, parent, ...rest) {
  const loaded = originalLoad.call(moduleInternals, request, parent, ...rest) as Record<string, unknown>
  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''

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

const authValue = {
  user: { id: 'staff-1', login: 'editor', role: 'admin' as const },
  loading: false,
  login: async () => {},
  logout: async () => {},
  clearUser: () => {},
}

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

let adminPromise: Promise<ComponentType> | undefined

async function loadAdmin(): Promise<ComponentType> {
  adminPromise ??= import('../src/pages/Admin').then((module) => module.default)
  return adminPromise
}

async function renderAdmin(articles: AdminNews[]): Promise<string> {
  const Admin = await loadAdmin()
  articlesForRender = articles

  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/admin']}>
      <AuthContext.Provider value={authValue}>
        <Admin />
      </AuthContext.Provider>
    </MemoryRouter>,
  )
}

function sectionByLabel(html: string, label: string): string | undefined {
  const pattern = new RegExp(`<section\\b[^>]*aria-label="${label}"[^>]*>([\\s\\S]*?)</section>`)
  return html.match(pattern)?.[1]
}

function tableRows(html: string): string[] {
  return Array.from(html.matchAll(/<tr\b[^>]*>([\s\S]*?)<\/tr>/g), (match) => match[1])
}

function rowForTitle(html: string, title: string): string | undefined {
  return tableRows(html).find((row) => row.includes(title))
}

function firstCell(row: string): string {
  const match = row.match(/<td\b[^>]*>([\s\S]*?)<\/td>/)
  assert.ok(match, `expected a title cell in:\n${row}`)
  return match[1]
}

function titleCellFor(html: string, title: string): string {
  const row = rowForTitle(html, title)
  assert.ok(row, `expected a table row for «${title}» in:\n${html}`)

  return firstCell(row)
}

function visibleText(html: string): string {
  return html
    .replace(/<[^>]*>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

test('Admin renders the stats without a «Чернетки на сторінці» card', async () => {
  const html = await renderAdmin([
    makeArticle({ id: 'published', slug: 'published', status: 'published' }),
    makeArticle({ id: 'draft', slug: 'draft', status: 'draft' }),
  ])
  const stats = sectionByLabel(html, 'Статистика')

  assert.ok(stats, `expected the stats section in:\n${html}`)
  assert.ok(
    stats.includes('Новин на сторінці'),
    `expected the «Новин на сторінці» stat to remain in:\n${stats}`,
  )
  assert.ok(
    stats.includes('Опубліковано на сторінці'),
    `expected the «Опубліковано на сторінці» stat to remain in:\n${stats}`,
  )
  assert.ok(
    !stats.includes('Чернетки на сторінці'),
    `expected no «Чернетки на сторінці» stat in:\n${stats}`,
  )
})

test('Admin shows a «Подія дня» badge by the title only for main_event articles', async () => {
  const html = await renderAdmin([
    makeArticle({ id: 'main', slug: 'main', title: 'Головна подія', main_event: true }),
    makeArticle({ id: 'regular', slug: 'regular', title: 'Звичайна новина', main_event: false }),
  ])
  const mainCell = titleCellFor(html, 'Головна подія')
  const regularCell = titleCellFor(html, 'Звичайна новина')

  assert.ok(
    visibleText(mainCell).includes('Подія дня'),
    `expected the «Подія дня» badge next to the main-event title in:\n${mainCell}`,
  )
  assert.ok(
    !visibleText(regularCell).includes('Подія дня'),
    `expected no «Подія дня» badge next to a regular title in:\n${regularCell}`,
  )
})

test('Admin keeps the featured Star independent from the main-event badge', async () => {
  const cases: Array<{
    name: string
    article: AdminNews
    expectBadge: boolean
    expectStar: boolean
  }> = [
    {
      name: 'featured article without main event keeps only the Star',
      article: makeArticle({ title: 'Лише на головній', featured: true, main_event: false }),
      expectBadge: false,
      expectStar: true,
    },
    {
      name: 'main event without featured shows only the badge',
      article: makeArticle({ title: 'Лише подія дня', featured: false, main_event: true }),
      expectBadge: true,
      expectStar: false,
    },
    {
      name: 'article that is both featured and main event shows both markers',
      article: makeArticle({ title: 'І головна, і подія', featured: true, main_event: true }),
      expectBadge: true,
      expectStar: true,
    },
  ]

  for (const { name, article, expectBadge, expectStar } of cases) {
    const html = await renderAdmin([article])
    const cell = titleCellFor(html, article.title)
    const hasBadge = visibleText(cell).includes('Подія дня')
    const hasStar = cell.includes('lucide-star')

    assert.equal(hasBadge, expectBadge, `${name}: unexpected badge state in:\n${cell}`)
    assert.equal(hasStar, expectStar, `${name}: unexpected Star state in:\n${cell}`)
  }
})

test('Admin keeps the main-event badge visible for a draft article', async () => {
  const html = await renderAdmin([
    makeArticle({
      title: 'Чернетка події дня',
      status: 'draft',
      main_event: true,
    }),
  ])
  const cell = titleCellFor(html, 'Чернетка події дня')

  assert.ok(
    visibleText(cell).includes('Подія дня'),
    `expected the badge to follow main_event regardless of status in:\n${cell}`,
  )
})
