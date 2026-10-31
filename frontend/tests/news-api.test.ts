import { afterEach, test } from 'node:test'
import assert from 'node:assert/strict'

import {
  createNews,
  deleteNews,
  getNewsErrorMessage,
  listNews,
  listPublicNews,
  updateNews,
} from '../src/api/news'
import type { PublicNews, PublicNewsListResponse } from '../src/api/news'
import { validateNewsPayload } from '../src/lib/news-validation'
import type { NewsFormValues } from '../src/types'

const originalFetch = globalThis.fetch

const payload: NewsFormValues = {
  category: 'Новини',
  title: 'Тестова новина',
  description: 'Короткий опис',
  text: 'Повний текст',
  language: 'uk',
  status: 'draft',
  featured: false,
  main_event: true,
  image_url: '',
}

const publicNewsItem: PublicNews = {
  slug: 'main-event-news',
  category: 'Новини',
  title: 'Головна подія дня',
  description: 'Короткий опис головної події',
  image_url: 'https://cdn.example.com/main-event.jpg',
  featured: false,
  main_event: true,
  views: 120,
  published_at: '2026-10-25T11:35:00Z',
}

afterEach(() => {
  globalThis.fetch = originalFetch
})

test('listNews builds pagination and status query', async () => {
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items: [], page: 2, limit: 20 }), { status: 200 })
  }

  const result = await listNews(2, 20, 'published')

  assert.equal(
    requestedUrl,
    'http://localhost:8080/api/admin/news?page=2&limit=20&status=published',
  )
  assert.deepEqual(result, { items: [], page: 2, limit: 20 })
})

test('listNews omits an empty status filter', async () => {
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items: [], page: 1, limit: 20 }), { status: 200 })
  }

  await listNews(1, 20)

  assert.equal(requestedUrl, 'http://localhost:8080/api/admin/news?page=1&limit=20')
})

test('createNews sends a POST JSON payload', async () => {
  let request: { url: string; init?: RequestInit } | undefined
  globalThis.fetch = async (input, init) => {
    request = { url: String(input), init }
    return new Response(JSON.stringify({ id: '1', ...payload }), { status: 201 })
  }

  await createNews(payload)

  assert.equal(request?.url, 'http://localhost:8080/api/admin/news')
  assert.equal(request?.init?.method, 'POST')
  assert.deepEqual(request?.init?.headers, { 'Content-Type': 'application/json' })
  assert.equal(request?.init?.body, JSON.stringify(payload))

  const body = JSON.parse(String(request?.init?.body)) as Record<string, unknown>
  const mainEvent: boolean = payload.main_event
  assert.equal(mainEvent, true, 'NewsFormValues must require main_event')
  assert.equal(body.main_event, true, 'the create payload must serialise main_event')
  assert.ok('main_event' in body, 'the create payload must contain the main_event key')
})

test('updateNews encodes the slug and sends PUT', async () => {
  let request: { url: string; init?: RequestInit } | undefined
  globalThis.fetch = async (input, init) => {
    request = { url: String(input), init }
    return new Response(JSON.stringify({ id: '1', ...payload }), { status: 200 })
  }

  await updateNews('новина/1', payload)

  assert.equal(
    request?.url,
    'http://localhost:8080/api/admin/news/%D0%BD%D0%BE%D0%B2%D0%B8%D0%BD%D0%B0%2F1',
  )
  assert.equal(request?.init?.method, 'PUT')

  const body = JSON.parse(String(request?.init?.body)) as Record<string, unknown>
  assert.equal(body.main_event, true, 'the update payload must serialise main_event')
  assert.ok('main_event' in body, 'the update payload must contain the main_event key')
})

test('listPublicNews preserves the required main_event flag', async () => {
  const responseBody: PublicNewsListResponse = { items: [publicNewsItem], page: 1, limit: 20 }
  globalThis.fetch = async () =>
    new Response(JSON.stringify(responseBody), { status: 200 })

  const result = await listPublicNews()

  const mainEvent: boolean = publicNewsItem.main_event
  assert.equal(mainEvent, true, 'PublicNews must require main_event')
  assert.equal(result.items[0].main_event, true)
  assert.ok('main_event' in result.items[0], 'the public contract must expose main_event')
})

test('deleteNews sends DELETE and accepts an empty response', async () => {
  let request: { url: string; init?: RequestInit } | undefined
  globalThis.fetch = async (input, init) => {
    request = { url: String(input), init }
    return new Response(null, { status: 200 })
  }

  await deleteNews('test-slug')

  assert.equal(request?.url, 'http://localhost:8080/api/admin/news/test-slug')
  assert.equal(request?.init?.method, 'DELETE')
})

test('API errors use the backend message', async () => {
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ error: { message: 'News article not found' } }), { status: 404 })

  await assert.rejects(listNews(), /News article not found/)
})

test('network failures have a useful Ukrainian message', async () => {
  globalThis.fetch = async () => {
    throw new TypeError('fetch failed')
  }

  await assert.rejects(listNews(), /Не вдалося з’єднатися з сервером/)
})

test('getNewsErrorMessage handles unknown values', () => {
  assert.equal(getNewsErrorMessage(null), 'Сталася невідома помилка. Спробуйте ще раз.')
  assert.equal(getNewsErrorMessage(new Error('Помилка')), 'Помилка')
})

test('validation rejects a blank title', () => {
  const errors = validateNewsPayload({ ...payload, title: '   ' })

  assert.equal(errors.title, 'Введіть заголовок новини.')
})

test('validation accepts a draft without text', () => {
  assert.deepEqual(validateNewsPayload({ ...payload, text: '' }), {})
})

test('validation requires text for published news', () => {
  const errors = validateNewsPayload({ ...payload, status: 'published', text: '  ' })

  assert.equal(errors.text, 'Додайте текст перед публікацією.')
})

test('validation limits the language code to ten characters', () => {
  const errors = validateNewsPayload({ ...payload, language: '12345678901' })

  assert.equal(errors.language, 'Код мови не може бути довшим за 10 символів.')
})

test('validation rejects an invalid cover URL', () => {
  const errors = validateNewsPayload({ ...payload, image_url: 'not-a-url' })

  assert.equal(errors.image_url, 'Введіть коректне посилання, яке починається з http:// або https://.')
})
