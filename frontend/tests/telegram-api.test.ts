import { after, afterEach, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { getTelegramChannel } from '../src/api/telegram'
import type { TelegramChannelStats } from '../src/types'

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

function makeChannelStats(overrides: Partial<TelegramChannelStats> = {}): TelegramChannelStats {
  return {
    subscribers: 3846,
    updated_at: '2026-11-23T10:00:00Z',
    ...overrides,
  }
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

test('getTelegramChannel requests the channel endpoint with credentials and resolves the parsed body', async () => {
  const stats = makeChannelStats()
  const captured = captureJsonResponse(stats)

  const result = await getTelegramChannel()

  assert.equal(captured.length, 1)
  assert.equal(captured[0]?.url, '/api/telegram/channel')
  assert.equal(captured[0]?.init?.credentials, 'include')
  assert.deepEqual(result, stats)
})

test('getTelegramChannel uses the backend error message', async () => {
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ error: { message: 'Telegram stats unavailable' } }), { status: 503 })

  await assert.rejects(getTelegramChannel(), /Telegram stats unavailable/)
})

test('getTelegramChannel falls back to the status message for non-JSON errors', async () => {
  globalThis.fetch = async () => new Response('internal failure', { status: 500 })

  await assert.rejects(getTelegramChannel(), /Помилка сервера \(500\)\./)
})

test('getTelegramChannel reports network failures', async () => {
  globalThis.fetch = async () => {
    throw new TypeError('fetch failed')
  }

  await assert.rejects(getTelegramChannel(), /Не вдалося з’єднатися з сервером/)
})

const malformedChannelBodies: Array<{ name: string; body: unknown }> = [
  { name: 'missing subscribers', body: { updated_at: '2026-11-23T10:00:00Z' } },
  { name: 'string subscribers', body: { subscribers: '3846', updated_at: '2026-11-23T10:00:00Z' } },
  { name: 'null subscribers', body: { subscribers: null, updated_at: '2026-11-23T10:00:00Z' } },
]

for (const { name, body } of malformedChannelBodies) {
  test(`getTelegramChannel rejects a 200 response with ${name}`, async () => {
    captureJsonResponse(body)

    await assert.rejects(getTelegramChannel(), /Некоректна відповідь сервера\./)
  })
}
