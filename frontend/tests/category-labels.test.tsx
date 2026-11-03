import { after, afterEach, test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType, ReactNode } from 'react'

import { AuthContext } from '../src/context/auth-context'
import { validateNewsPayload } from '../src/lib/news-validation'
import type { AdminNews, Article, Category, NewsFormValues } from '../src/types'

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

const assetExtensions = new Set(['.jpg', '.jpeg', '.png', '.svg', '.webp'])

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  if (assetExtensions.has(path.extname(target)) && !existsSync(target)) {
    return target
  }

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

const originalJpgLoader = require.extensions['.jpg']
const originalSvgLoader = require.extensions['.svg']
require.extensions['.jpg'] = (module, filename) => {
  module.exports = filename
}
require.extensions['.svg'] = (module, filename) => {
  module.exports = filename
}

// ---------------------------------------------------------------------------
// The Category page reads its data inside useEffect, which the server renderer
// never runs. A fixed state keeps the real page markup observable.
// ---------------------------------------------------------------------------

interface CategoryNewsState {
  articles: Article[]
  loading: boolean
  error: string | null
  page: number
  limit: number
  hasNextPage: boolean
  retry: () => unknown
  nextPage: () => unknown
  previousPage: () => unknown
}

const categoryState: CategoryNewsState = {
  articles: [],
  loading: false,
  error: null,
  page: 1,
  limit: 20,
  hasNextPage: false,
  retry: () => undefined,
  nextPage: () => undefined,
  previousPage: () => undefined,
}

const categoryHookProxy = (): CategoryNewsState => categoryState

// ---------------------------------------------------------------------------
// The Admin page loads its rows through useEffect; supplying its own initial
// state keeps the real Admin tree rendering while making its table observable.
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

// Radix renders dialog content through a portal, and the server renderer drops
// portals. A passthrough keeps the real dialog while exposing its markup.
function PortalPassthrough({ children }: { children?: ReactNode }) {
  return <>{children}</>
}

moduleInternals._load = function loadWithTestStubs(request, parent, ...rest) {
  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''
  const importsCategoryHook =
    request === '@/hooks/use-category-news' ||
    (parentFilename.endsWith(path.join('pages', 'Category.js')) &&
      request.endsWith('use-category-news'))

  if (importsCategoryHook) {
    return { __esModule: true, default: categoryHookProxy, useCategoryNews: categoryHookProxy }
  }

  const loaded = originalLoad.call(moduleInternals, request, parent, ...rest) as Record<string, unknown>

  if (request === '@radix-ui/react-dialog') {
    return { ...loaded, Portal: PortalPassthrough }
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

  if (originalJpgLoader) {
    require.extensions['.jpg'] = originalJpgLoader
  } else {
    delete require.extensions['.jpg']
  }
  if (originalSvgLoader) {
    require.extensions['.svg'] = originalSvgLoader
  } else {
    delete require.extensions['.svg']
  }
})

afterEach(() => {
  articlesForRender = []
})

// ---------------------------------------------------------------------------
// Fixtures and helpers
// ---------------------------------------------------------------------------

const formValues: NewsFormValues = {
  category: 'Новини',
  title: 'Тестова новина',
  description: 'Короткий опис',
  text: 'Повний текст',
  language: 'uk',
  status: 'draft',
  featured: false,
  main_event: false,
  image_url: '',
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

let appPromise: Promise<ComponentType> | undefined
let adminPromise: Promise<ComponentType> | undefined
let editorPromise: Promise<ComponentType<{ onClose: () => void; onSave: (values: NewsFormValues) => void }>> | undefined

async function renderCategoryRoute(): Promise<string> {
  appPromise ??= import('../src/App').then((module) => module.default)
  const App = await appPromise

  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/sport']}>
      <App />
    </MemoryRouter>,
  )
}

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

async function renderEditor(): Promise<string> {
  editorPromise ??= import('../src/components/admin/NewsEditorDialog').then((module) => module.default)
  const NewsEditorDialog = await editorPromise

  return renderToStaticMarkup(
    <NewsEditorDialog onClose={() => {}} onSave={() => {}} />,
  )
}

function assertNoRubric(html: string, surface: string): void {
  assert.equal(
    visibleText(html).includes('рубрик'),
    false,
    `${surface}: the visible text must not say «рубрика»:\n${html}`,
  )
}

function headerCellTexts(html: string): string[] {
  return [...html.matchAll(/<th\b[^>]*>([\s\S]*?)<\/th>/gi)].map((match) => visibleText(match[1]))
}

function eyebrowAboveHeading(html: string): string {
  const match = /<div\b[^>]*>\s*([^<]*?)\s*<\/div>\s*<h1\b[^>]*>/i.exec(html)

  assert.ok(match, `expected an eyebrow element directly above the <h1> in:\n${html}`)

  return visibleText(match[1])
}

function labelTextFor(html: string, htmlFor: string): string {
  const pattern = new RegExp(`<label\\b[^>]*\\bfor="${htmlFor}"[^>]*>([\\s\\S]*?)</label>`, 'i')
  const match = pattern.exec(html)

  assert.ok(match, `expected a <label for="${htmlFor}"> in:\n${html}`)

  return visibleText(match[1])
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test('validateNewsPayload asks for «категорію» instead of «рубрику»', () => {
  const cases: Array<{ name: string; category: Category | undefined }> = [
    { name: 'missing category', category: undefined },
    { name: 'blank category', category: '' as Category },
  ]

  for (const { name, category } of cases) {
    const errors = validateNewsPayload({ ...formValues, category: category as Category })

    assert.equal(errors.category, 'Оберіть категорію.', `${name}: wrong category message`)
    assert.equal(
      errors.category?.toLocaleLowerCase('uk').includes('рубрик'),
      false,
      `${name}: the category message must not say «рубрика»`,
    )
  }
})

test('Category route labels the eyebrow above the heading «КАТЕГОРІЯ»', async () => {
  const html = await renderCategoryRoute()

  assert.equal(
    eyebrowAboveHeading(html),
    'категорія',
    `Category route: expected the eyebrow directly above <h1> to read «КАТЕГОРІЯ»:\n${html}`,
  )
  assertNoRubric(html, 'Category route')
})

test('Admin page labels the category column «КАТЕГОРІЯ»', async () => {
  const html = await renderAdmin()
  const headers = headerCellTexts(html)

  assert.ok(
    headers.includes('категорія'),
    `Admin page: expected a <th> cell reading «КАТЕГОРІЯ», got ${JSON.stringify(headers)}:\n${html}`,
  )
  assertNoRubric(html, 'Admin page')
})

test('Admin page exposes the category filter control', async () => {
  const html = await renderAdmin()
  const trigger = /<[^>]*\baria-label="Фільтр за категорією"[^>]*>/i.exec(html)

  assert.ok(
    trigger,
    `Admin page: expected the category filter trigger with aria-label «Фільтр за категорією»:\n${html}`,
  )
  assert.ok(
    trigger[0].includes('role="combobox"'),
    `Admin page: the category filter trigger must be a combobox, got ${trigger[0]}`,
  )
})

test('NewsEditorDialog labels the category field «КАТЕГОРІЯ»', async () => {
  const html = await renderEditor()

  assert.equal(
    labelTextFor(html, 'news-category'),
    'категорія',
    `NewsEditorDialog: expected <label for="news-category"> to read «КАТЕГОРІЯ»:\n${html}`,
  )
  assertNoRubric(html, 'NewsEditorDialog')
})
