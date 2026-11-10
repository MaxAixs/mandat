import { after, afterEach, test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType, ReactElement, ReactNode } from 'react'

import { NAV_LINKS, SITE_NAME } from '../src/constants'
import type { Article, Category } from '../src/types'

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the other frontend tests do.
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

// tsc emits no binary assets, so asset imports resolve to their compiled path
// and are stubbed by the require extensions below (Hero and Telegram use them).
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
// The category page reads its data from useCategoryNews inside useEffect. The
// server renderer never runs effects, so the page tests inject the hook state
// that a browser would obtain, while every presentational component (Header,
// Footer, NewsCard, the page markup) stays the real implementation.
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

type CategoryHook = (category: Category) => CategoryNewsState

function unavailableHook(category: Category): CategoryNewsState {
  throw new Error(
    `CategoryPage rendered «${category}» without a configured useCategoryNews test state`,
  )
}

let currentHook: CategoryHook = unavailableHook
const hookCalls: Category[] = []

const hookProxy = (category: Category): CategoryNewsState => currentHook(category)

// The category page may import the hook through the '@/...' alias or through a
// relative path; both resolve to the same test proxy. The proxy is stable, so
// switching the state between renders also works after Category.js was loaded.
moduleInternals._load = function loadWithCategoryHook(request, parent, ...rest) {
  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''
  const importsCategoryHook =
    request === '@/hooks/use-category-news' ||
    (parentFilename.endsWith(path.join('pages', 'Category.js')) &&
      request.endsWith('use-category-news'))

  if (importsCategoryHook) {
    return { __esModule: true, default: hookProxy, useCategoryNews: hookProxy }
  }

  const loaded = originalLoad.call(moduleInternals, request, parent, ...rest) as Record<string, unknown>

  // Record the elements the page (and its children) create so the pagination
  // and retry callbacks can be invoked without a DOM test renderer.
  if (request === 'react/jsx-runtime') {
    const runtime = loaded as unknown as {
      jsx: JsxFactory
      jsxs: JsxFactory
    }

    return {
      ...runtime,
      jsx: recordElements(runtime.jsx),
      jsxs: recordElements(runtime.jsxs),
    }
  }

  return loaded
}

type JsxFactory = (type: unknown, props: unknown, key?: unknown) => ReactElement

let capturedElements: ReactElement[] = []

function recordElements(create: JsxFactory): JsxFactory {
  return (type, props, key) => {
    const element = create(type, props, key)
    capturedElements.push(element)

    return element
  }
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
  currentHook = unavailableHook
  hookCalls.length = 0
  capturedElements = []
})

// ---------------------------------------------------------------------------
// Fixtures and helpers
// ---------------------------------------------------------------------------

type ArticleWithMainEvent = Article & { mainEvent?: boolean }

function makeArticle(
  category: Category,
  overrides: Partial<ArticleWithMainEvent> = {},
): ArticleWithMainEvent {
  return {
    id: 'test-slug',
    category,
    title: 'Тестовий заголовок новини',
    excerpt: 'Короткий опис новини для картки',
    views: '12,4\u00A0тис.',
    publishedAt: '25.10.2026, 14:35',
    featured: false,
    mainEvent: false,
    imageUrl: 'https://cdn.example.com/news-card.jpg',
    ...overrides,
  }
}

function makeArticles(
  count: number,
  category: Category,
  prefix: string,
): Article[] {
  return Array.from({ length: count }, (_, index) =>
    makeArticle(category, {
      id: `${prefix}-${index + 1}`,
      title: `${prefix} ${index + 1}`,
    }),
  )
}

interface FakeHookOptions extends Partial<CategoryNewsState> {
  onRetry?: () => void
  onNextPage?: () => void
  onPreviousPage?: () => void
}

