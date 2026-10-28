import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

import type { Article } from '../src/types'

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the existing API tests do.
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

// tsc emits no binary assets, so asset imports resolve to their would-be
// location under out/ and are stubbed by the require extension below.
const assetExtensions = new Set(['.jpg', '.jpeg', '.png', '.svg', '.webp'])

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  if (assetExtensions.has(path.extname(target)) && !existsSync(target)) {
    return target
  }

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

const originalJpgLoader = require.extensions['.jpg']
require.extensions['.jpg'] = (module, filename) => {
  module.exports = filename
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  if (originalJpgLoader) {
    require.extensions['.jpg'] = originalJpgLoader
  } else {
    delete require.extensions['.jpg']
  }
})

async function loadNewsCard(): Promise<ComponentType<{ article: Article }>> {
  const newsCardModule = await import('../src/components/NewsCard')

  return newsCardModule.default
}

async function loadHero(): Promise<ComponentType> {
  const heroModule = await import('../src/sections/Hero')

  return heroModule.default
}

function makeArticle(overrides: Partial<Article> = {}): Article {
  return {
    id: 'test-slug',
    category: 'Новини',
    title: 'Тестовий заголовок новини',
    excerpt: 'Короткий опис новини для картки',
    views: '12,4\u00A0тис.',
    publishedAt: '25.10.2026, 14:35',
    featured: false,
    imageUrl: 'https://cdn.example.com/news-card.jpg',
    ...overrides,
  }
}

function attribute(attributes: string, name: string): string | undefined {
  const match = attributes.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

  return match?.[1]
}

function openingAttributes(html: string, tag: string): string {
  const match = html.match(new RegExp(`<${tag}\\b([^>]*)>`))
  assert.ok(match, `expected a <${tag}> element in:\n${html}`)

  return match[1]
}

function classTokens(attributes: string): string[] {
  return (attribute(attributes, 'class') ?? '').split(/\s+/).filter(Boolean)
}

test('NewsCard keeps featured grid spans on the featured outer link only', async () => {
  const NewsCard = await loadNewsCard()
  const featuredHtml = renderToStaticMarkup(
    <MemoryRouter>
      <NewsCard article={makeArticle({ id: 'featured', featured: true })} />
    </MemoryRouter>,
  )
  const regularHtml = renderToStaticMarkup(
    <MemoryRouter>
      <NewsCard article={makeArticle({ id: 'regular' })} />
    </MemoryRouter>,
  )

  const featuredClasses = classTokens(openingAttributes(featuredHtml, 'a'))
  const regularClasses = classTokens(openingAttributes(regularHtml, 'a'))

  for (const className of ['lg:col-span-2', 'lg:row-span-2']) {
    assert.ok(
      featuredClasses.includes(className),
      `expected the featured outer link to retain ${className} in:\n${featuredHtml}`,
    )
    assert.ok(
      !regularClasses.includes(className),
      `expected the regular outer link not to include ${className} in:\n${regularHtml}`,
    )
  }
})

test('Hero uses responsive minimum heights and preserves its background and text', async () => {
  const Hero = await loadHero()
  const html = renderToStaticMarkup(
    <MemoryRouter>
      <Hero />
    </MemoryRouter>,
  )
  const sectionClasses = classTokens(openingAttributes(html, 'section'))

  for (const className of ['min-h-[240px]', 'sm:min-h-[360px]', 'lg:min-h-[480px]']) {
    assert.ok(
      sectionClasses.includes(className),
      `expected the Hero section to include ${className} in:\n${html}`,
    )
  }
  for (const className of ['h-[240px]', 'sm:h-[360px]', 'lg:h-[480px]']) {
    assert.ok(
      !sectionClasses.includes(className),
      `expected the Hero section to use expandable minimum heights, not ${className}`,
    )
  }

  const images = Array.from(html.matchAll(/<img\b([^>]*)>/g), (match) => match[1])
  const backgroundImage = images.find((attributes) =>
    (attribute(attributes, 'src') ?? '').endsWith('hero-bg.jpg'),
  )
  assert.ok(backgroundImage, `expected the existing Hero background image in:\n${html}`)
  assert.equal(attribute(backgroundImage, 'alt'), '')
  assert.equal(attribute(backgroundImage, 'aria-hidden'), 'true')
  assert.ok(
    classTokens(backgroundImage).includes('object-cover'),
    'expected the existing Hero background image treatment',
  )

  assert.ok(html.includes('Про владу, рішення,'), 'expected the existing Hero heading')
  assert.ok(html.includes('гроші та '), 'expected the complete existing Hero heading')
  assert.ok(html.includes('вплив.'), 'expected the highlighted Hero heading text')
  assert.ok(
    html.includes(
      'Факти без емоцій. Аналітика без шуму. Новини та інсайди, яким довіряють — у зручному форматі сайту.',
    ),
    'expected the existing Hero description',
  )
  assert.ok(html.includes('42 728'), 'expected the existing Hero audience statistic')
})
