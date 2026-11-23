import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
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
  _load: (request: string, parent?: unknown, isMain?: boolean) => unknown
}

const moduleInternals = Module as unknown as ModuleInternals
const originalResolveFilename = moduleInternals._resolveFilename
const originalLoad = moduleInternals._load
const compiledSrcRoot = path.resolve(__dirname, '..', 'src')

// tsc emits no binary assets, so asset imports resolve to their would-be
// location under out/ and are stubbed by the require extensions below
// (banner-ads constants import banner screenshots).
const assetExtensions = new Set(['.jpg', '.jpeg', '.png', '.svg', '.webp'])

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  if (assetExtensions.has(path.extname(target)) && !existsSync(target)) {
    return target
  }

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

const originalExtensionLoaders: Record<string, (module: NodeModule, filename: string) => unknown> = {}
for (const extension of assetExtensions) {
  const existing = require.extensions[extension]
  if (existing) originalExtensionLoaders[extension] = existing
  require.extensions[extension] = (module, filename) => {
    module.exports = filename
  }
}

// ---------------------------------------------------------------------------
// The Advertising page loads the Telegram count with getTelegramChannel()
// inside useEffect, which the server renderer never runs. The injected
// useState below stands in for the state a browser would hold after the API
// answers, while every presentational component (Header, Footer, the tiles)
// stays the real implementation. Only Advertising's `react` import is
// patched, so every other module keeps the real hooks.
// ---------------------------------------------------------------------------

type UseStateResult = [unknown, (value: unknown) => void]

let realUseState: ((initial?: unknown) => UseStateResult) | undefined
let subscribersForRender: number | null = null

function advertisingUseState(initial?: unknown): UseStateResult {
  // The contract: the Telegram count lives in `useState<number | null>(null)`
  // next to the existing `useState<AdFormat[]>([])` for the format cards.
  if (initial === null) {
    return [subscribersForRender, () => {}]
  }
  if (!realUseState) throw new Error('react useState was not captured for Advertising')

  return realUseState(initial)
}

moduleInternals._load = function loadWithTelegramState(request, parent, ...rest) {
  const loaded = originalLoad.call(moduleInternals, request, parent, ...rest) as Record<string, unknown>
  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''

  if (request === 'react' && parentFilename.endsWith(path.join('pages', 'Advertising.js'))) {
    realUseState = loaded.useState as (initial?: unknown) => UseStateResult

    return { ...(loaded as object), useState: advertisingUseState }
  }

  return loaded
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  moduleInternals._load = originalLoad

  for (const extension of assetExtensions) {
    const original = originalExtensionLoaders[extension]
    if (original) {
      require.extensions[extension] = original
    } else {
      delete require.extensions[extension]
    }
  }
})

afterEach(() => {
  subscribersForRender = null
})

// The subjects load lazily: compiled src modules keep the '@/...' alias, so
// they must be required after the resolver above is installed (static imports
// would run first and fail on the alias).
type SiteConstants = typeof import('../src/constants')
type AdvertisingConstants = typeof import('../src/constants/advertising')

let TELEGRAM_CHANNEL!: SiteConstants['TELEGRAM_CHANNEL']
let SOCIAL_LINKS!: SiteConstants['SOCIAL_LINKS']
let ADVERTISING_AUDIENCE!: AdvertisingConstants['ADVERTISING_AUDIENCE']
let ADVERTISING_AUDIENCE_TITLE!: string
let ADVERTISING_FORMATS_TITLE!: string
let Advertising!: ComponentType

before(async () => {
  const siteConstants: SiteConstants = await import('../src/constants')
  TELEGRAM_CHANNEL = siteConstants.TELEGRAM_CHANNEL
  SOCIAL_LINKS = siteConstants.SOCIAL_LINKS
  const advertisingConstants: AdvertisingConstants = await import('../src/constants/advertising')
  ADVERTISING_AUDIENCE = advertisingConstants.ADVERTISING_AUDIENCE
  ADVERTISING_AUDIENCE_TITLE = advertisingConstants.ADVERTISING_AUDIENCE_TITLE
  ADVERTISING_FORMATS_TITLE = advertisingConstants.ADVERTISING_FORMATS_TITLE
  ;({ default: Advertising } = await import('../src/pages/Advertising'))
})