function installFakeHook(options: FakeHookOptions = {}): void {
  const {
    onRetry = () => undefined,
    onNextPage = () => undefined,
    onPreviousPage = () => undefined,
    ...overrides
  } = options
  const state: CategoryNewsState = {
    articles: [],
    loading: false,
    error: null,
    page: 1,
    limit: 20,
    hasNextPage: false,
    retry: onRetry,
    nextPage: onNextPage,
    previousPage: onPreviousPage,
    ...overrides,
  }

  hookCalls.length = 0
  currentHook = (category) => {
    hookCalls.push(category)

    return state
  }
}

let appPromise: Promise<ComponentType> | undefined

async function loadApp(): Promise<ComponentType> {
  appPromise ??= import('../src/App').then((module) => {
    assert.equal(typeof module.default, 'function', 'src/App.tsx must default-export the router')

    return module.default
  })

  try {
    return await appPromise
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    assert.fail(`src/App.tsx must render the category routes, but importing it failed: ${reason}`)
  }
}

async function renderRoute(route: string): Promise<string> {
  const App = await loadApp()
  capturedElements = []

  return renderToStaticMarkup(
    <MemoryRouter initialEntries={[route]}>
      <App />
    </MemoryRouter>,
  )
}

function attribute(attributes: string, name: string): string | undefined {
  const match = attributes.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

  return match?.[1]
}

function classTokens(attributes: string): string[] {
  return (attribute(attributes, 'class') ?? '').split(/\s+/).filter(Boolean)
}

interface AnchorElement {
  attributes: string
  content: string
}

function anchors(html: string): AnchorElement[] {
  return Array.from(html.matchAll(/<a\b([^>]*)>([\s\S]*?)<\/a>/g), (match) => ({
    attributes: match[1],
    content: match[2],
  }))
}

function elementContent(html: string, tag: string): string {
  const match = html.match(new RegExp(`<${tag}\\b[^>]*>([\\s\\S]*?)</${tag}>`))
  assert.ok(match, `expected a <${tag}> element in:\n${html}`)

  return match[1]
}

function decodeHtml(value: string): string {
  return value
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#x27;/g, "'")
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, '&')
}

function visibleText(html: string): string {
  return decodeHtml(html.replace(/<[^>]*>/g, ' '))
    .replace(/&nbsp;/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

interface ButtonElement {
  attributes: string
  content: string
}

function buttons(html: string): ButtonElement[] {
  return Array.from(html.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g), (match) => ({
    attributes: match[1],
    content: match[2],
  }))
}

function buttonByLabel(html: string, label: string): ButtonElement {
  const button = buttons(html).find((candidate) => visibleText(candidate.content).includes(label))
  assert.ok(button, `expected a «${label}» button in:\n${html}`)

  return button
}

function isDisabled(button: ButtonElement): boolean {
  return attribute(button.attributes, 'disabled') !== undefined
}

function articleLinks(html: string): AnchorElement[] {
  return anchors(html).filter((anchor) =>
    (attribute(anchor.attributes, 'href') ?? '').startsWith('/news/'),
  )
}

function nodeText(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === 'boolean') return ''
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(nodeText).join(' ')
  if (typeof node === 'object' && 'props' in node) {
    return nodeText((node as ReactElement<{ children?: ReactNode }>).props.children)
  }

  return ''
}

function renderedControl(label: string): ReactElement<{ onClick?: unknown }> {
  const control = capturedElements.find((element) => {
    const props = element.props as { onClick?: unknown; children?: ReactNode }

    return typeof props.onClick === 'function' && nodeText(props.children).includes(label)
  })

  assert.ok(
    control,
    `expected the rendered page to contain a «${label}» control with an onClick handler`,
  )

  return control as ReactElement<{ onClick?: unknown }>
}

function clickControl(label: string): void {
  const control = renderedControl(label)
  assert.equal(typeof control.props.onClick, 'function', `expected «${label}» to be clickable`)

  ;(control.props.onClick as (event: unknown) => void)({
    preventDefault: () => undefined,
    stopPropagation: () => undefined,
  })
}

