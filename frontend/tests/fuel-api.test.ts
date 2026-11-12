import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

// The fuel API and hook import through the '@/' alias; resolve it at runtime
// exactly like the existing suites do.
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
// tsx runs this file as ESM, where __dirname is undefined; the compiled run keeps it.
const compiledSrcRoot =
  typeof __dirname === 'string'
    ? path.resolve(__dirname, '..', 'src')
    : path.resolve(process.cwd(), 'src')

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

// Minimal hook driver serving React-compatible slots to the real hook module.

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

type Slot = StateSlot | RefSlot | CallbackSlot | EffectSlot

function sameDeps(
  previous: readonly unknown[] | undefined,
  next: readonly unknown[] | undefined,
): boolean {
  if (!previous || !next) return false

  return (
    previous.length === next.length && previous.every((value, index) => Object.is(value, next[index]))
  )
}

interface FuelPricesState {
  rates: FuelRateItem[]
  date: string
  loading: boolean
  error: string | null
  retry: () => void
}

type FuelHook = () => FuelPricesState

let activeDriver: HookDriver | null = null

function setActiveDriver(driver: HookDriver | null): void {
  activeDriver = driver
}

class HookDriver {
  private readonly slots: Slot[] = []
  private cursor = 0
  private pendingEffects: number[] = []
  private dirty = false
  result: unknown

  constructor(private readonly hook: FuelHook) {}

  get needsRender(): boolean {
    return this.dirty
  }

  get hasPendingEffects(): boolean {
    return this.pendingEffects.length > 0
  }

  render(): void {
    this.cursor = 0
    this.pendingEffects = []
    this.dirty = false
    setActiveDriver(this)
    try {
      this.result = this.hook()
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
  useRef: <S,>(initial: S): { current: S } => requireDriver().useRef(initial),
  useCallback: <T extends (...args: never[]) => unknown>(
    callback: T,
    deps?: readonly unknown[],
  ): T => requireDriver().useCallback(callback, deps),
  useEffect: (effect: () => void | (() => void), deps?: readonly unknown[]): void =>
    requireDriver().useEffect(effect, deps),
}

// The CJS loader hook binds the hook module to this driver; the ESM loader below
// does the same when tsx loads modules as ESM.
interface ModuleLoader {
  _load: (request: string, parent?: unknown, isMain?: boolean) => unknown
}

const moduleLoader = Module as unknown as ModuleLoader
const originalLoad = moduleLoader._load

moduleLoader._load = function loadHookRuntime(request, parent, isMain) {
  if (request === 'react') return hookRuntime

  return originalLoad.call(moduleLoader, request, parent, isMain)
}

const hookRuntimeGlobal = globalThis as unknown as { __fuelHookRuntime?: typeof hookRuntime }
hookRuntimeGlobal.__fuelHookRuntime = hookRuntime

const reactStubUrl =
  'data:text/javascript,' +
  encodeURIComponent(
    [
      'const runtime = globalThis.__fuelHookRuntime',
      'export const useState = runtime.useState',
      'export const useRef = runtime.useRef',
      'export const useCallback = runtime.useCallback',
      'export const useEffect = runtime.useEffect',
      'export default runtime',
    ].join('\n'),
  )

const loaderSource = [
  'export async function resolve(specifier, context, nextResolve) {',
  `  if (specifier === 'react') return { url: ${JSON.stringify(reactStubUrl)}, shortCircuit: true }`,
  '  return nextResolve(specifier, context)',
  '}',
].join('\n')

interface ModuleRegistrar {
  register?: (specifier: string) => void
}

const moduleRegistrar = Module as unknown as ModuleRegistrar
moduleRegistrar.register?.('data:text/javascript,' + encodeURIComponent(loaderSource))

interface FuelRateItem {
  code: string
  rate: number
}

interface FuelRatesResponse {
  base: string
  unit: string
  date: string
  items: FuelRateItem[]
}

type FetchFuelRates = () => Promise<FuelRatesResponse>

let fetchFuelRates: FetchFuelRates | undefined
let useFuelPrices: FuelHook | undefined

before(async () => {
  let fuelModule: Record<string, unknown>
  const modulePath: string = '../src/api/fuel'

  try {
    fuelModule = (await import(modulePath)) as Record<string, unknown>
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    assert.fail(
      'src/api/fuel must exist and export fetchFuelRates, ' +
        `but importing it failed: ${reason}`,
    )
  }

  const fetchFn = fuelModule.fetchFuelRates

  assert.equal(typeof fetchFn, 'function', 'src/api/fuel must export fetchFuelRates')

  fetchFuelRates = fetchFn as FetchFuelRates
})

before(async () => {
  let hookModule: Record<string, unknown>
  const modulePath: string = '../src/hooks/use-fuel-prices'

  try {
    hookModule = (await import(modulePath)) as Record<string, unknown>
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    assert.fail(
      'src/hooks/use-fuel-prices must exist and export useFuelPrices, ' +
        `but importing it failed: ${reason}`,
    )
  }

  const candidate = hookModule.default ?? hookModule.useFuelPrices

  assert.equal(
    typeof candidate,
    'function',
    'src/hooks/use-fuel-prices must export useFuelPrices (default or named)',
  )
  useFuelPrices = candidate as FuelHook
})

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  moduleLoader._load = originalLoad
})

