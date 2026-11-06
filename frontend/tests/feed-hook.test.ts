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
// Minimal hook driver (same approach as tests/category-news-hook.test.ts).
//
// The repository has no DOM renderer, so this driver executes the real hook
// module against React-compatible hook slots by serving a local `react` module
// for the duration of this file. Only the dispatcher is local; the hook and all
// of its collaborators are the real compiled sources.
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

// The feed page filters by a rubric or shows everything (null).
type FeedFilter = Category | null

type FeedHook = (filter: FeedFilter) => unknown

let activeDriver: HookDriver | null = null

function setActiveDriver(driver: HookDriver | null): void {
  activeDriver = driver
}

class HookDriver {
  private readonly slots: Slot[] = []
  private cursor = 0
  private pendingEffects: number[] = []
  private dirty = false
  private filter: FeedFilter
  result: unknown

  constructor(
    private readonly hook: FeedHook,
    filter: FeedFilter,
  ) {
    this.filter = filter
  }

  get needsRender(): boolean {
    return this.dirty
  }

  get hasPendingEffects(): boolean {
    return this.pendingEffects.length > 0
  }

  update(filter: FeedFilter): void {
    this.filter = filter
    this.dirty = true
  }

  render(): void {
    this.cursor = 0
    this.pendingEffects = []
    this.dirty = false
    setActiveDriver(this)
    try {
      this.result = this.hook(this.filter)
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

interface FeedNewsState {
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

let useCategoryNews: FeedHook | undefined

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
  useCategoryNews = candidate as FeedHook
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

function requireHook(): FeedHook {
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

  assert.fail('the feed hook never settled: it keeps rendering without a new request')
}

function stateOf(driver: HookDriver): FeedNewsState {
  return driver.result as FeedNewsState
}

async function mount(filter: FeedFilter): Promise<HookDriver> {
  const driver = new HookDriver(requireHook(), filter)
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

test('useCategoryNews without a filter requests the unfiltered first page', async () => {
  const requests = installDeferredFetch()
  const driver = await mount(null)

  assert.equal(requests.length, 1, 'the feed must load exactly one page on mount')
  assert.equal(
    requests[0].url,
    `${PUBLIC_NEWS_URL}?page=1&limit=20`,
    'a null filter must not append a category parameter',
  )
  const params = requestParams(requests[0])
  assert.equal(params.get('page'), '1')
  assert.equal(params.get('limit'), '20')
  assert.equal(params.has('category'), false)

  requests[0].respond({
    items: [makeItem({ slug: 'all-1', category: 'Новини', title: 'Новина без фільтра' })],
    page: 1,
    limit: 20,
  })
  await settle(driver)

  const state = stateOf(driver)
  assert.equal(state.loading, false)
  assert.equal(state.error, null)
  assert.equal(state.page, 1)
  assert.deepEqual(
    state.articles.map((article) => article.id),
    ['all-1'],
  )
})

test('useCategoryNews resets to the first page when the filter changes from null to «Спорт»', async () => {
  const requests = installDeferredFetch()
  const driver = await mount(null)

  requests[0].respond({ items: makeItems(20, 'all', 'Новини'), page: 1, limit: 20 })
  await settle(driver)
  assert.equal(stateOf(driver).hasNextPage, true)

  assert.equal(requests[0].url, `${PUBLIC_NEWS_URL}?page=1&limit=20`)

  stateOf(driver).nextPage()
  await settle(driver)
  assert.equal(requests.length, 2)
  assert.equal(requestParams(requests[1]).get('page'), '2')
  requests[1].respond({ items: makeItems(20, 'all-page-2', 'Новини'), page: 2, limit: 20 })
  await settle(driver)
  assert.equal(stateOf(driver).page, 2)

  driver.update('Спорт')
  await settle(driver)

  const categoryRequests = requests.slice(2)
  assert.ok(categoryRequests.length >= 1, 'expected a new request after the filter changed')
  const firstPageRequest = categoryRequests.find(
    (request) =>
      requestParams(request).get('category') === 'Спорт' &&
      requestParams(request).get('page') === '1',
  )
  assert.ok(firstPageRequest, 'expected the filter change to reset the page to 1')
  assert.equal(stateOf(driver).page, 1)

  firstPageRequest.respond({
    items: [makeItem({ slug: 'sport-1', category: 'Спорт' })],
    page: 1,
    limit: 20,
  })
  await settle(driver)

  const state = stateOf(driver)
  assert.deepEqual(
    state.articles.map((article) => article.id),
    ['sport-1'],
  )
  assert.equal(state.page, 1)
})