// ---------------------------------------------------------------------------
// Header and navigation
// ---------------------------------------------------------------------------

test('NAV_LINKS points every rubric to its route and keeps the other links', () => {
  const hrefByLabel = new Map(NAV_LINKS.map((link) => [link.label, link.href]))

  assert.equal(NAV_LINKS.length, 7, 'the header must contain exactly the seven rubric links')
  assert.deepEqual(
    NAV_LINKS.map((link) => link.label),
    ['Новини', 'Політика', 'Міжнародні новини', 'Регіональні новини', 'Спорт', 'Інтерв\'ю', 'Про нас'],
    'the header must list the seven rubrics in navigation order',
  )
  assert.equal(hrefByLabel.get('Новини'), '/', '«Новини» must keep the home route')
  assert.equal(hrefByLabel.get('Політика'), '/politics', '«Політика» must open /politics')
  assert.equal(
    hrefByLabel.get('Міжнародні новини'),
    '/international',
    '«Міжнародні новини» must open /international',
  )
  assert.equal(
    hrefByLabel.get('Регіональні новини'),
    '/regional',
    '«Регіональні новини» must open /regional',
  )
  assert.equal(hrefByLabel.get('Спорт'), '/sport', '«Спорт» must open /sport')
  assert.equal(hrefByLabel.get('Інтерв\'ю'), '/interview', '«Інтерв\'ю» must open /interview')
  assert.equal(hrefByLabel.get('Про нас'), '#about', '«Про нас» must keep its current href')
})

test('Header renders the exact rubric hrefs for Sport and International', async () => {
  const Header = (await import('../src/sections/Header')).default
  const html = renderToStaticMarkup(
    <MemoryRouter>
      <Header />
    </MemoryRouter>,
  )

  for (const { label, href } of [
    { label: 'Новини', href: '/' },
    { label: 'Міжнародні новини', href: '/international' },
    { label: 'Спорт', href: '/sport' },
  ]) {
    const link = anchors(html).find((candidate) => visibleText(candidate.content) === label)
    assert.ok(link, `expected the header link «${label}» in:\n${html}`)
    assert.equal(
      attribute(link.attributes, 'href'),
      href,
      `expected «${label}» to link to ${href}`,
    )
  }
})

// ---------------------------------------------------------------------------
// Routes and page content
// ---------------------------------------------------------------------------

const categoryRoutes: Array<{ route: string; category: Category }> = [
  { route: '/sport', category: 'Спорт' },
  { route: '/international', category: 'Міжнародні новини' },
  { route: '/politics', category: 'Політика' },
  { route: '/regional', category: 'Регіональні новини' },
  { route: '/interview', category: 'Інтерв\'ю' },
]

test('Category routes render their rubric heading and keep the site chrome', async () => {
  for (const { route, category } of categoryRoutes) {
    installFakeHook({ articles: [makeArticle(category, { id: 'one-article' })] })

    const html = await renderRoute(route)
    const heading = decodeHtml(elementContent(html, 'h1'))

    assert.ok(
      heading.includes(category),
      `${route}: expected the page heading to name the rubric «${category}», got ${JSON.stringify(visibleText(heading))}`,
    )
    assert.deepEqual(
      hookCalls,
      [category],
      `${route}: expected the page to load the matching category`,
    )
    assert.match(html, /<header\b/, `${route}: expected the site header`)
    assert.match(html, /<footer\b/, `${route}: expected the site footer`)
    assert.ok(html.includes(SITE_NAME), `${route}: expected the header logo text`)
    assert.ok(
      visibleText(html).includes('Усі права захищено'),
      `${route}: expected the footer content`,
    )
  }
})