const FUEL_RATES_URL = 'http://localhost:8080/api/fuel/rates'
const CONNECTION_ERROR =
  'Не вдалося з’єднатися з сервером. Перевірте, чи запущено backend.'
const originalFetch = globalThis.fetch

function stubJsonFetch(body: unknown): string[] {
  const urls: string[] = []
  globalThis.fetch = ((input: RequestInfo | URL) => {
    urls.push(String(input))

    return Promise.resolve({
      ok: true,
      status: 200,
      text: async () => JSON.stringify(body),
    } as Response)
  }) as typeof fetch

  return urls
}

function stubFailingFetch(): void {
  globalThis.fetch = (() =>
    Promise.reject(new TypeError('fetch failed'))) as unknown as typeof fetch
}

afterEach(() => {
  globalThis.fetch = originalFetch
})

function requireApi(): { fetchFuelRates: FetchFuelRates } {
  assert.ok(fetchFuelRates, 'src/api/fuel must export fetchFuelRates')

  return { fetchFuelRates }
}

function requireHook(): FuelHook {
  assert.ok(useFuelPrices, 'src/hooks/use-fuel-prices must export useFuelPrices')

  return useFuelPrices
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

  assert.fail('the fuel prices hook never settled: it keeps rendering without a new request')
}

async function mount(): Promise<HookDriver> {
  const driver = new HookDriver(requireHook())
  driver.render()
  await settle(driver)

  return driver
}

function stateOf(driver: HookDriver): FuelPricesState {
  return driver.result as FuelPricesState
}

test('fetchFuelRates maps items', async () => {
  const requestUrls = stubJsonFetch({
    base: 'UAH',
    unit: 'liter',
    date: '11.11.2026',
    items: [{ code: 'A95', rate: 88.74 }],
  })

  const result = await requireApi().fetchFuelRates()

  assert.deepEqual(result.items, [{ code: 'A95', rate: 88.74 }])
  assert.equal(result.base, 'UAH')
  assert.equal(result.unit, 'liter')
  assert.equal(result.date, '11.11.2026')
  assert.equal(requestUrls[0], FUEL_RATES_URL)
})

test('fetchFuelRates throws connection error when backend is down', async () => {
  stubFailingFetch()

  await assert.rejects(requireApi().fetchFuelRates(), (error: unknown) => {
    assert.ok(error instanceof Error, 'the request must reject with an Error')
    assert.equal(error.message, CONNECTION_ERROR)

    return true
  })
})

test('useFuelPrices returns raw rates in response order', async () => {
  stubJsonFetch({
    base: 'UAH',
    unit: 'liter',
    date: '11.11.2026',
    items: [
      { code: 'A95', rate: 88.74 },
      { code: 'A92', rate: 84.19 },
      { code: 'A98', rate: 95.6 },
      { code: 'DIESEL', rate: 92.35 },
      { code: 'LPG', rate: 41.07 },
    ],
  })

  const driver = await mount()

  assert.deepEqual(stateOf(driver).rates, [
    { code: 'A95', rate: 88.74 },
    { code: 'A92', rate: 84.19 },
    { code: 'A98', rate: 95.6 },
    { code: 'DIESEL', rate: 92.35 },
    { code: 'LPG', rate: 41.07 },
  ])
})

test('useFuelPrices returns empty rates when rates fail', async () => {
  stubFailingFetch()

  const driver = await mount()
  const state = stateOf(driver)

  assert.deepEqual(state.rates, [])
  assert.equal(typeof state.error, 'string')
  assert.ok(state.error && state.error.length > 0, 'the hook must report a non-empty error message')
})
