import { afterEach, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { searchPublicNews } from '../src/api/news'
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

afterEach(() => {
  globalThis.fetch = originalFetch
})

test('searchPublicNews builds the search URL with encoded q and default limit 7', async () => {
  const item = makeItem({ slug: 'kyiv-1', title: 'Новини Києва' })
  const responseBody: PublicNewsListResponse = { items: [item], page: 1, limit: 7 }
  let requestedUrl = ''
  let requestedInit: RequestInit | undefined
  globalThis.fetch = async (input, init) => {
    requestedUrl = String(input)
    requestedInit = init
    return new Response(JSON.stringify(responseBody), { status: 200 })
  }

  const result = await searchPublicNews('Київ')

  assert.ok(
    requestedUrl.endsWith('/search?q=%D0%9A%D0%B8%D1%97%D0%B2&limit=7'),
    `expected the search URL to end with the encoded query and the default limit, got ${requestedUrl}`,
  )
  assert.equal(requestedInit?.credentials, 'include', 'expected the search request to send credentials')
  assert.deepEqual(result, responseBody)
})

test('searchPublicNews passes an explicit limit', async () => {
  const responseBody: PublicNewsListResponse = { items: [], page: 1, limit: 3 }
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify(responseBody), { status: 200 })
  }

  const result = await searchPublicNews('тест', 3)

  assert.ok(
    requestedUrl.endsWith('/search?q=%D1%82%D0%B5%D1%81%D1%82&limit=3'),
    `expected the explicit limit in the search URL, got ${requestedUrl}`,
  )
  assert.deepEqual(result, responseBody)
})

test('searchPublicNews surfaces the server error message', async () => {
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ error: { message: 'Invalid input' } }), { status: 400 })

  await assert.rejects(searchPublicNews('Київ'), (error: unknown) => {
    assert.ok(error instanceof Error, 'expected the search client to reject with an Error')
    assert.equal(error.message, 'Invalid input')
    return true
  })
})
