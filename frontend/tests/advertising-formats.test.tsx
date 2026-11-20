import { after, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import type { ComponentType } from 'react'

import type { AdFormat } from '../src/types'

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
type SiteConstants = typeof import('../src/constants')
type FormatCardModule = typeof import('../src/components/AdvertisingFormatCard')

let advertisingConstants!: AdvertisingConstants
let CATEGORIES!: SiteConstants['CATEGORIES']
let AdvertisingFormatCard!: FormatCardModule['default']

before(async () => {
  advertisingConstants = await import('../src/constants/advertising')
  ;({ CATEGORIES } = await import('../src/constants'))
  ;({ default: AdvertisingFormatCard } = await import('../src/components/AdvertisingFormatCard'))
})

function makeFormat(overrides: Partial<AdFormat> = {}): AdFormat {
  return {
    id: 'special-projects',
    title: 'Спецпроєкти',
    description: 'Окрема сторінка з власним дизайном',
    image_url: 'https://cdn.example.com/format.jpg',
    position: 1,
    ...overrides,
  }
}

type FormatCardProps = { format: AdFormat; href: string; ariaLabel: string }

function renderCard(format: AdFormat): string {
  const Card = AdvertisingFormatCard as unknown as ComponentType<FormatCardProps>

  return renderToStaticMarkup(
    <Card
      format={format}
      href="mailto:ads@mandat.ua?subject=order"
      ariaLabel={`Замовити рекламу: ${format.title}`}
    />,
  )
}

function imageTags(html: string): string[] {
  return Array.from(html.matchAll(/<img\b[^>]*>/g), (match) => match[0])
}

function attribute(tag: string, name: string): string | undefined {
  const match = tag.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

  return match?.[1]
}

test('advertising constants no longer export the ADVERTISING_FORMATS list', () => {
  const legacy = advertisingConstants as unknown as Record<string, unknown>

  assert.equal(
    legacy.ADVERTISING_FORMATS,
    undefined,
    'expected ADVERTISING_FORMATS to be removed: the advertising page must load cards from GET /api/ad-formats',
  )
})

test('advertising admin section stays outside the news rubrics', () => {
  assert.equal(advertisingConstants.ADVERTISING_ADMIN_SECTION, 'Реклама')

  assert.ok(
    !(CATEGORIES as string[]).includes(advertisingConstants.ADVERTISING_ADMIN_SECTION),
    'expected «Реклама» to stay out of CATEGORIES: it is an admin-only section, not a news rubric',
  )
})

test('AdvertisingFormatCard shows the image only when image_url is set', () => {
  const cases = [
    { name: 'a card with an image', format: makeFormat(), expectedImages: 1 },
    { name: 'a card without an image', format: makeFormat({ image_url: '' }), expectedImages: 0 },
  ]

  for (const { name, format, expectedImages } of cases) {
    const html = renderCard(format)
    const images = imageTags(html)

    assert.equal(images.length, expectedImages, `${name}: expected ${expectedImages} image(s) in:\n${html}`)
    if (expectedImages > 0) {
      assert.equal(
        attribute(images[0] ?? '', 'src'),
        format.image_url,
        `${name}: expected the preview src to be the card image_url in:\n${html}`,
      )
    }
  }
})

test('AdvertisingFormatCard renders the title and description', () => {
  const cases = [makeFormat(), makeFormat({ image_url: '' })]

  for (const format of cases) {
    const html = renderCard(format)

    assert.ok(html.includes(format.title), `expected the card title in:\n${html}`)
    assert.ok(html.includes(format.description), `expected the card description in:\n${html}`)
  }
})
