import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import type { PublicNews } from '../src/api/news'
import type { Article, Category } from '../src/types'

// src/hooks/use-category-news.ts imports '@/api' and '@/api/news'; Node needs
// the alias resolved at runtime, exactly like the existing frontend tests do.
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

// ---------------------------------------------------------------------------
// Minimal hook driver
//
// The repository has no DOM renderer (no jsdom, no react-test-renderer), so a
// React component using effects cannot be mounted here. This driver executes
// the real hook module against React-compatible hook slots (useState, useRef,
// useCallback, useMemo, useEffect/useLayoutEffect, useReducer) by serving a
// local `react` module for the duration of this file. The hook itself and all
// of its collaborators are the real compiled sources; only the dispatcher is
// local. Nothing from the production code is reimplemented here.
// ---------------------------------------------------------------------------

type SetState<S> = (next: S | ((previous: S) => S)) => void

interface StateSlot {
  kind: 'state'
  value: unknown
  setter: (next: unknown) => void
}

interface RefSlot {
  kind: 'ref'
  value: { current: unknown }
}

interface MemoSlot {
  kind: 'memo'
  value: unknown
  deps?: readonly unknown[]
}

interface CallbackSlot {
  kind: 'callback'
  value: unknown
  deps?: readonly unknown[]
}

interface EffectSlot {
  kind: 'effect'
  effect: () => void | (() => void)
  deps?: readonly unknown[]
  cleanup?: () => void
}

type Slot = StateSlot | RefSlot | MemoSlot | CallbackSlot | EffectSlot

function sameDeps(
  previous: readonly unknown[] | undefined,
  next: readonly unknown[] | undefined,
): boolean {
  if (!previous || !next) return false

  return (
    previous.length === next.length && previous.every((value, index) => Object.is(value, next[index]))
  )
}

type CategoryHook = (category: Category) => unknown

let activeDriver: HookDriver | null = null

function setActiveDriver(driver: HookDriver | null): void {
  activeDriver = driver
}

class HookDriver {
  private readonly slots: Slot[] = []
  private cursor = 0
  private pendingEffects: number[] = []
  private dirty = false
  private category: Category
  result: unknown

  constructor(
    private readonly hook: CategoryHook,
    category: Category,
  ) {
    this.category = category
  }

  get needsRender(): boolean {
    return this.dirty
  }

  get hasPendingEffects(): boolean {
    return this.pendingEffects.length > 0
  }

  update(category: Category): void {
    this.category = category
    this.dirty = true
  }

  render(): void {
    this.cursor = 0
    this.pendingEffects = []
    this.dirty = false
    setActiveDriver(this)
    try {
      this.result = this.hook(this.category)
    } finally {
      setActiveDriver(null)
    }
  }

  commitEffects(): void {
    const indexes = this.pendingEffects
    this.pendingEffects = []

    for (const index of indexes) {
      const slot = this.slots[index] as EffectSlot
      if (slot.cleanup) slot.cleanup()

      const cleanup = slot.effect()
      slot.cleanup = typeof cleanup === 'function' ? cleanup : undefined
    }
  }

  useState<S>(initial: S | (() => S)): [S, SetState<S>] {
    const index = this.cursor
    this.cursor += 1
    let slot = this.slots[index]

    if (!slot || slot.kind !== 'state') {
      const value = typeof initial === 'function' ? (initial as () => S)() : initial
      const created: StateSlot = {
        kind: 'state',
        value,
        setter: (next) => {
          created.value =
            typeof next === 'function' ? (next as (previous: unknown) => unknown)(created.value) : next
          this.dirty = true
        },
      }
      slot = created
      this.slots[index] = created
    }

    const stateSlot = slot as StateSlot

    return [stateSlot.value as S, stateSlot.setter as SetState<S>]
  }

