import { after, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

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

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
})

// The subjects load lazily: compiled src modules keep the '@/...' alias, so
// they must be required after the resolver above is installed (static imports
// would run first and fail on the alias).
type AdvertisingConstants = typeof import('../src/constants/advertising')

let ADVERTISING_FORMATS_TITLE!: string
let ADVERTISING_FORMATS_LEAD!: string
let ADVERTISING_ORDER_LABEL!: string
let Advertising!: ComponentType

before(async () => {
  const advertisingConstants: AdvertisingConstants = await import('../src/constants/advertising')
  ADVERTISING_FORMATS_TITLE = advertisingConstants.ADVERTISING_FORMATS_TITLE
  ADVERTISING_FORMATS_LEAD = advertisingConstants.ADVERTISING_FORMATS_LEAD
  ADVERTISING_ORDER_LABEL = advertisingConstants.ADVERTISING_ORDER_LABEL
  ;({ default: Advertising } = await import('../src/pages/Advertising'))
})

function renderAdvertising(): string {
  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/advertising']}>
      <Advertising />
    </MemoryRouter>,
  )
}

test('Advertising page shows the formats heading with its lead', () => {
  const html = renderAdvertising()

  assert.ok(
    html.includes(ADVERTISING_FORMATS_TITLE),
    `expected the formats heading «${ADVERTISING_FORMATS_TITLE}» in:\n${html}`,
  )
  assert.ok(
    html.includes(ADVERTISING_FORMATS_LEAD),
    `expected the formats lead in:\n${html}`,
  )
})

test('Advertising page shows the order call to action while loading', () => {
  const html = renderAdvertising()

  assert.ok(
    html.includes(ADVERTISING_ORDER_LABEL),
    `expected the «${ADVERTISING_ORDER_LABEL}» call to action in:\n${html}`,
  )
})

test('Advertising page shows no format cards before the API answers', () => {
  const html = renderAdvertising()

  // Cards are the only links carrying an aria-label: each one is labelled
  // `${ADVERTISING_ORDER_LABEL}: ${format.title}`. The formats grid starts
  // empty (useEffect has not answered yet), so the label with a colon must
  // be absent while the plain call-to-action text stays on the page.
  assert.ok(
    !html.includes(`${ADVERTISING_ORDER_LABEL}:`),
    `expected no format card aria-labels before the API answers in:\n${html}`,
  )
})
