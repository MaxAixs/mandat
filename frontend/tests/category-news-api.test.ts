import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { listPublicNews } from '../src/api/news'
import type { PublicNews, PublicNewsListResponse } from '../src/api/news'
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

type HomeApi = typeof import('../src/api')

// CategoryNewsData and getCategoryNews are specified by the plan but not
// exported yet. Declaring them locally keeps this suite compiling so the
// failure stays behavioral instead of a type error.
type CategoryNewsDataPayload = {
  articles: Article[]
  page: number
  limit: number
}

type ExpectedCategoryApi = HomeApi & {
  getCategoryNews?: (
    category: Category,
    page?: number,
    limit?: number,
  ) => Promise<CategoryNewsDataPayload>
}

// The optional third argument is not part of the current signature yet, so the
// test keeps the specified call shape locally.
type ListPublicNewsWithCategory = (
  page?: number,
  limit?: number,
  category?: Category,
) => Promise<PublicNewsListResponse>

let getCategoryNews: ExpectedCategoryApi['getCategoryNews']

before(async () => {
  const api = (await import('../src/api')) as ExpectedCategoryApi
  getCategoryNews = api.getCategoryNews
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

test('listPublicNews encodes the category together with page and limit', async () => {
  const listWithCategory = listPublicNews as unknown as ListPublicNewsWithCategory
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items: [], page: 2, limit: 20 }), { status: 200 })
  }

  const result = await listWithCategory(2, 20, 'Спорт')

  assert.equal(
    requestedUrl,
    `${PUBLIC_NEWS_URL}?page=2&limit=20&category=${SPORT_CATEGORY_ENCODED}`,
  )
  assert.equal(searchParams(requestedUrl).get('category'), 'Спорт')
  assert.deepEqual(result, { items: [], page: 2, limit: 20 })
})

test('listPublicNews keeps existing two-argument calls free of the category parameter', async () => {
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items: [], page: 2, limit: 20 }), { status: 200 })
  }

  await listPublicNews(2, 20)

  assert.equal(requestedUrl, `${PUBLIC_NEWS_URL}?page=2&limit=20`)
  assert.equal(
    searchParams(requestedUrl).has('category'),
    false,
    'the home feed must stay unfiltered when no category is requested',
  )
})

test('getCategoryNews maps rubric items into Article cards and preserves pagination', async () => {
  const getCategory = requireFunction(getCategoryNews, 'getCategoryNews')
  const withImage = makeItem({
    slug: 'sport-final',
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
    slug: 'sport-preview',
    category: 'Спорт',
    title: 'Прев’ю матчу',
    description: 'Опис прев’ю',
    image_url: '',
    featured: false,
    main_event: true,
    views: 0,
    published_at: localIsoDate(24, 9, 5),
  })
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items: [withImage, withoutImage], page: 2, limit: 2 }), {
      status: 200,
    })
  }

  const result = await getCategory('Спорт', 2, 2)

  const params = searchParams(requestedUrl)
  assert.equal(params.get('page'), '2')
  assert.equal(params.get('limit'), '2')
  assert.equal(params.get('category'), 'Спорт')
  assert.deepEqual(result, {
    articles: [
      {
        id: 'sport-final',
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
        id: 'sport-preview',
        category: 'Спорт',
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

test('getCategoryNews defaults to the first page with 20 items', async () => {
  const getCategory = requireFunction(getCategoryNews, 'getCategoryNews')
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items: [], page: 1, limit: 20 }), { status: 200 })
  }

  const result = await getCategory('Міжнародні новини')

  const params = searchParams(requestedUrl)
  assert.equal(params.get('page'), '1')
  assert.equal(params.get('limit'), '20')
  assert.equal(params.get('category'), 'Міжнародні новини')
  assert.deepEqual(result, { articles: [], page: 1, limit: 20 })
})

test('getCategoryNews returns an empty article list for an empty rubric', async () => {
  const getCategory = requireFunction(getCategoryNews, 'getCategoryNews')
  mockResponse({ items: [], page: 1, limit: 20 })

  const result = await getCategory('Спорт')

  assert.deepEqual(result, { articles: [], page: 1, limit: 20 })
})

test('getCategoryNews surfaces backend errors', async () => {
  const getCategory = requireFunction(getCategoryNews, 'getCategoryNews')
  mockResponse({ error: { message: 'Invalid input' } }, 400)

  await assert.rejects(getCategory('Спорт'), /Invalid input/)
})