  useReducer<S, A>(
    reducer: (state: S, action: A) => S,
    initialArg: S,
    init?: (arg: S) => S,
  ): [S, (action: A) => void] {
    const index = this.cursor
    this.cursor += 1
    let slot = this.slots[index]

    if (!slot || slot.kind !== 'state') {
      const created: StateSlot = {
        kind: 'state',
        value: init ? init(initialArg) : initialArg,
        setter: () => undefined,
      }
      created.setter = (action) => {
        created.value = reducer(created.value as S, action as A)
        this.dirty = true
      }
      slot = created
      this.slots[index] = created
    }

    const stateSlot = slot as StateSlot

    return [stateSlot.value as S, stateSlot.setter as (action: A) => void]
  }

  useRef<S>(initial: S): { current: S } {
    const index = this.cursor
    this.cursor += 1
    let slot = this.slots[index]

    if (!slot || slot.kind !== 'ref') {
      slot = { kind: 'ref', value: { current: initial } }
      this.slots[index] = slot
    }

    return (slot as RefSlot).value as { current: S }
  }

  useMemo<T>(factory: () => T, deps?: readonly unknown[]): T {
    const index = this.cursor
    this.cursor += 1
    const existing = this.slots[index]

    if (!existing || existing.kind !== 'memo' || !sameDeps(existing.deps, deps)) {
      const slot: MemoSlot = { kind: 'memo', value: factory(), deps: deps ? [...deps] : undefined }
      this.slots[index] = slot

      return slot.value as T
    }

    return (existing as MemoSlot).value as T
  }

  useCallback<T extends (...args: never[]) => unknown>(callback: T, deps?: readonly unknown[]): T {
    const index = this.cursor
    this.cursor += 1
    const existing = this.slots[index]

    if (!existing || existing.kind !== 'callback' || !sameDeps(existing.deps, deps)) {
      const slot: CallbackSlot = {
        kind: 'callback',
        value: callback,
        deps: deps ? [...deps] : undefined,
      }
      this.slots[index] = slot

      return callback
    }

    return (existing as CallbackSlot).value as T
  }

  useEffect(effect: () => void | (() => void), deps?: readonly unknown[]): void {
    const index = this.cursor
    this.cursor += 1
    const existing = this.slots[index]

    if (existing && existing.kind === 'effect' && sameDeps(existing.deps, deps)) return

    const slot: EffectSlot = {
      kind: 'effect',
      effect,
      deps: deps ? [...deps] : undefined,
      cleanup: existing && existing.kind === 'effect' ? existing.cleanup : undefined,
    }
    this.slots[index] = slot
    this.pendingEffects.push(index)
  }
}

function requireDriver(): HookDriver {
  if (!activeDriver) {
    throw new Error('a React hook was called outside of the test hook driver render')
  }

  return activeDriver
}

const hookRuntime = {
  useState: <S,>(initial: S | (() => S)): [S, SetState<S>] => requireDriver().useState(initial),
  useReducer: <S, A>(
    reducer: (state: S, action: A) => S,
    initialArg: S,
    init?: (arg: S) => S,
  ): [S, (action: A) => void] => requireDriver().useReducer(reducer, initialArg, init),
  useRef: <S,>(initial: S): { current: S } => requireDriver().useRef(initial),
  useMemo: <T,>(factory: () => T, deps?: readonly unknown[]): T =>
    requireDriver().useMemo(factory, deps),
  useCallback: <T extends (...args: never[]) => unknown>(
    callback: T,
    deps?: readonly unknown[],
  ): T => requireDriver().useCallback(callback, deps),
  useEffect: (effect: () => void | (() => void), deps?: readonly unknown[]): void =>
    requireDriver().useEffect(effect, deps),
  useLayoutEffect: (effect: () => void | (() => void), deps?: readonly unknown[]): void =>
    requireDriver().useEffect(effect, deps),
}

