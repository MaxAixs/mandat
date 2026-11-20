import { after, afterEach, test } from 'node:test'
import assert from 'node:assert/strict'
import { File as NodeFile } from 'node:buffer'
import Module from 'node:module'
import path from 'node:path'

import {
  createAdFormat,
  deleteAdFormat,
  listAdFormats,
  listAdminAdFormats,
  moveAdFormat,
  updateAdFormat,
} from '../src/api/adFormats'
import type { AdFormat, AdFormatFormValues, AdFormatMoveDirection } from '../src/types'

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the other frontend suites do.
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

afterEach(() => {
  globalThis.fetch = originalFetch
})

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
})

function makeCard(overrides: Partial<AdFormat> = {}): AdFormat {
  return {
    id: 'format-1',
    title: 'Спецпроєкти',
    description: 'Окрема сторінка з власним дизайном',
    image_url: 'https://cdn.example.com/format.jpg',
    position: 1,
    ...overrides,
  }
}

function makeImageFile(name = 'card.jpg'): File {
  return new NodeFile(['image-bytes'], name, { type: 'image/jpeg' }) as unknown as File
}

interface CapturedRequest {
  url: string
  init?: RequestInit
}

function captureJsonResponse(body: unknown, status = 200): CapturedRequest[] {
  const captured: CapturedRequest[] = []
  globalThis.fetch = async (input, init) => {
    captured.push({ url: String(input), init })
    return new Response(JSON.stringify(body), { status })
  }

  return captured
}

function contentTypeOf(init?: RequestInit): string | null {
  return new Headers(init?.headers).get('content-type')
}

function formBody(init?: RequestInit): FormData {
  const body = init?.body as unknown as FormData
  assert.ok(body instanceof FormData, `expected a FormData body, got: ${typeof init?.body}`)

  return body
}

test('listAdFormats requests the public endpoint and resolves the items', async () => {
  const card = makeCard()
  const captured = captureJsonResponse({ items: [card] })

  const result = await listAdFormats()

  assert.equal(captured.length, 1)
  assert.equal(captured[0]?.url, '/api/ad-formats')
  assert.deepEqual(result, [card])
})

test('listAdminAdFormats requests the admin endpoint with credentials', async () => {
  const card = makeCard()
  const captured = captureJsonResponse({ items: [card] })

  const result = await listAdminAdFormats()

  assert.equal(captured.length, 1)
  assert.equal(captured[0]?.url, '/api/admin/ad-formats')
  assert.equal(captured[0]?.init?.credentials, 'include')
  assert.deepEqual(result, [card])
})

test('createAdFormat posts multipart form data without a manual Content-Type', async () => {
  const card = makeCard()
  const captured = captureJsonResponse(card, 201)

  const result = await createAdFormat({
    title: 'Новий формат',
    description: 'Опис нового формату',
    image: makeImageFile(),
  })

  assert.equal(captured[0]?.url, '/api/admin/ad-formats')
  assert.equal(captured[0]?.init?.method, 'POST')
  assert.equal(captured[0]?.init?.credentials, 'include')
  assert.equal(contentTypeOf(captured[0]?.init), null, 'the browser must set the multipart boundary')

  const body = formBody(captured[0]?.init)
  assert.equal(body.get('title'), 'Новий формат')
  assert.equal(body.get('description'), 'Опис нового формату')
  assert.equal((body.get('image') as File).name, 'card.jpg')
  assert.deepEqual(result, card)
})

test('createAdFormat omits the image part when no file is given', async () => {
  const card = makeCard({ image_url: '' })
  const captured = captureJsonResponse(card, 201)

  const result = await createAdFormat({ title: 'Новий формат', description: 'Опис нового формату' })

  assert.equal(captured[0]?.url, '/api/admin/ad-formats')
  assert.equal(captured[0]?.init?.method, 'POST')
  assert.equal(contentTypeOf(captured[0]?.init), null, 'the browser must set the multipart boundary')

  const body = formBody(captured[0]?.init)
  assert.equal(body.get('title'), 'Новий формат')
  assert.equal(body.get('description'), 'Опис нового формату')
  assert.equal(body.has('image'), false, 'expected no image part without a file')
  assert.deepEqual(result, card)
})