test('Category article cards link to /news/:slug', async () => {
  for (const { route, category } of categoryRoutes) {
    const article = makeArticle(category, {
      id: 'світ/економіка 2026?x=1&y=2',
      title: `Новина рубрики ${category}`,
    })
    installFakeHook({ articles: [article] })

    const html = await renderRoute(route)
    const expectedHref = `/news/${encodeURIComponent(article.id)}`
    const link = articleLinks(html).find(
      (candidate) => attribute(candidate.attributes, 'href') === expectedHref,
    )

    assert.ok(link, `${route}: expected an article link to ${expectedHref} in:\n${html}`)
    assert.ok(
      visibleText(link.content).includes(article.title),
      `${route}: expected the card title inside the article link`,
    )
  }
})

test('Category page renders the main event as a large first card inside the grid', async () => {
  const mainEvent = makeArticle('Спорт', {
    id: 'sport-main-event',
    title: 'Головна подія спорту',
    mainEvent: true,
  })
  const regular = makeArticle('Спорт', {
    id: 'sport-regular',
    title: 'Звичайна новина',
  })
  installFakeHook({ articles: [regular, mainEvent] })

  const html = await renderRoute('/sport')
  const expectedHref = `/news/${encodeURIComponent(mainEvent.id)}`
  const largeCards = articleLinks(html).filter((anchor) =>
    classTokens(anchor.attributes).includes('lg:col-span-2'),
  )

  assert.equal(
    largeCards.length,
    1,
    `expected exactly one large main-event card in:\n${html}`,
  )

  const [largeCard] = largeCards
  assert.equal(
    attribute(largeCard.attributes, 'href'),
    expectedHref,
    'the large card must link to the main-event article',
  )
  assert.ok(
    visibleText(largeCard.content).includes(mainEvent.title),
    'the large card must show the main-event title',
  )

  const gridIndex = html.indexOf('lg:grid-cols-3')
  const largeCardIndex = html.indexOf(largeCard.attributes)

  assert.ok(gridIndex >= 0, `expected the article grid in:\n${html}`)
  assert.ok(
    largeCardIndex > gridIndex,
    'the main-event card must be rendered inside the article grid, not above it',
  )

  const links = articleLinks(html)
  assert.equal(
    links.length,
    2,
    'expected the large main card and the regular card only, with the main event not duplicated',
  )
  assert.equal(
    links.filter((anchor) => attribute(anchor.attributes, 'href') === expectedHref).length,
    1,
    'the main-event article must be linked exactly once',
  )
})

test('Category page omits the large block when no article is the main event', async () => {
  const articles = makeArticles(3, 'Спорт', 'sport')
  installFakeHook({ articles })

  const html = await renderRoute('/sport')
  const largeBlocks = articleLinks(html).filter((anchor) =>
    classTokens(anchor.attributes).includes('lg:col-span-2'),
  )

  assert.deepEqual(largeBlocks, [], `expected no large main-event block in:\n${html}`)
  assert.equal(articleLinks(html).length, articles.length)
})

// ---------------------------------------------------------------------------
// Loading, error and empty states
// ---------------------------------------------------------------------------

test('Category page shows a loading status while the rubric is loading', async () => {
  // The real hook starts in its loading state; SSR must already expose it.
  currentHook = (await import('../src/hooks/use-category-news')).default

  const html = await renderRoute('/sport')

  assert.match(
    html,
    /role="status"|aria-live="polite"/,
    `expected an accessible loading status in:\n${html}`,
  )
  assert.match(visibleText(html), /Завантаження/, `expected loading copy in:\n${html}`)
  assert.ok(
    !visibleText(html).includes('Опублікованих новин поки немає.'),
    'the loading state must not look like an empty rubric',
  )
  assert.ok(!/role="alert"/.test(html), 'the loading state must not look like an error')
})