// The compiled tests live outside the repository, where Node cannot resolve
// the frontend's `react` package. Intercepting the loader also guarantees that
// the hook module binds to this driver instead of a real React dispatcher.
interface ModuleLoader {
  _load: (request: string, parent?: unknown, isMain?: boolean) => unknown
}

const moduleLoader = Module as unknown as ModuleLoader
const originalLoad = moduleLoader._load

moduleLoader._load = function loadHookRuntime(request, parent, isMain) {
  if (request === 'react') return hookRuntime

  return originalLoad.call(moduleLoader, request, parent, isMain)
}

// ---------------------------------------------------------------------------
// Specified hook shape (not implemented yet), declared locally so this suite
// compiles while the failure stays behavioral.
// ---------------------------------------------------------------------------

interface CategoryNewsState {
  articles: Article[]
  loading: boolean
  error: string | null
  page: number
  limit: number
  hasNextPage: boolean
  retry: () => unknown
  nextPage: () => unknown
  previousPage: () => unknown
}

let useCategoryNews: CategoryHook | undefined

before(async () => {
  const modulePath: string = '../src/hooks/use-category-news'
  let hookModule: Record<string, unknown>

  try {
    hookModule = (await import(modulePath)) as Record<string, unknown>
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    assert.fail(
      'src/hooks/use-category-news must exist and export useCategoryNews, ' +
        `but importing it failed: ${reason}`,
    )

    return
  }

  const candidate = hookModule.useCategoryNews ?? hookModule.default

  assert.equal(
    typeof candidate,
    'function',
    'src/hooks/use-category-news must export useCategoryNews (named or default)',
  )
  useCategoryNews = candidate as CategoryHook
})

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  moduleLoader._load = originalLoad
})

const PUBLIC_NEWS_URL = 'http://localhost:8080/api/news'
const originalFetch = globalThis.fetch

interface PendingRequest {
  url: string
  respond: (body: unknown, status?: number) => void
  fail: (error: unknown) => void
}

let currentRequests: PendingRequest[] = []

function installDeferredFetch(): PendingRequest[] {
  const requests: PendingRequest[] = []
  currentRequests = requests

  globalThis.fetch = (input) =>
    new Promise<Response>((resolve, reject) => {
      requests.push({
        url: String(input),
        respond: (body, status = 200) =>
          resolve(new Response(JSON.stringify(body), { status })),
        fail: reject,
      })
    })

  return requests
}

afterEach(() => {
  for (const request of currentRequests) {
    request.fail(new TypeError('fetch failed'))
  }
  currentRequests = []
  globalThis.fetch = originalFetch
})

function requireHook(): CategoryHook {
  assert.ok(useCategoryNews, 'src/hooks/use-category-news must export useCategoryNews')

  return useCategoryNews
}

async function settle(driver: HookDriver): Promise<void> {
  let idleTurns = 0

  for (let turn = 0; turn < 200; turn += 1) {
    await new Promise<void>((resolve) => {
      setImmediate(resolve)
    })

    if (driver.needsRender) {
      driver.render()
      idleTurns = 0
      continue
    }

    if (driver.hasPendingEffects) {
      driver.commitEffects()
      idleTurns = 0
      continue
    }

    idleTurns += 1
    if (idleTurns >= 3) return
  }

  assert.fail('the category hook never settled: it keeps rendering without a new request')
}

function stateOf(driver: HookDriver): CategoryNewsState {
  return driver.result as CategoryNewsState
}

async function mount(category: Category): Promise<HookDriver> {
  const driver = new HookDriver(requireHook(), category)
  driver.render()
  await settle(driver)

  return driver
}

function requestParams(request: PendingRequest): URLSearchParams {
  return new URL(request.url).searchParams
}

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

function makeItems(count: number, prefix: string, category: Category): PublicNews[] {
  return Array.from({ length: count }, (_, index) =>
    makeItem({
      slug: `${prefix}-${index + 1}`,
      category,
      title: `${prefix} ${index + 1}`,
      published_at: localIsoDate(25, 12, index % 60),
    }),
  )
}

