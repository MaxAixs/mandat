import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import {
  createNews,
  MAX_IMAGE_BYTES,
  MAX_IMAGES,
  IMAGE_TYPES,
  requestImageUpload,
  updateNews,
} from '../src/api/news'
import type { PublicNewsDetail, VideoUpload } from '../src/api/news'
import type { AdminNews, ArticleDetail, NewsFormValues } from '../src/types'

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

let getPublicNewsArticle: HomeApi['getPublicNewsArticle']

before(async () => {
  const api = await import('../src/api')
  getPublicNewsArticle = api.getPublicNewsArticle
})

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
})

const originalFetch = globalThis.fetch

const payload: NewsFormValues = {
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

function requireFunction<T>(fn: T | undefined, name: string): T {
  assert.equal(typeof fn, 'function', `${name} must be exported and callable`)
  return fn as T
}

afterEach(() => {
  globalThis.fetch = originalFetch
})

test('image upload limits match the specified contract', () => {
  assert.equal(
    MAX_IMAGES,
    10,
    'MAX_IMAGES must be exported from src/api/news and equal 10',
  )
  assert.equal(
    MAX_IMAGE_BYTES,
    10 * 1024 * 1024,
    'MAX_IMAGE_BYTES must be exported from src/api/news and equal 10 MB',
  )
  assert.deepEqual(
    IMAGE_TYPES,
    ['image/jpeg', 'image/png', 'image/webp', 'image/gif'],
    'IMAGE_TYPES must be exported from src/api/news',
  )
})

test('requestImageUpload posts the content type and size', async () => {
  const cases = [
    { type: 'image/jpeg', size: 12_345 },
    { type: 'image/png', size: 999 },
    { type: 'image/webp', size: 1_000_000 },
    { type: 'image/gif', size: 42 },
  ]

  for (const file of cases) {
    let request: { url: string; init?: RequestInit } | undefined
    const responseBody: VideoUpload = {
      key: 'pending/images/0123456789abcdef0123456789abcdef.jpg',
      upload_url: 'https://r2.example.com/presigned-put',
      expires_at: '2026-11-23T12:00:00Z',
    }
    globalThis.fetch = async (input, init) => {
      request = { url: String(input), init }
      return new Response(JSON.stringify(responseBody), { status: 200 })
    }

    const upload = await requireFunction(requestImageUpload, 'requestImageUpload')(file)

    assert.equal(request?.url, '/api/admin/news/image-uploads')
    assert.equal(request?.init?.method, 'POST')
    assert.equal(request?.init?.credentials, 'include')
    assert.deepEqual(request?.init?.headers, { 'Content-Type': 'application/json' })
    assert.equal(
      request?.init?.body,
      JSON.stringify({ content_type: file.type, size: file.size }),
    )
    assert.deepEqual(upload, responseBody)
  }
})

test('requestImageUpload throws the server error message', async () => {
  const cases = [
    { status: 400, message: 'Розмір файлу перевищує 10 МБ.' },
    { status: 415, message: 'Непідтримуваний тип зображення.' },
  ]

  for (const { status, message } of cases) {
    globalThis.fetch = async () =>
      new Response(JSON.stringify({ error: { message } }), { status })

    await assert.rejects(
      requireFunction(requestImageUpload, 'requestImageUpload')({
        type: 'image/jpeg',
        size: 11 * 1024 * 1024,
      }),
      new RegExp(message.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')),
    )
  }
})

test('requestImageUpload falls back to a status message for non-JSON errors', async () => {
  globalThis.fetch = async () => new Response('internal failure', { status: 500 })

  await assert.rejects(
    requireFunction(requestImageUpload, 'requestImageUpload')({
      type: 'image/jpeg',
      size: 100,
    }),
    /Помилка сервера \(500\)\./,
  )
})

test('requestImageUpload reports network failures like other calls', async () => {
  globalThis.fetch = async () => {
    throw new TypeError('fetch failed')
  }

  await assert.rejects(
    requireFunction(requestImageUpload, 'requestImageUpload')({
      type: 'image/jpeg',
      size: 100,
    }),
    /Не вдалося з’єднатися з сервером/,
  )
})

test('createNews sends images in the JSON body in order', async () => {
  const images = [
    'pending/images/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg',
    'https://cdn.example.com/images/kept.png',
    'https://example.com/external.webp',
  ]
  let body: unknown
  globalThis.fetch = async (input, init) => {
    assert.equal(String(input), '/api/admin/news')
    body = JSON.parse(String(init?.body)) as unknown
    return new Response(JSON.stringify({ id: '1', slug: 'test-slug' }), { status: 201 })
  }

  await createNews({ ...payload, images })

  const parsed = body as Record<string, unknown>
  assert.deepEqual(parsed.images, images, 'the create payload must send images in order')
})

test('createNews sends an explicit empty images array', async () => {
  let body: unknown
  globalThis.fetch = async (input, init) => {
    assert.equal(String(input), '/api/admin/news')
    body = JSON.parse(String(init?.body)) as unknown
    return new Response(JSON.stringify({ id: '1', slug: 'test-slug' }), { status: 201 })
  }

  await createNews({ ...payload, images: [] })

  const parsed = body as Record<string, unknown>
  assert.deepEqual(parsed.images, [])
})

test('updateNews sends images in the JSON body in order', async () => {
  const images = [
    'https://cdn.example.com/images/kept-a.jpg',
    'pending/images/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png',
    'https://cdn.example.com/images/kept-b.webp',
  ]
  let body: unknown
  globalThis.fetch = async (input, init) => {
    assert.equal(String(input), '/api/admin/news/test-slug')
    body = JSON.parse(String(init?.body)) as unknown
    return new Response(JSON.stringify({ id: '1', slug: 'test-slug' }), { status: 200 })
  }

  await updateNews('test-slug', { ...payload, images })

  const parsed = body as Record<string, unknown>
  assert.deepEqual(parsed.images, images, 'the update payload must send images in order')
})

test('NewsFormValues accepts an optional ordered images list', () => {
  const withImages: NewsFormValues = {
    ...payload,
    images: ['pending/images/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg'],
  }
  const withoutImages: NewsFormValues = { ...payload }

  assert.deepEqual(withImages.images, ['pending/images/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg'])
  assert.equal(withoutImages.images, undefined)
})

test('AdminNews carries an optional images list', () => {
  const admin: AdminNews = {
    ...payload,
    id: '1',
    slug: 'test-slug',
    views: 0,
    published_at: null,
    created_at: '2026-11-23T10:00:00Z',
    updated_at: '2026-11-23T10:00:00Z',
    images: ['https://cdn.example.com/images/a.jpg'],
  }

  assert.deepEqual(admin.images, ['https://cdn.example.com/images/a.jpg'])
})

test('ArticleDetail carries an optional images list', () => {
  const article: ArticleDetail = {
    id: 'test-slug',
    category: 'Новини',
    title: 'Тестова новина',
    excerpt: 'Короткий опис',
    views: '0',
    publishedAt: '',
    text: 'Повний текст',
    images: ['https://cdn.example.com/images/a.jpg'],
  }

  assert.deepEqual(article.images, ['https://cdn.example.com/images/a.jpg'])
})

function makePublicDetail(overrides: Partial<PublicNewsDetail> = {}): PublicNewsDetail {
  return {
    slug: 'image-article',
    category: 'Новини',
    title: 'Новина з фото',
    description: 'Короткий опис',
    image_url: '',
    featured: false,
    main_event: false,
    views: 0,
    published_at: null,
    text: 'Текст новини',
    video: null,
    images: [],
    ...overrides,
  }
}

test('getPublicNewsArticle maps a non-empty images array onto the article', async () => {
  const images = [
    'https://cdn.example.com/images/a.jpg',
    'https://cdn.example.com/images/b.png',
  ]
  globalThis.fetch = async () =>
    new Response(JSON.stringify(makePublicDetail({ images })), { status: 200 })

  const article = await getPublicNewsArticle('image-article')

  assert.deepEqual(article.images, images)
})

test('getPublicNewsArticle leaves images undefined for an empty array', async () => {
  globalThis.fetch = async () =>
    new Response(JSON.stringify(makePublicDetail({ images: [] })), { status: 200 })

  const article = await getPublicNewsArticle('image-article')

  assert.equal(article.images, undefined)
})
