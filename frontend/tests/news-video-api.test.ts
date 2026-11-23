import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import {
  createNews,
  MAX_VIDEO_BYTES,
  requestVideoUpload,
  updateNews,
  uploadFile,
  VIDEO_TYPES,
} from '../src/api/news'
import type { PublicNewsDetail, VideoUpload } from '../src/api/news'
import type { ArticleDetail, NewsFormValues, NewsVideo } from '../src/types'

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
const originalXMLHttpRequest = globalThis.XMLHttpRequest

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

function requireFunction<T>(fn: T | undefined, name: string): T {
  assert.equal(typeof fn, 'function', `${name} must be exported and callable`)
  return fn as T
}

function withVideo(
  values: NewsFormValues,
  video: { video_key?: string; video_url?: string },
): NewsFormValues {
  return { ...values, ...video }
}

afterEach(() => {
  globalThis.fetch = originalFetch
  globalThis.XMLHttpRequest = originalXMLHttpRequest
  FakeXMLHttpRequest.instances = []
})

test('requestVideoUpload posts the content type and size', async () => {
  const cases = [
    { type: 'video/mp4', size: 12_345 },
    { type: 'video/webm', size: 999 },
  ]

  for (const file of cases) {
    let request: { url: string; init?: RequestInit } | undefined
    const responseBody: VideoUpload = {
      key: 'pending/videos/0123456789abcdef0123456789abcdef.mp4',
      upload_url: 'https://r2.example.com/presigned-put',
      expires_at: '2026-11-23T12:00:00Z',
    }
    globalThis.fetch = async (input, init) => {
      request = { url: String(input), init }
      return new Response(JSON.stringify(responseBody), { status: 200 })
    }

    const upload = await requireFunction(requestVideoUpload, 'requestVideoUpload')(file)

    assert.equal(request?.url, '/api/admin/news/video-uploads')
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

test('requestVideoUpload throws the server error message', async () => {
  const cases = [
    { status: 400, message: 'Розмір файлу перевищує 500 МБ.' },
    { status: 413, message: 'Непідтримуваний тип відео.' },
  ]

  for (const { status, message } of cases) {
    globalThis.fetch = async () =>
      new Response(JSON.stringify({ error: { message } }), { status })

    await assert.rejects(
      requireFunction(requestVideoUpload, 'requestVideoUpload')({
        type: 'video/mp4',
        size: 600 * 1024 * 1024,
      }),
      new RegExp(message.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')),
    )
  }
})

test('requestVideoUpload falls back to a status message for non-JSON errors', async () => {
  globalThis.fetch = async () => new Response('internal failure', { status: 500 })

  await assert.rejects(
    requireFunction(requestVideoUpload, 'requestVideoUpload')({
      type: 'video/mp4',
      size: 100,
    }),
    /Помилка сервера \(500\)\./,
  )
})

test('requestVideoUpload reports network failures like other calls', async () => {
  globalThis.fetch = async () => {
    throw new TypeError('fetch failed')
  }

  await assert.rejects(
    requireFunction(requestVideoUpload, 'requestVideoUpload')({
      type: 'video/mp4',
      size: 100,
    }),
    /Не вдалося з’єднатися з сервером/,
  )
})

test('video upload limits match the specified contract', () => {
  assert.equal(
    MAX_VIDEO_BYTES,
    500 * 1024 * 1024,
    'MAX_VIDEO_BYTES must be exported from src/api/news and equal 500 MB',
  )
  assert.deepEqual(
    VIDEO_TYPES,
    ['video/mp4', 'video/webm'],
    'VIDEO_TYPES must be exported from src/api/news',
  )
})

interface FakeProgressEvent {
  lengthComputable: boolean
  loaded: number
  total: number
}

type FakeEventHandler = (event: FakeProgressEvent) => void

class FakeUploadTarget {
  private listeners = new Map<string, FakeEventHandler[]>()

  addEventListener(type: string, listener: FakeEventHandler): void {
    const current = this.listeners.get(type) ?? []
    current.push(listener)
    this.listeners.set(type, current)
  }

  removeEventListener(type: string, listener: FakeEventHandler): void {
    this.listeners.set(type, (this.listeners.get(type) ?? []).filter((item) => item !== listener))
  }

  emit(type: string, event: FakeProgressEvent): void {
    for (const listener of this.listeners.get(type) ?? []) listener(event)
  }
}

class FakeXMLHttpRequest {
  static instances: FakeXMLHttpRequest[] = []

  upload = new FakeUploadTarget()
  onload: FakeEventHandler | null = null
  onerror: FakeEventHandler | null = null
  onabort: FakeEventHandler | null = null
  method = ''
  url = ''
  requestHeaders: Record<string, string> = {}
  withCredentials = false
  sentBody: unknown = null
  aborted = false
  status = 0
  private listeners = new Map<string, FakeEventHandler[]>()

  open(method: string, url: string): void {
    this.method = method
    this.url = url
    FakeXMLHttpRequest.instances.push(this)
  }

  setRequestHeader(name: string, value: string): void {
    this.requestHeaders[name.toLowerCase()] = value
  }

  addEventListener(type: string, listener: FakeEventHandler): void {
    const current = this.listeners.get(type) ?? []
    current.push(listener)
    this.listeners.set(type, current)
  }

  removeEventListener(type: string, listener: FakeEventHandler): void {
    this.listeners.set(type, (this.listeners.get(type) ?? []).filter((item) => item !== listener))
  }

  send(body: unknown): void {
    this.sentBody = body
  }

  abort(): void {
    this.aborted = true
    const event: FakeProgressEvent = { lengthComputable: false, loaded: 0, total: 0 }
    for (const listener of this.listeners.get('abort') ?? []) listener(event)
    this.onabort?.(event)
  }

  emitUploadProgress(loaded: number, total: number, lengthComputable = true): void {
    this.upload.emit('progress', { lengthComputable, loaded, total })
  }

  complete(status: number): void {
    this.status = status
    const event: FakeProgressEvent = { lengthComputable: false, loaded: 0, total: 0 }
    for (const listener of this.listeners.get('load') ?? []) listener(event)
    this.onload?.(event)
  }

  failNetwork(): void {
    const event: FakeProgressEvent = { lengthComputable: false, loaded: 0, total: 0 }
    for (const listener of this.listeners.get('error') ?? []) listener(event)
    this.onerror?.(event)
  }
}

function installFakeXhr(): void {
  FakeXMLHttpRequest.instances = []
  globalThis.XMLHttpRequest = FakeXMLHttpRequest as unknown as typeof XMLHttpRequest
}

function currentXhr(): FakeXMLHttpRequest {
  const xhr = FakeXMLHttpRequest.instances[FakeXMLHttpRequest.instances.length - 1]
  assert.ok(xhr, 'expected uploadFile to construct an XMLHttpRequest')
  return xhr
}

function makeFileBlob(): Blob {
  return new Blob(['file-bytes'], { type: 'video/mp4' })
}

test('uploadFile PUTs the file with its content type and resolves on 2xx', async () => {
  for (const status of [200, 201, 204]) {
    installFakeXhr()
    const file = makeFileBlob()

    const pending = requireFunction(uploadFile, 'uploadFile')(
      'https://r2.example.com/presigned-put',
      file,
    )
    const xhr = currentXhr()

    assert.equal(xhr.method, 'PUT')
    assert.equal(xhr.url, 'https://r2.example.com/presigned-put')
    assert.equal(xhr.requestHeaders['content-type'], 'video/mp4')
    assert.equal(xhr.withCredentials, false)
    assert.equal(xhr.sentBody, file)

    xhr.complete(status)
    await pending
  }
})

test('uploadFile reports progress as loaded/total fractions', async () => {
  installFakeXhr()
  const fractions: number[] = []

  const pending = requireFunction(uploadFile, 'uploadFile')(
    'https://r2.example.com/presigned-put',
    makeFileBlob(),
    (fraction: number) => {
      fractions.push(fraction)
    },
  )
  const xhr = currentXhr()

  xhr.emitUploadProgress(250, 1000)
  xhr.emitUploadProgress(500, 1000)
  xhr.complete(200)
  await pending

  assert.deepEqual(fractions, [0.25, 0.5])
})

test('uploadFile ignores progress events without a computable length', async () => {
  installFakeXhr()
  const fractions: number[] = []

  const pending = requireFunction(uploadFile, 'uploadFile')(
    'https://r2.example.com/presigned-put',
    makeFileBlob(),
    (fraction: number) => {
      fractions.push(fraction)
    },
  )
  const xhr = currentXhr()

  xhr.emitUploadProgress(500, 0, false)
  xhr.complete(200)
  await pending

  assert.deepEqual(fractions, [])
})

test('uploadFile resolves without a progress callback', async () => {
  installFakeXhr()

  const pending = requireFunction(uploadFile, 'uploadFile')(
    'https://r2.example.com/presigned-put',
    makeFileBlob(),
  )
  currentXhr().complete(200)

  await pending
})

test('uploadFile rejects with the generic file message on non-2xx responses', async () => {
  for (const status of [400, 403, 500]) {
    installFakeXhr()

    const pending = requireFunction(uploadFile, 'uploadFile')(
      'https://r2.example.com/presigned-put',
      makeFileBlob(),
    )
    currentXhr().complete(status)

    await assert.rejects(pending, { message: `Не вдалося завантажити файл (${status}).` })
  }
})

test('uploadFile rejects with the generic network message', async () => {
  installFakeXhr()

  const pending = requireFunction(uploadFile, 'uploadFile')(
    'https://r2.example.com/presigned-put',
    makeFileBlob(),
  )
  currentXhr().failNetwork()

  await assert.rejects(pending, { message: 'Не вдалося завантажити файл: помилка мережі.' })
})

test('uploadFile rejects immediately when the signal is already aborted', async () => {
  installFakeXhr()
  const controller = new AbortController()
  controller.abort()

  const pending = requireFunction(uploadFile, 'uploadFile')(
    'https://r2.example.com/presigned-put',
    makeFileBlob(),
    undefined,
    controller.signal,
  )

  await assert.rejects(pending, { message: 'Завантаження скасовано.' })
})

test('uploadFile aborts the XHR when the signal fires', async () => {
  installFakeXhr()
  const controller = new AbortController()

  const pending = requireFunction(uploadFile, 'uploadFile')(
    'https://r2.example.com/presigned-put',
    makeFileBlob(),
    undefined,
    controller.signal,
  )
  const xhr = currentXhr()

  controller.abort()

  await assert.rejects(pending, /Завантаження скасовано\./)
  assert.equal(xhr.aborted, true)
})

test('createNews sends video_key and video_url in the JSON body', async () => {
  let body: unknown
  globalThis.fetch = async (input, init) => {
    assert.equal(String(input), '/api/admin/news')
    body = JSON.parse(String(init?.body)) as unknown
    return new Response(JSON.stringify({ id: '1', slug: 'test-slug' }), { status: 201 })
  }

  await createNews(
    withVideo(payload, {
      video_key: 'pending/videos/0123456789abcdef0123456789abcdef.mp4',
      video_url: '',
    }),
  )

  const parsed = body as Record<string, unknown>
  assert.equal(parsed.video_key, 'pending/videos/0123456789abcdef0123456789abcdef.mp4')
  assert.ok('video_url' in parsed, 'the create payload must contain the video_url key')
})

test('updateNews sends video_key and video_url in the JSON body', async () => {
  let body: unknown
  globalThis.fetch = async (input, init) => {
    assert.equal(String(input), '/api/admin/news/test-slug')
    body = JSON.parse(String(init?.body)) as unknown
    return new Response(JSON.stringify({ id: '1', slug: 'test-slug' }), { status: 200 })
  }

  await updateNews(
    'test-slug',
    withVideo(payload, {
      video_key: '',
      video_url: 'https://www.youtube-nocookie.com/embed/AbC123dEfG_',
    }),
  )

  const parsed = body as Record<string, unknown>
  assert.equal(parsed.video_url, 'https://www.youtube-nocookie.com/embed/AbC123dEfG_')
  assert.ok('video_key' in parsed, 'the update payload must contain the video_key key')
})

function makeDetailWithVideo(video: NewsVideo | null): PublicNewsDetail {
  return {
    slug: 'video-article',
    category: 'Новини',
    title: 'Новина з відео',
    description: 'Короткий опис',
    image_url: '',
    featured: false,
    main_event: false,
    views: 0,
    published_at: null,
    text: 'Текст новини',
    images: [],
    video,
  }
}

function articleVideo(article: ArticleDetail): NewsVideo | undefined {
  return article.video
}

test('getPublicNewsArticle maps a file video onto the article', async () => {
  const video: NewsVideo = { kind: 'file', url: 'https://cdn.example.com/videos/a.mp4' }
  globalThis.fetch = async () =>
    new Response(JSON.stringify(makeDetailWithVideo(video)), { status: 200 })

  const article = await getPublicNewsArticle('video-article')

  assert.deepEqual(articleVideo(article), video)
})

test('getPublicNewsArticle maps an embed video onto the article', async () => {
  const video: NewsVideo = {
    kind: 'embed',
    url: 'https://www.youtube-nocookie.com/embed/AbC123dEfG_',
  }
  globalThis.fetch = async () =>
    new Response(JSON.stringify(makeDetailWithVideo(video)), { status: 200 })

  const article = await getPublicNewsArticle('video-article')

  assert.deepEqual(articleVideo(article), video)
})

test('getPublicNewsArticle leaves video undefined when the response has null', async () => {
  globalThis.fetch = async () =>
    new Response(JSON.stringify(makeDetailWithVideo(null)), { status: 200 })

  const article = await getPublicNewsArticle('video-article')

  assert.equal(articleVideo(article), undefined)
})
