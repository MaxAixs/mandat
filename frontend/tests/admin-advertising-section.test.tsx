import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

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
// rendering while making its filters observable. A closed Radix select
// renders its items into a DocumentFragment through a portal, which the
// server renderer drops; exposing that fragment keeps the real option list
// observable. Only the select internals and Admin's `react` import are
// patched, so every other module keeps the real hooks.
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

const selectDistSuffix = path.join('@radix-ui', 'react-select', 'dist', 'index.js')

moduleInternals._load = function loadWithAdminInitialState(request, parent, ...rest) {
  const loaded = originalLoad.call(moduleInternals, request, parent, ...rest) as Record<string, unknown>
  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''

  if (request === 'react-dom') {
    return { ...loaded, createPortal: (children: React.ReactNode) => children }
  }

  if (request === '@radix-ui/react-portal') {
    return { ...loaded, Portal: ({ children }: { children?: React.ReactNode }) => children }
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

// The subjects load lazily: compiled src modules keep the '@/...' alias, so
// they must be required after the resolver above is installed (static imports
// would run first and fail on the alias).
type SiteConstants = typeof import('../src/constants')
type AdvertisingConstants = typeof import('../src/constants/advertising')

let CATEGORIES!: SiteConstants['CATEGORIES']
let ADVERTISING_ADMIN_SECTION!: string
let Admin!: ComponentType
let AdFormatsPanel!: ComponentType

before(async () => {
  const siteConstants: SiteConstants = await import('../src/constants')
  const advertisingConstants: AdvertisingConstants = await import('../src/constants/advertising')
  CATEGORIES = siteConstants.CATEGORIES
  ADVERTISING_ADMIN_SECTION = advertisingConstants.ADVERTISING_ADMIN_SECTION
  ;({ default: Admin } = await import('../src/pages/Admin'))
  ;({ default: AdFormatsPanel } = await import('../src/components/admin/AdFormatsPanel'))
})

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
    .replace(/&#x27;/g, "'")
    .replace(/&quot;/g, '"')
    .replace(/&amp;/g, '&')
    .replace(/\s+/g, ' ')
    .trim()
    .toLocaleLowerCase('uk')
}

async function renderAdmin(): Promise<string> {
  articlesForRender = [makeArticle()]

  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/admin']}>
      <AuthContext.Provider value={authValue}>
        <Admin />
      </AuthContext.Provider>
    </MemoryRouter>,
  )
}

function renderPanel(): string {
  return renderToStaticMarkup(<AdFormatsPanel />)
}

/**
 * Radix SelectItem renders role="option". Reading the option list (instead of
 * the whole page text) keeps the assertion scoped to the category filter: the
 * page itself may mention advertising elsewhere (e.g. «з рекламою»).
 */
function selectOptionLabels(html: string): string[] {
  const matches = html.matchAll(/<div[^>]*\brole="option"[^>]*>([\s\S]*?)<\/div>/g)

  return Array.from(matches, (match) => visibleText(match[1]))
}

function sectionNavs(html: string): string[] {
  const matches = html.matchAll(/<nav[^>]*aria-label="Розділи адмінпанелі"[^>]*>([\s\S]*?)<\/nav>/g)

  return Array.from(matches, (match) => match[1])
}

interface NavButton {
  openingTag: string
  label: string
}

function navButtons(navHtml: string): NavButton[] {
  const matches = navHtml.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g)

  return Array.from(matches, (match) => ({
    openingTag: match[1],
    label: visibleText(match[2]),
  }))
}

test('Admin page category filter offers only news rubrics without «Реклама»', async () => {
  const html = await renderAdmin()
  const options = selectOptionLabels(html)

  assert.ok(
    options.length > 0,
    `Admin page: expected the category filter to render its options in:\n${html}`,
  )
  assert.ok(
    !options.some((label) =>
      label.includes(ADVERTISING_ADMIN_SECTION.toLocaleLowerCase('uk')),
    ),
    `Admin page: expected no «${ADVERTISING_ADMIN_SECTION}» option in the category filter, got [${options.join(', ')}] in:\n${html}`,
  )
  assert.ok(
    !html.includes('select-separator'),
    `Admin page: expected no separator in the category filter in:\n${html}`,
  )
  assert.ok(
    !html.includes('role="separator"'),
    `Admin page: expected no separator role in the category filter in:\n${html}`,
  )
  assert.equal(
    options.length,
    CATEGORIES.length,
    `Admin page: expected exactly the news rubrics in the filter, got [${options.join(', ')}] in:\n${html}`,
  )
})

test('Admin page category filter still offers every news rubric', async () => {
  const html = await renderAdmin()
  const options = selectOptionLabels(html)

  for (const category of CATEGORIES) {
    assert.ok(
      options.includes(category.toLocaleLowerCase('uk')),
      `Admin page: expected the category filter to still offer «${category}», got [${options.join(', ')}] in:\n${html}`,
    )
  }
})

test('Admin page sections nav switches between news and advertising management', async () => {
  const html = await renderAdmin()
  const navs = sectionNavs(html)

  assert.ok(
    navs.length >= 1,
    `Admin page: expected a <nav aria-label="Розділи адмінпанелі"> in:\n${html}`,
  )

  for (const nav of navs) {
    const buttons = navButtons(nav)
    const newsButton = buttons.find((button) =>
      button.label.includes('керування новинами'),
    )
    const adsButton = buttons.find((button) =>
      button.label.includes('формати реклами'),
    )

    assert.ok(
      newsButton,
      `Admin page: expected a «Керування новинами» button in the sections nav in:\n${nav}`,
    )
    assert.ok(
      adsButton,
      `Admin page: expected a «Формати реклами» button in the sections nav in:\n${nav}`,
    )
    assert.ok(
      newsButton.openingTag.includes('type="button"'),
      `Admin page: expected the news section item to be <button type="button"> in:\n${nav}`,
    )
    assert.ok(
      adsButton.openingTag.includes('type="button"'),
      `Admin page: expected the advertising section item to be <button type="button"> in:\n${nav}`,
    )
    assert.ok(
      newsButton.openingTag.includes('aria-current="page"'),
      `Admin page: expected the news button to carry aria-current="page" on the initial render in:\n${nav}`,
    )
    assert.ok(
      !adsButton.openingTag.includes('aria-current'),
      `Admin page: expected the advertising button to carry no aria-current on the initial render in:\n${nav}`,
    )
  }
})

test('Admin initial render shows the news management view', async () => {
  const html = await renderAdmin()

  assert.ok(
    html.includes('Керування новинами'),
    `Admin page: expected the «Керування новинами» heading on the initial render in:\n${html}`,
  )
  assert.ok(
    html.includes('Додати новину'),
    `Admin page: expected the «Додати новину» button on the initial render in:\n${html}`,
  )
})

test('AdFormatsPanel shows the advertising management heading and add button', () => {
  const html = renderPanel()

  assert.ok(html.includes('Керування рекламою'), `expected the «Керування рекламою» title in:\n${html}`)
  assert.ok(html.includes('Додати формат'), `expected the «Додати формат» button in:\n${html}`)
})

test('AdFormatsPanel shows the loading status on the first render', () => {
  const html = renderPanel()

  assert.ok(html.includes('role="status"'), `expected a loading status region in:\n${html}`)
  assert.ok(
    html.includes('Завантаження форматів'),
    `expected the «Завантаження форматів...» text on the first render in:\n${html}`,
  )
})
