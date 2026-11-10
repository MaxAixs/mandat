import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import type { PublicNews } from '../src/api/news'
import type { Category, LatestNewsItem } from '../src/types'

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

type CategoryApi = typeof import('../src/api')

// getCategoryLatestNews is specified by the review but may not be exported yet.
// Declaring it locally keeps the suite compiling so the failure stays behavioral.
type ExpectedCategoryApi = CategoryApi & {
  getCategoryLatestNews?: (category: Category) => Promise<LatestNewsItem[]>
}

let getCategoryLatestNews: ExpectedCategoryApi['getCategoryLatestNews']

before(async () => {
  const api = (await import('../src/api')) as ExpectedCategoryApi
  getCategoryLatestNews = api.getCategoryLatestNews
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

test('getCategoryLatestNews requests the first page of the rubric with limit 5', async () => {
  const loadLatest = requireFunction(getCategoryLatestNews, 'getCategoryLatestNews')
  const requestedUrls: string[] = []
  globalThis.fetch = async (input) => {
    requestedUrls.push(String(input))
    return new Response(JSON.stringify({ items: [], page: 1, limit: 5 }), { status: 200 })
  }

  await loadLatest('Спорт')

  assert.equal(requestedUrls.length, 1, 'the strip loader must issue exactly one request')
  assert.equal(
    requestedUrls[0],
    `${PUBLIC_NEWS_URL}?page=1&limit=5&category=${SPORT_CATEGORY_ENCODED}`,
  )
  const params = searchParams(requestedUrls[0])
  assert.equal(params.get('page'), '1')
  assert.equal(params.get('limit'), '5')
  assert.equal(params.get('category'), 'Спорт')
})

test('getCategoryLatestNews maps items into LatestNewsItem entries', async () => {
  const loadLatest = requireFunction(getCategoryLatestNews, 'getCategoryLatestNews')
  const interview = makeItem({
    slug: 'interview-final',
    category: 'Інтерв\'ю',
    title: 'Інтерв’ю з тренером',
    published_at: localIsoDate(25, 14, 35),
  })
  const regular = makeItem({
    slug: 'sport-preview',
    category: 'Спорт',
    title: 'Прев’ю матчу',
    published_at: null,
  })
  mockResponse({ items: [interview, regular], page: 1, limit: 5 })

  const result = await loadLatest('Спорт')

  assert.deepEqual(result, [
    {
      id: 'interview-final',
      time: '14:35',
      source: 'Інтерв\'ю',
      title: 'Інтерв’ю з тренером',
    },
    {
      id: 'sport-preview',
      time: '',
      source: 'Спорт',
      title: 'Прев’ю матчу',
    },
  ])
})

test('getCategoryLatestNews surfaces backend errors', async () => {
  const loadLatest = requireFunction(getCategoryLatestNews, 'getCategoryLatestNews')
  mockResponse({ error: { message: 'Invalid input' } }, 400)

  await assert.rejects(loadLatest('Спорт'), /Invalid input/)
})
