import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import type { PublicNews } from '../src/api/news'
import type { Article, Category } from '../src/types'

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

type FeedApi = typeof import('../src/api')

// getFeedNews and FeedNewsData are specified by the feed plan but not exported
// yet. Declaring them locally keeps this suite compiling so the failure stays
// behavioral instead of a type error.
type FeedNewsDataPayload = {
  articles: Article[]
  page: number
  limit: number
}

type ExpectedFeedApi = FeedApi & {
  getFeedNews?: (
    category: Category | null,
    page?: number,
    limit?: number,
  ) => Promise<FeedNewsDataPayload>
}

let getFeedNews: ExpectedFeedApi['getFeedNews']

before(async () => {
  const api = (await import('../src/api')) as ExpectedFeedApi
  getFeedNews = api.getFeedNews
})

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
})

const PUBLIC_NEWS_URL = 'http://localhost:8080/api/news'
const SPORT_CATEGORY_ENCODED = '%D0%A1%D0%BF%D0%BE%D1%80%D1%82'
const originalFetch = globalThis.fetch

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

function localIsoDate(day: number, hour: number, minute: number): string {
  return new Date(2026, 9, day, hour, minute).toISOString()
}

function mockResponse(body: unknown, status = 200): void {
  globalThis.fetch = async () => new Response(JSON.stringify(body), { status })
}

function requireFunction<T>(fn: T | undefined, name: string): T {
  assert.equal(typeof fn, 'function', `${name} must be exported from src/api and callable`)

  return fn as T
}

function searchParams(url: string): URLSearchParams {
  return new URL(url).searchParams
}

afterEach(() => {
  globalThis.fetch = originalFetch
})

test('getFeedNews is exported from src/api and callable', () => {
  requireFunction(getFeedNews, 'getFeedNews')
})

test('getFeedNews without a category requests the unfiltered first page', async () => {
  const getFeed = requireFunction(getFeedNews, 'getFeedNews')
  const requestedUrls: string[] = []
  globalThis.fetch = async (input) => {
    requestedUrls.push(String(input))
    return new Response(JSON.stringify({ items: [], page: 1, limit: 20 }), { status: 200 })
  }

  const result = await getFeed(null)

  assert.equal(requestedUrls.length, 1, 'the feed loader must issue exactly one request')
  assert.equal(
    requestedUrls[0],
    `${PUBLIC_NEWS_URL}?page=1&limit=20`,
    'a null filter must not append a category parameter',
  )
  assert.equal(
    searchParams(requestedUrls[0]).has('category'),
    false,
    'the unfiltered feed must not be filtered by a category',
  )
  assert.deepEqual(result, { articles: [], page: 1, limit: 20 })
})

test('getFeedNews encodes the selected category', async () => {
  const getFeed = requireFunction(getFeedNews, 'getFeedNews')
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items: [], page: 1, limit: 20 }), { status: 200 })
  }

  await getFeed('Спорт')

  assert.equal(
    requestedUrl,
    `${PUBLIC_NEWS_URL}?page=1&limit=20&category=${SPORT_CATEGORY_ENCODED}`,
  )
  assert.equal(searchParams(requestedUrl).get('category'), 'Спорт')
})

test('getFeedNews maps rubric items into Article cards and preserves pagination', async () => {
  const getFeed = requireFunction(getFeedNews, 'getFeedNews')
  const withImage = makeItem({
    slug: 'feed-sport-final',
    category: 'Спорт',
    title: 'Перемога у фіналі',
    description: 'Короткий опис перемоги',
    image_url: 'https://cdn.example.com/sport.jpg',
    featured: true,
    main_event: false,
    views: 12400,
    published_at: localIsoDate(25, 14, 35),
  })
  const withoutImage = makeItem({
    slug: 'feed-news-preview',
    category: 'Новини',
    title: 'Прев’ю матчу',
    description: 'Опис прев’ю',
    image_url: '',
    featured: false,
    main_event: true,
    views: 0,
    published_at: localIsoDate(24, 9, 5),
  })
  mockResponse({ items: [withImage, withoutImage], page: 2, limit: 2 })

  const result = await getFeed('Спорт', 2, 2)

  assert.deepEqual(result, {
    articles: [
      {
        id: 'feed-sport-final',
        category: 'Спорт',
        title: 'Перемога у фіналі',
        excerpt: 'Короткий опис перемоги',
        views: '12,4\u00A0тис.',
        publishedAt: '25.10.2026, 14:35',
        featured: true,
        mainEvent: false,
        imageUrl: 'https://cdn.example.com/sport.jpg',
      },
      {
        id: 'feed-news-preview',
        category: 'Новини',
        title: 'Прев’ю матчу',
        excerpt: 'Опис прев’ю',
        views: '0',
        publishedAt: '24.10.2026, 09:05',
        featured: false,
        mainEvent: true,
        imageUrl: undefined,
      },
    ],
    page: 2,
    limit: 2,
  })
})

test('getFeedNews passes the requested page and limit through to the backend', async () => {
  const getFeed = requireFunction(getFeedNews, 'getFeedNews')
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items: [], page: 3, limit: 10 }), { status: 200 })
  }

  const result = await getFeed(null, 3, 10)

  const params = searchParams(requestedUrl)
  assert.equal(params.get('page'), '3')
  assert.equal(params.get('limit'), '10')
  assert.equal(params.has('category'), false)
  assert.deepEqual(result, { articles: [], page: 3, limit: 10 })
})

test('getFeedNews surfaces backend errors', async () => {
  const getFeed = requireFunction(getFeedNews, 'getFeedNews')
  mockResponse({ error: { message: 'Invalid input' } }, 400)

  await assert.rejects(getFeed('Спорт'), /Invalid input/)
})