test('Category page shows a retryable error instead of an empty rubric', async () => {
  installFakeHook({ error: 'Категорію не знайдено', loading: false })

  const html = await renderRoute('/sport')

  assert.match(html, /role="alert"/, `expected an error alert in:\n${html}`)
  assert.ok(
    visibleText(html).includes('Категорію не знайдено'),
    `expected the API error message in:\n${html}`,
  )
  buttonByLabel(html, 'Спробувати ще раз')
  assert.ok(
    !visibleText(html).includes('Опублікованих новин поки немає.'),
    'an error must not be rendered as an empty rubric',
  )
  assert.deepEqual(articleLinks(html), [], 'an error must not show stale article cards')
})

test('Category page explains an empty rubric', async () => {
  installFakeHook({ articles: [], loading: false, error: null })

  const html = await renderRoute('/international')
  const text = visibleText(html)

  assert.ok(
    text.includes('Опублікованих новин поки немає.'),
    `expected the public empty-rubric copy in:\n${html}`,
  )
  assert.ok(!/role="alert"/.test(html), 'an empty rubric is not an error')
  assert.deepEqual(articleLinks(html), [], 'an empty rubric must not show article cards')
  assert.ok(
    text.includes('Міжнародні новини'),
    'the rubric heading must stay visible when the rubric is empty',
  )
})

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

interface PaginationCase {
  name: string
  state: Partial<CategoryNewsState>
  pageText: string
  backDisabled: boolean
  nextDisabled: boolean
}

const paginationCases: PaginationCase[] = [
  {
    name: 'first page without further results disables both directions',
    state: { page: 1, articles: [makeArticle('Спорт')], hasNextPage: false },
    pageText: 'Сторінка 1',
    backDisabled: true,
    nextDisabled: true,
  },
  {
    name: 'first page with further results enables only «Далі»',
    state: { page: 1, articles: makeArticles(20, 'Спорт', 'sport'), hasNextPage: true },
    pageText: 'Сторінка 1',
    backDisabled: true,
    nextDisabled: false,
  },
  {
    name: 'middle page enables both directions',
    state: { page: 2, articles: makeArticles(20, 'Спорт', 'sport'), hasNextPage: true },
    pageText: 'Сторінка 2',
    backDisabled: false,
    nextDisabled: false,
  },
  {
    name: 'last page enables only «Назад»',
    state: { page: 2, articles: [makeArticle('Спорт')], hasNextPage: false },
    pageText: 'Сторінка 2',
    backDisabled: false,
    nextDisabled: true,
  },
]

test('Category pagination shows the current page and disables unavailable controls', async () => {
  for (const paginationCase of paginationCases) {
    installFakeHook(paginationCase.state)

    const html = await renderRoute('/sport')
    const text = visibleText(html)

    assert.ok(
      text.includes(paginationCase.pageText),
      `${paginationCase.name}: expected «${paginationCase.pageText}» in:\n${html}`,
    )
    assert.equal(
      isDisabled(buttonByLabel(html, 'Назад')),
      paginationCase.backDisabled,
      `${paginationCase.name}: unexpected «Назад» state`,
    )
    assert.equal(
      isDisabled(buttonByLabel(html, 'Далі')),
      paginationCase.nextDisabled,
      `${paginationCase.name}: unexpected «Далі» state`,
    )
  }
})

test('Category pagination and retry controls call the hook callbacks', async () => {
  const invocations: string[] = []
  installFakeHook({
    page: 2,
    articles: makeArticles(20, 'Спорт', 'sport'),
    hasNextPage: true,
    nextPage: () => invocations.push('nextPage'),
    previousPage: () => invocations.push('previousPage'),
  })

  await renderRoute('/sport')

  clickControl('Назад')
  clickControl('Далі')

  assert.deepEqual(
    invocations,
    ['previousPage', 'nextPage'],
    'expected the pagination buttons to call the hook callbacks',
  )

  installFakeHook({
    error: 'Категорію не знайдено',
    retry: () => invocations.push('retry'),
  })
  await renderRoute('/sport')

  clickControl('Спробувати ще раз')

  assert.deepEqual(
    invocations,
    ['previousPage', 'nextPage', 'retry'],
    'expected the retry button to call the hook retry callback',
  )
})