function renderAdvertising(): string {
  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/advertising']}>
      <Advertising />
    </MemoryRouter>,
  )
}

// The «Соціальні мережі» block sits between its heading and the formats
// heading; the footer further down reuses the same social links, so the tile
// lookup must stay inside this section.
function audienceSection(html: string): string {
  const start = html.indexOf(ADVERTISING_AUDIENCE_TITLE)
  assert.ok(start !== -1, `expected the «${ADVERTISING_AUDIENCE_TITLE}» block in:\n${html}`)
  const end = html.indexOf(ADVERTISING_FORMATS_TITLE, start)
  assert.ok(end > start, `expected the formats heading after the audience block in:\n${html}`)

  return html.slice(start, end)
}

function tileHtml(section: string, href: string): string {
  const anchors = Array.from(
    section.matchAll(/<a\b[^>]*href="([^"]*)"[^>]*>([\s\S]*?)<\/a>/g),
    (match) => ({ href: match[1] ?? '', content: match[2] ?? '' }),
  )
  const tile = anchors.find((anchor) => anchor.href === href)

  assert.ok(tile, `expected a social tile linking to ${href} in:\n${section}`)

  return tile.content
}

test('TELEGRAM_CHANNEL no longer carries a hardcoded subscribers count', () => {
  assert.equal(TELEGRAM_CHANNEL.name, 'МАНДАТ у Telegram')
  assert.equal(TELEGRAM_CHANNEL.url, 'https://t.me/mandat_ua')
  assert.ok(
    !('subscribers' in TELEGRAM_CHANNEL),
    `expected TELEGRAM_CHANNEL to drop the hardcoded subscribers count, got: ${JSON.stringify(TELEGRAM_CHANNEL)}`,
  )
})

test('ADVERTISING_AUDIENCE no longer carries the Telegram count', () => {
  const audience = ADVERTISING_AUDIENCE as unknown as Record<string, string | undefined>
  const telegram = audience['telegram']

  assert.ok(
    telegram === undefined || telegram === '—',
    `expected no hardcoded Telegram count in ADVERTISING_AUDIENCE, got ${JSON.stringify(telegram)} in: ${JSON.stringify(ADVERTISING_AUDIENCE)}`,
  )

  for (const id of ['instagram', 'threads', 'facebook']) {
    assert.equal(
      audience[id],
      '—',
      `expected the ${id} tile to keep its «—» placeholder in: ${JSON.stringify(ADVERTISING_AUDIENCE)}`,
    )
  }
})

test('Advertising page shows a placeholder in the Telegram tile while the count is loading', () => {
  subscribersForRender = null
  const html = renderAdvertising()
  const tile = tileHtml(audienceSection(html), TELEGRAM_CHANNEL.url)

  assert.ok(tile.includes('—'), `expected the Telegram tile to show «—» while loading in:\n${tile}`)
  assert.ok(
    !tile.includes('42 728') && !tile.includes('42728'),
    `expected no hardcoded count in the Telegram tile while loading in:\n${tile}`,
  )
})

test('Advertising page shows the live Telegram count formatted for uk-UA', () => {
  const cases = [3846, 42728]

  for (const count of cases) {
    subscribersForRender = count
    const html = renderAdvertising()
    const tile = tileHtml(audienceSection(html), TELEGRAM_CHANNEL.url)
    const expected = new Intl.NumberFormat('uk-UA').format(count)

    assert.ok(
      tile.includes(expected),
      `expected the Telegram tile to show ${expected} for ${count} in:\n${tile}`,
    )
    assert.ok(
      !tile.includes('—'),
      `expected no «—» placeholder in the Telegram tile once the count ${count} is loaded in:\n${tile}`,
    )
  }
})

test('Advertising page keeps placeholders for the other networks', () => {
  subscribersForRender = 3846
  const html = renderAdvertising()
  const section = audienceSection(html)
  const others = SOCIAL_LINKS.filter((social) => social.id !== 'telegram')

  assert.ok(others.length > 0, `expected social links besides Telegram in:\n${section}`)

  for (const social of others) {
    const tile = tileHtml(section, social.href)

    assert.ok(
      tile.includes('—'),
      `expected the ${social.id} tile to keep its «—» placeholder in:\n${tile}`,
    )
  }
})