test('useCategoryNews loads the first page with the default limit of 20', async () => {
  const requests = installDeferredFetch()
  const driver = await mount('Спорт')

  assert.equal(requests.length, 1)
  const params = requestParams(requests[0])
  assert.equal(params.get('page'), '1')
  assert.equal(params.get('limit'), '20')
  assert.equal(params.get('category'), 'Спорт')
  assert.equal(requests[0].url.startsWith(PUBLIC_NEWS_URL), true)
  assert.equal(stateOf(driver).loading, true)

  requests[0].respond({
    items: [makeItem({ slug: 'sport-1', category: 'Спорт', title: 'Спортивна новина' })],
    page: 1,
    limit: 20,
  })
  await settle(driver)

  const state = stateOf(driver)
  assert.equal(state.loading, false)
  assert.equal(state.error, null)
  assert.equal(state.page, 1)
  assert.equal(state.limit, 20)
  assert.deepEqual(
    state.articles.map((article) => article.id),
    ['sport-1'],
  )
  assert.equal(state.hasNextPage, false)
})

test('useCategoryNews reports hasNextPage when a full page is returned', async () => {
  const requests = installDeferredFetch()
  const driver = await mount('Спорт')

  requests[0].respond({ items: makeItems(20, 'sport', 'Спорт'), page: 1, limit: 20 })
  await settle(driver)

  const state = stateOf(driver)
  assert.equal(state.articles.length, 20)
  assert.equal(state.hasNextPage, true)
  assert.deepEqual(state.articles[0], {
    id: 'sport-1',
    category: 'Спорт',
    title: 'sport 1',
    excerpt: 'Короткий опис',
    views: '0',
    publishedAt: '25.10.2026, 12:00',
    featured: false,
    mainEvent: false,
    imageUrl: undefined,
  })
})

test('useCategoryNews returns no articles and no next page for an empty rubric', async () => {
  const requests = installDeferredFetch()
  const driver = await mount('Спорт')

  requests[0].respond({ items: [], page: 1, limit: 20 })
  await settle(driver)

  const state = stateOf(driver)
  assert.deepEqual(state.articles, [])
  assert.equal(state.hasNextPage, false)
  assert.equal(state.error, null)
  assert.equal(state.loading, false)
})

test('useCategoryNews clears stale articles on failure and retries the same page', async () => {
  const requests = installDeferredFetch()
  const driver = await mount('Спорт')

  requests[0].respond({ items: makeItems(20, 'sport', 'Спорт'), page: 1, limit: 20 })
  await settle(driver)
  assert.equal(stateOf(driver).hasNextPage, true)

  stateOf(driver).nextPage()
  await settle(driver)
  assert.equal(requests.length, 2)
  assert.equal(requestParams(requests[1]).get('page'), '2')

  requests[1].respond({ error: { message: 'Категорію не знайдено' } }, 400)
  await settle(driver)

  let state = stateOf(driver)
  assert.equal(state.error, 'Категорію не знайдено')
  assert.deepEqual(
    state.articles,
    [],
    'a failed page must not keep showing the articles of the previous page',
  )
  assert.equal(state.loading, false)

  state.retry()
  await settle(driver)

  assert.equal(requests.length, 3)
  const retryParams = requestParams(requests[2])
  assert.equal(retryParams.get('page'), '2')
  assert.equal(retryParams.get('limit'), '20')
  assert.equal(retryParams.get('category'), 'Спорт')

  requests[2].respond({
    items: [makeItem({ slug: 'sport-page-2', category: 'Спорт' })],
    page: 2,
    limit: 20,
  })
  await settle(driver)

  state = stateOf(driver)
  assert.equal(state.error, null)
  assert.equal(state.loading, false)
  assert.deepEqual(
    state.articles.map((article) => article.id),
    ['sport-page-2'],
  )
})

