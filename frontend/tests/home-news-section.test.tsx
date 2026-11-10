import { after, afterEach, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

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

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

// ---------------------------------------------------------------------------
// «Головні новини» now loads its own page through useCategoryNews. The server
// renderer never runs effects, so the tests inject the hook state a browser
// would obtain, while the section markup and NewsCard stay the real code.
// ---------------------------------------------------------------------------

interface HomeNewsHookState {
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

interface HomeNewsHookCall {
  category: Category | null
  limit: number | undefined
}

type HomeNewsHook = (category: Category | null, limit?: number) => HomeNewsHookState

function unavailableHook(category: Category | null): HomeNewsHookState {
  throw new Error(
    `News rendered the home feed («${String(category)}») without a configured useCategoryNews test state`,
  )
}

let currentHook: HomeNewsHook = unavailableHook
const hookCalls: HomeNewsHookCall[] = []

// A stable proxy, so switching the state between renders keeps working after
// News.js has already been loaded and cached.
const hookProxy: HomeNewsHook = (category, limit) => {
  hookCalls.push({ category, limit })

  return currentHook(category, limit)
}

moduleInternals._load = function loadWithHomeNewsHook(request, parent, ...rest) {
  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''
  const importsHomeNewsHook =
    request === '@/hooks/use-category-news' ||
    (parentFilename.endsWith(path.join('sections', 'News.js')) &&
      request.endsWith('use-category-news'))

  if (importsHomeNewsHook) {
    return { __esModule: true, default: hookProxy, useCategoryNews: hookProxy }
  }

  return originalLoad.call(moduleInternals, request, parent, ...rest)
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  moduleInternals._load = originalLoad
})

afterEach(() => {
  currentHook = unavailableHook
  hookCalls.length = 0
})

// ---------------------------------------------------------------------------
// Fixtures and helpers
// ---------------------------------------------------------------------------

interface FakeHookOptions extends Partial<HomeNewsHookState> {
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
  const state: HomeNewsHookState = {
    articles: [],
    loading: false,
    error: null,
    page: 1,
    limit: 6,
    hasNextPage: false,
    retry: onRetry,
    nextPage: onNextPage,
    previousPage: onPreviousPage,
    ...overrides,
  }

  hookCalls.length = 0
  currentHook = () => state
}

let newsPromise: Promise<ComponentType> | undefined

async function loadNews(): Promise<ComponentType> {
  newsPromise ??= import('../src/sections/News').then((module) => {
    assert.equal(
      typeof module.default,
      'function',
      'src/sections/News.tsx must default-export the news section',
    )

    return module.default
  })

  try {
    return await newsPromise
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    assert.fail(`src/sections/News.tsx must render the home news section, but it failed: ${reason}`)
  }
}

async function renderNews(): Promise<string> {
  const News = await loadNews()

  return renderToStaticMarkup(
    <MemoryRouter>
      <News />
    </MemoryRouter>,
  )
}

function makeArticle(overrides: Partial<Article> = {}): Article {
  return {
    id: 'test-slug',
    category: 'Новини',
    title: 'Тестовий заголовок новини',
    excerpt: 'Короткий опис новини для картки',
    views: '12,4\u00A0тис.',
    publishedAt: '25.10.2026, 14:35',
    featured: false,
    imageUrl: 'https://cdn.example.com/news-card.jpg',
    ...overrides,
  }
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

function cardClasses(html: string, id: string): string[] | undefined {
  const anchor = anchors(html).find(
    (candidate) => attribute(candidate.attributes, 'href') === `/news/${encodeURIComponent(id)}`,
  )

  return anchor ? classTokens(anchor.attributes) : undefined
}

// A large NewsCard marks its outer link with lg:col-span-2 lg:row-span-2.
function largeCardCount(html: string): number {
  return html.split('lg:col-span-2').length - 1
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

// The section must keep the «Назад / Сторінка N / Далі» pagination and disable
// the directions the hook reports as unavailable.
function assertPagination(html: string, page: number, backDisabled: boolean, nextDisabled: boolean): void {
  assert.ok(
    visibleText(html).includes(`Сторінка ${page}`),
    `expected the pagination to show «Сторінка ${page}» in:\n${html}`,
  )
  assert.equal(
    isDisabled(buttonByLabel(html, 'Назад')),
    backDisabled,
    `unexpected «Назад» state on page ${page}:\n${html}`,
  )
  assert.equal(
    isDisabled(buttonByLabel(html, 'Далі')),
    nextDisabled,
    `unexpected «Далі» state on page ${page}:\n${html}`,
  )
}

// ---------------------------------------------------------------------------
// Large mosaic card
// ---------------------------------------------------------------------------

test('News renders only the first featured article as the large card', async () => {
  const first = makeArticle({ id: 'first-featured', featured: true })
  const second = makeArticle({ id: 'second-featured', featured: true })
  const third = makeArticle({ id: 'regular-third' })
  installFakeHook({ articles: [first, second, third], page: 1, hasNextPage: true })

  const html = await renderNews()

  assert.equal(
    largeCardCount(html),
    1,
    `expected at most one large mosaic card, got ${largeCardCount(html)} in:\n${html}`,
  )

  const firstClasses = cardClasses(html, first.id)
  assert.ok(firstClasses, `expected a card for ${first.id} in:\n${html}`)
  assert.ok(
    firstClasses.includes('lg:col-span-2'),
    `expected the first featured article to render large in:\n${html}`,
  )

  for (const article of [second, third]) {
    const classes = cardClasses(html, article.id)
    assert.ok(classes, `expected a card for ${article.id} in:\n${html}`)
    assert.ok(
      !classes.includes('lg:col-span-2'),
      `expected ${article.id} at index > 0 not to render large in:\n${html}`,
    )
    assert.ok(
      !classes.includes('lg:row-span-2'),
      `expected ${article.id} at index > 0 not to span two rows in:\n${html}`,
    )
  }

  // The section loads the unfiltered home feed itself: six articles per page.
  assert.deepEqual(
    hookCalls,
    [{ category: null, limit: 6 }],
    'expected «Головні новини» to load the home feed page of six without a rubric filter',
  )
  assertPagination(html, 1, true, false)
})

test('News renders the mosaic leader large when it is the main event', async () => {
  const leader = makeArticle({ id: 'mosaic-leader', mainEvent: true, featured: false })
  const featuredLater = makeArticle({ id: 'featured-later', featured: true })
  const mainEventLater = makeArticle({ id: 'main-event-later', mainEvent: true })
  const regular = makeArticle({ id: 'regular' })
  installFakeHook({
    articles: [leader, featuredLater, mainEventLater, regular],
    page: 2,
    hasNextPage: false,
  })

  const html = await renderNews()

  assert.equal(
    largeCardCount(html),
    1,
    `expected exactly one large mosaic card, got ${largeCardCount(html)} in:\n${html}`,
  )

  const leaderClasses = cardClasses(html, leader.id)
  assert.ok(leaderClasses, `expected a card for ${leader.id} in:\n${html}`)
  for (const className of ['lg:col-span-2', 'lg:row-span-2']) {
    assert.ok(
      leaderClasses.includes(className),
      `expected the main-event leader to render large with ${className} in:\n${html}`,
    )
  }

  for (const later of [featuredLater, mainEventLater]) {
    const laterClasses = cardClasses(html, later.id)
    assert.ok(laterClasses, `expected a card for ${later.id} in:\n${html}`)
    assert.ok(
      !laterClasses.includes('lg:col-span-2'),
      `expected ${later.id} at index > 0 not to render large in:\n${html}`,
    )
  }

  assertPagination(html, 2, false, true)
})

test('News keeps every card regular when the leader is neither featured nor a main event', async () => {
  const leader = makeArticle({ id: 'plain-leader', featured: false, mainEvent: false })
  const featuredLater = makeArticle({ id: 'featured-later', featured: true })
  const regular = makeArticle({ id: 'regular' })
  installFakeHook({ articles: [leader, featuredLater, regular], page: 1, hasNextPage: false })

  const html = await renderNews()

  assert.equal(
    largeCardCount(html),
    0,
    `expected no large mosaic card when the leader is plain, got ${largeCardCount(html)} in:\n${html}`,
  )
})