test('updateAdFormat puts multipart form data to the encoded id', async () => {
  const card = makeCard()
  const cases: Array<{
    name: string
    values: AdFormatFormValues
    expectedImage: string | null
    expectedRemoveImage: string | null
  }> = [
    {
      name: 'attaches the image file when given',
      values: { title: 'Формат', description: 'Опис', image: makeImageFile('replacement.png') },
      expectedImage: 'replacement.png',
      expectedRemoveImage: null,
    },
    {
      name: 'omits the image part when no file is given',
      values: { title: 'Формат', description: 'Опис' },
      expectedImage: null,
      expectedRemoveImage: null,
    },
    {
      name: 'sends remove_image=true only when removal is requested',
      values: { title: 'Формат', description: 'Опис', removeImage: true },
      expectedImage: null,
      expectedRemoveImage: 'true',
    },
    {
      name: 'omits remove_image when removal is not requested',
      values: { title: 'Формат', description: 'Опис', removeImage: false },
      expectedImage: null,
      expectedRemoveImage: null,
    },
  ]

  for (const { name, values, expectedImage, expectedRemoveImage } of cases) {
    const captured = captureJsonResponse(card)

    const result = await updateAdFormat('format/1 2', values)

    assert.equal(
      captured[0]?.url,
      '/api/admin/ad-formats/format%2F1%202',
      `${name}: expected the id to be encoded`,
    )
    assert.equal(captured[0]?.init?.method, 'PUT', `${name}: expected a PUT request`)
    assert.equal(contentTypeOf(captured[0]?.init), null, `${name}: the browser must set the multipart boundary`)

    const body = formBody(captured[0]?.init)
    assert.equal(body.get('title'), values.title, `${name}: expected the title field`)
    assert.equal(body.get('description'), values.description, `${name}: expected the description field`)
    if (expectedImage === null) {
      assert.equal(body.has('image'), false, `${name}: expected no image part`)
    } else {
      assert.equal((body.get('image') as File).name, expectedImage, `${name}: expected the image file`)
    }
    if (expectedRemoveImage === null) {
      assert.equal(body.has('remove_image'), false, `${name}: expected no remove_image part`)
    } else {
      assert.equal(body.get('remove_image'), 'true', `${name}: expected remove_image=true`)
    }
    assert.deepEqual(result, card, `${name}: expected the updated card`)
  }
})

test('deleteAdFormat sends DELETE to the admin endpoint', async () => {
  let request: { url: string; init?: RequestInit } | undefined
  globalThis.fetch = async (input, init) => {
    request = { url: String(input), init }
    return new Response(null, { status: 200 })
  }

  await deleteAdFormat('format-1')

  assert.equal(request?.url, '/api/admin/ad-formats/format-1')
  assert.equal(request?.init?.method, 'DELETE')
})

test('moveAdFormat posts the direction as JSON', async () => {
  const directions: AdFormatMoveDirection[] = ['up', 'down']

  for (const direction of directions) {
    let request: { url: string; init?: RequestInit } | undefined
    globalThis.fetch = async (input, init) => {
      request = { url: String(input), init }
      return new Response(null, { status: 200 })
    }

    await moveAdFormat('format-1', direction)

    assert.equal(request?.url, '/api/admin/ad-formats/format-1/move', `expected the move endpoint for ${direction}`)
    assert.equal(request?.init?.method, 'POST', `expected a POST request for ${direction}`)
    assert.deepEqual(request?.init?.headers, { 'Content-Type': 'application/json' })
    assert.equal(request?.init?.body, JSON.stringify({ direction }))
  }
})

test('ad format requests use the backend error message', async () => {
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ error: { message: 'Ad format not found' } }), { status: 404 })

  await assert.rejects(listAdFormats(), /Ad format not found/)
})

test('ad format requests fall back to the status message for non-JSON errors', async () => {
  globalThis.fetch = async () => new Response('internal failure', { status: 500 })

  await assert.rejects(listAdFormats(), /Помилка сервера \(500\)\./)
})

test('ad format requests report network failures', async () => {
  globalThis.fetch = async () => {
    throw new TypeError('fetch failed')
  }

  await assert.rejects(listAdFormats(), /Не вдалося з’єднатися з сервером/)
})

test('a 401 from an admin endpoint dispatches mandat:unauthorized', async () => {
  const dispatched: string[] = []
  const windowStub = {
    dispatchEvent: (event: Event): boolean => {
      dispatched.push(event.type)
      return true
    },
  }
  ;(globalThis as { window?: unknown }).window = windowStub
  try {
    globalThis.fetch = async () =>
      new Response(JSON.stringify({ error: { message: 'Unauthorized' } }), { status: 401 })

    await assert.rejects(listAdminAdFormats(), /Unauthorized/)
    assert.deepEqual(dispatched, ['mandat:unauthorized'])
  } finally {
    delete (globalThis as { window?: unknown }).window
  }
})
