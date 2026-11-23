import { after, afterEach, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import {
  createContact,
  deleteContact,
  listAdminContacts,
  listContacts,
  moveContact,
  updateContact,
} from '../src/api/contacts'
import type { Contact, ContactFormValues, ContactMoveDirection } from '../src/types'

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

function makeContact(overrides: Partial<Contact> = {}): Contact {
  return {
    id: 'contact-1',
    type: 'telegram',
    value: 'mandat',
    label: 'Підтримка',
    display: '@mandat',
    href: 'https://t.me/mandat',
    position: 1,
    ...overrides,
  }
}

function makeValues(overrides: Partial<ContactFormValues> = {}): ContactFormValues {
  return {
    type: 'telegram',
    value: 'mandat',
    label: 'Підтримка',
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

function contentTypeOf(init?: RequestInit): string | null {
  return new Headers(init?.headers).get('content-type')
}

function jsonBody(init?: RequestInit): unknown {
  assert.equal(typeof init?.body, 'string', `expected a JSON string body, got: ${typeof init?.body}`)

  return JSON.parse(init?.body as string)
}

test('listContacts requests the public endpoint and resolves the items', async () => {
  const contact = makeContact()
  const captured = captureJsonResponse({ items: [contact] })

  const result = await listContacts()

  assert.equal(captured.length, 1)
  assert.equal(captured[0]?.url, '/api/contacts')
  assert.equal(captured[0]?.init?.credentials, 'include')
  assert.deepEqual(result, [contact])
})

test('listAdminContacts requests the admin endpoint with credentials', async () => {
  const contact = makeContact()
  const captured = captureJsonResponse({ items: [contact] })

  const result = await listAdminContacts()

  assert.equal(captured.length, 1)
  assert.equal(captured[0]?.url, '/api/admin/contacts')
  assert.equal(captured[0]?.init?.credentials, 'include')
  assert.deepEqual(result, [contact])
})

test('createContact posts the form values as JSON', async () => {
  const contact = makeContact()
  const values = makeValues()
  const captured = captureJsonResponse(contact, 201)

  const result = await createContact(values)

  assert.equal(captured[0]?.url, '/api/admin/contacts')
  assert.equal(captured[0]?.init?.method, 'POST')
  assert.equal(captured[0]?.init?.credentials, 'include')
  assert.deepEqual(captured[0]?.init?.headers, { 'Content-Type': 'application/json' })
  assert.equal(contentTypeOf(captured[0]?.init), 'application/json')
  assert.deepEqual(jsonBody(captured[0]?.init), { type: values.type, value: values.value, label: values.label })
  assert.deepEqual(result, contact)
})

test('updateContact puts the form values as JSON to the encoded id', async () => {
  const contact = makeContact()
  const values = makeValues({ type: 'email', value: 'mandat@gmail.com', label: 'Редакція' })
  const captured = captureJsonResponse(contact)

  const result = await updateContact('contact/1 2', values)

  assert.equal(captured[0]?.url, '/api/admin/contacts/contact%2F1%202')
  assert.equal(captured[0]?.init?.method, 'PUT')
  assert.equal(captured[0]?.init?.credentials, 'include')
  assert.deepEqual(captured[0]?.init?.headers, { 'Content-Type': 'application/json' })
  assert.deepEqual(jsonBody(captured[0]?.init), { type: values.type, value: values.value, label: values.label })
  assert.deepEqual(result, contact)
})

test('deleteContact sends DELETE to the encoded admin endpoint', async () => {
  let request: { url: string; init?: RequestInit } | undefined
  globalThis.fetch = async (input, init) => {
    request = { url: String(input), init }
    return new Response(null, { status: 200 })
  }

  await deleteContact('contact-1')

  assert.equal(request?.url, '/api/admin/contacts/contact-1')
  assert.equal(request?.init?.method, 'DELETE')
  assert.equal(request?.init?.credentials, 'include')
})

test('moveContact posts the direction as JSON', async () => {
  const directions: ContactMoveDirection[] = ['up', 'down']

  for (const direction of directions) {
    let request: { url: string; init?: RequestInit } | undefined
    globalThis.fetch = async (input, init) => {
      request = { url: String(input), init }
      return new Response(null, { status: 200 })
    }

    await moveContact('contact-1', direction)

    assert.equal(request?.url, '/api/admin/contacts/contact-1/move', `expected the move endpoint for ${direction}`)
    assert.equal(request?.init?.method, 'POST', `expected a POST request for ${direction}`)
    assert.equal(request?.init?.credentials, 'include', `expected credentials for ${direction}`)
    assert.deepEqual(request?.init?.headers, { 'Content-Type': 'application/json' })
    assert.equal(request?.init?.body, JSON.stringify({ direction }))
  }
})

test('contact requests use the backend error message', async () => {
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ error: { message: 'Contact not found' } }), { status: 404 })

  await assert.rejects(listContacts(), /Contact not found/)
})

test('contact requests fall back to the status message for non-JSON errors', async () => {
  globalThis.fetch = async () => new Response('internal failure', { status: 500 })

  await assert.rejects(listContacts(), /Помилка сервера \(500\)\./)
})

test('contact requests report network failures', async () => {
  globalThis.fetch = async () => {
    throw new TypeError('fetch failed')
  }

  await assert.rejects(listContacts(), /Не вдалося з’єднатися з сервером/)
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

    await assert.rejects(listAdminContacts(), /Unauthorized/)
    assert.deepEqual(dispatched, ['mandat:unauthorized'])
  } finally {
    delete (globalThis as { window?: unknown }).window
  }
})

test('a 401 from the public endpoint does not dispatch mandat:unauthorized', async () => {
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

    await assert.rejects(listContacts(), /Unauthorized/)
    assert.deepEqual(dispatched, [])
  } finally {
    delete (globalThis as { window?: unknown }).window
  }
})