test('useCategoryNews rejects a stale response from the previous category', async () => {
  const requests = installDeferredFetch()
  const driver = await mount('Спорт')
  assert.equal(requestParams(requests[0]).get('category'), 'Спорт')

  driver.update('Міжнародні новини')
  await settle(driver)

  const internationalRequest = requests[1]
  assert.ok(internationalRequest, 'expected a request for the new category')
  assert.equal(requestParams(internationalRequest).get('category'), 'Міжнародні новини')
  assert.equal(requestParams(internationalRequest).get('page'), '1')

  internationalRequest.respond({
    items: [makeItem({ slug: 'international-1', category: 'Міжнародні новини' })],
    page: 1,
    limit: 20,
  })
  await settle(driver)
  assert.deepEqual(
    stateOf(driver).articles.map((article) => article.id),
    ['international-1'],
  )

  // The first category answers after the switch: it must not replace the feed.
  requests[0].respond({
    items: [makeItem({ slug: 'sport-late', category: 'Спорт' })],
    page: 1,
    limit: 20,
  })
  await settle(driver)

  const state = stateOf(driver)
  assert.deepEqual(
    state.articles.map((article) => article.id),
    ['international-1'],
    'a late response from the previous category must not replace the current articles',
  )
  assert.equal(state.error, null)
  assert.equal(state.loading, false)
  assert.equal(state.page, 1)
})

test('useCategoryNews resets to the first page when the category changes', async () => {
  const requests = installDeferredFetch()
  const driver = await mount('Спорт')

  requests[0].respond({ items: makeItems(20, 'sport', 'Спорт'), page: 1, limit: 20 })
  await settle(driver)

  stateOf(driver).nextPage()
  await settle(driver)
  assert.equal(stateOf(driver).page, 2)
  requests[1].respond({ items: makeItems(20, 'sport-page-2', 'Спорт'), page: 2, limit: 20 })
  await settle(driver)

  driver.update('Міжнародні новини')
  await settle(driver)

  const categoryRequests = requests.slice(2)
  assert.ok(categoryRequests.length >= 1, 'expected a new request after the category changed')
  const firstPageRequest = categoryRequests.find(
    (request) =>
      requestParams(request).get('category') === 'Міжнародні новини' &&
      requestParams(request).get('page') === '1',
  )
  assert.ok(firstPageRequest, 'expected the category change to reset the page to 1')
  assert.equal(stateOf(driver).page, 1)
})

test('useCategoryNews never moves before the first page', async () => {
  const requests = installDeferredFetch()
  const driver = await mount('Спорт')

  requests[0].respond({ items: makeItems(20, 'sport', 'Спорт'), page: 1, limit: 20 })
  await settle(driver)
  assert.equal(stateOf(driver).page, 1)

  stateOf(driver).previousPage()
  await settle(driver)
  assert.equal(
    requests.length,
    1,
    'previousPage on the first page must not request another page',
  )
  assert.equal(stateOf(driver).page, 1)

  stateOf(driver).nextPage()
  await settle(driver)
  assert.equal(requests.length, 2)
  assert.equal(requestParams(requests[1]).get('page'), '2')
  requests[1].respond({ items: makeItems(20, 'sport-page-2', 'Спорт'), page: 2, limit: 20 })
  await settle(driver)
  assert.equal(stateOf(driver).page, 2)

  stateOf(driver).previousPage()
  await settle(driver)
  assert.equal(requests.length, 3)
  assert.equal(requestParams(requests[2]).get('page'), '1')
  requests[2].respond({ items: makeItems(20, 'sport-back', 'Спорт'), page: 1, limit: 20 })
  await settle(driver)
  assert.equal(stateOf(driver).page, 1)

  stateOf(driver).previousPage()
  await settle(driver)
  assert.equal(
    requests.length,
    3,
    'previousPage on the first page must stay at the lower bound',
  )
  assert.equal(stateOf(driver).page, 1)
})
