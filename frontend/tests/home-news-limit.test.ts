import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import type { PublicNews, PublicNewsListResponse } from '../src/api/news'

// src/api/index.ts imports '@/api/news'; Node needs the alias resolved at runtime.
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

type HomeApi = typeof import('../src/api')

let getHomeNews: HomeApi['getHomeNews']

before(async () => {
  const api = (await import('../src/api')) as HomeApi
  getHomeNews = api.getHomeNews
})

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
})

const PUBLIC_NEWS_URL = 'http://localhost:8080/api/news'
const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
})

function makeItem(overrides: Partial<PublicNews> = {}): PublicNews {
  return {
    slug: 'test-slug',
    category: 'Новини',
    title: 'Тестова новина',
    description: 'Короткий опис',
    image_url: '',
    featured: false,
    main_event: false,
    views: 0,
    published_at: null,
    ...overrides,
  }
}

// Eight public news items: three featured, five regular.
// published_at descends from item 1 to item 8, so chronological order is 1..8.
const items: PublicNews[] = [
  makeItem({
    slug: 'news-01',
    category: 'Новини',
    title: 'Новина 1',
    description: 'Опис новини 1',
    featured: false,
    views: 101,
    published_at: '2026-10-30T09:00:00Z',
  }),
  makeItem({
    slug: 'news-02',
    category: 'Міжнародні новини',
    title: 'Новина 2',
    description: 'Опис новини 2',
    featured: true,
    views: 102,
    published_at: '2026-10-29T09:00:00Z',
  }),
  makeItem({
    slug: 'news-03',
    category: 'Спорт',
    title: 'Новина 3',
    description: 'Опис новини 3',
    featured: false,
    views: 103,
    published_at: '2026-10-28T09:00:00Z',
  }),
  makeItem({
    slug: 'news-04',
    category: "Інтерв'ю",
    title: 'Новина 4',
    description: 'Опис новини 4',
    featured: true,
    views: 104,
    published_at: '2026-10-27T09:00:00Z',
  }),
  makeItem({
    slug: 'news-05',
    category: 'Новини',
    title: 'Новина 5',
    description: 'Опис новини 5',
    featured: false,
    views: 105,
    published_at: '2026-10-26T09:00:00Z',
  }),
  makeItem({
    slug: 'news-06',
    category: 'Міжнародні новини',
    title: 'Новина 6',
    description: 'Опис новини 6',
    featured: true,
    views: 106,
    published_at: '2026-10-25T09:00:00Z',
  }),
  makeItem({
    slug: 'news-07',
    category: 'Спорт',
    title: 'Новина 7',
    description: 'Опис новини 7',
    featured: false,
    views: 107,
    published_at: '2026-10-24T09:00:00Z',
  }),
  makeItem({
    slug: 'news-08',
    category: 'Новини',
    title: 'Новина 8',
    description: 'Опис новини 8',
    featured: false,
    views: 108,
    published_at: '2026-10-23T09:00:00Z',
  }),
]

// Featured-first order the home page uses before applying the limit:
// the three featured items keep their relative order, then the regular ones.
const FEATURED_FIRST_ORDER = [
  'news-02',
  'news-04',
  'news-06',
  'news-01',
  'news-03',
  'news-05',
  'news-07',
  'news-08',
]

// Chronological order used by «Останні новини» and the ticker.
const CHRONOLOGICAL_ORDER = [
  'news-01',
  'news-02',
  'news-03',
  'news-04',
  'news-05',
  'news-06',
  'news-07',
  'news-08',
]

test('getHomeNews limits the main news list to six featured-first articles', async () => {
  const responseBody: PublicNewsListResponse = { items, page: 1, limit: 20 }
  let requestedUrl = ''
  let calls = 0

  globalThis.fetch = async (input) => {
    calls += 1
    requestedUrl = String(input)
    return new Response(JSON.stringify(responseBody), { status: 200 })
  }

  const result = await getHomeNews()

  assert.equal(calls, 1, 'the home page must load the feed with a single request')
  assert.equal(requestedUrl, `${PUBLIC_NEWS_URL}?page=1&limit=20`)

  // «Головні новини» shows at most six articles.
  assert.equal(
    result.articles.length,
    6,
    `expected the main news list to be limited to 6, got ${result.articles.length}`,
  )

  // The six articles are the leading slice of the pre-limit featured-first
  // order — limiting must not reorder, drop featured items, or take raw
  // pagination order instead.
  assert.deepEqual(
    result.articles.map((article) => article.id),
    FEATURED_FIRST_ORDER.slice(0, 6),
    'expected the six main news to be the featured-first prefix',
  )
  assert.deepEqual(
    result.articles.map((article) => article.featured),
    [true, true, true, false, false, false],
    'expected featured articles to stay first inside the limited list',
  )

  // Only «Головні новини» is limited: the secondary feeds keep every item.
  assert.equal(
    result.latestNews.length,
    8,
    `expected latestNews to keep all 8 fetched items, got ${result.latestNews.length}`,
  )
  assert.deepEqual(
    result.latestNews.map((item) => item.id),
    CHRONOLOGICAL_ORDER,
    'expected latestNews to stay chronological over the full fetched page',
  )

  // The ticker keeps its own five-newest-headlines behavior.
  assert.equal(
    result.tickerItems.length,
    5,
    `expected tickerItems to keep its own limit of 5, got ${result.tickerItems.length}`,
  )
  assert.deepEqual(
    result.tickerItems,
    items.slice(0, 5).map((item) => item.title),
    'expected the ticker to keep the five newest headlines from the full page',
  )
})
