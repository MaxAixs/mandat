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

type HeroProps = { mainEvent?: Article | null }

async function loadHero(): Promise<ComponentType<HeroProps>> {
  const heroModule = await import('../src/sections/Hero')

  return heroModule.default as ComponentType<HeroProps>
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

interface AnchorElement {
  attributes: string
  content: string
}

function anchors(html: string): AnchorElement[] {
  return Array.from(html.matchAll(/<a\b([^>]*)>([\s\S]*?)<\/a>/g), (match) => ({
    attributes: match[1],
    content: match[2],
  }))
}

function elementContent(html: string, tag: string): string {
  const match = html.match(new RegExp(`<${tag}\\b[^>]*>([\\s\\S]*?)</${tag}>`))
  assert.ok(match, `expected a <${tag}> element in:\n${html}`)

  return match[1]
}

function imageWithSrcSuffix(html: string, suffix: string): string | undefined {
  return Array.from(html.matchAll(/<img\b([^>]*)>/g), (match) => match[1]).find((attributes) =>
    (attribute(attributes, 'src') ?? '').endsWith(suffix),
  )
}

// Opening <div> attributes of every element still open at the given text.
function openDivAncestors(html: string, text: string): string[] {
  const index = html.indexOf(text)
  assert.ok(index >= 0, `expected ${text} in:\n${html}`)

  const stack: string[] = []
  const tagPattern = /<\/?div\b([^>]*)>/g
  const prefix = html.slice(0, index)
  let match: RegExpExecArray | null = tagPattern.exec(prefix)

  while (match !== null) {
    if (match[0].startsWith('</')) stack.pop()
    else stack.push(match[1])
    match = tagPattern.exec(prefix)
  }

  return stack
}

function isDesktopBottomRight(classes: string[]): boolean {
  return (
    classes.includes('lg:absolute') &&
    classes.some((token) => /^lg:bottom-/.test(token)) &&
    classes.some((token) => /^lg:right-/.test(token))
  )
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

test('Hero renders the selected main event title, description, cover and news link', async () => {
  const Hero = await loadHero()
  const mainEvent = makeArticle({
    id: 'main-event-slug',
    title: 'Обрана головна подія',
    excerpt: 'Короткий опис обраної головної події',
    imageUrl: 'https://cdn.example.com/main-event-cover.jpg',
  })
  const html = renderToStaticMarkup(
    <MemoryRouter>
      <Hero mainEvent={mainEvent} />
    </MemoryRouter>,
  )

  assert.ok(html.includes(mainEvent.title), `expected the selected title in:\n${html}`)
  assert.ok(html.includes(mainEvent.excerpt), `expected the selected description in:\n${html}`)
  assert.ok(
    imageWithSrcSuffix(html, 'main-event-cover.jpg'),
    `expected the selected cover image in:\n${html}`,
  )

  const heading = elementContent(html, 'h1')
  assert.ok(
    heading.includes(mainEvent.title),
    `expected the selected title to become the Hero heading in:\n${html}`,
  )

  const newsLink = anchors(html).find(
    (anchor) => attribute(anchor.attributes, 'href') === `/news/${mainEvent.id}`,
  )
  assert.ok(newsLink, `expected the Hero heading to link to /news/${mainEvent.id} in:\n${html}`)
  assert.ok(
    newsLink.content.includes(mainEvent.title),
    `expected the selected heading to be the link text in:\n${html}`,
  )

  assert.ok(
    !html.includes('Про владу, рішення,'),
    'the static heading must not be rendered when a main event is selected',
  )
  assert.ok(
    !html.includes('Головна подія дня'),
    'the Hero must not add a separate «Головна подія дня» label',
  )
})

test('Hero keeps its static heading, description and background without a selected article', async () => {
  const Hero = await loadHero()
  const variants: Array<{ name: string; mainEvent: Article | null | undefined }> = [
    { name: 'omitted', mainEvent: undefined },
    { name: 'null', mainEvent: null },
  ]

  for (const { name, mainEvent } of variants) {
    const html = renderToStaticMarkup(
      <MemoryRouter>
        <Hero mainEvent={mainEvent} />
      </MemoryRouter>,
    )
    const heading = elementContent(html, 'h1')

    assert.ok(html.includes('Про владу, рішення,'), `${name}: expected the static heading`)
    assert.ok(html.includes('гроші та '), `${name}: expected the complete static heading`)
    assert.ok(html.includes('вплив.'), `${name}: expected the highlighted static heading`)
    assert.ok(
      html.includes(
        'Факти без емоцій. Аналітика без шуму. Новини та інсайди, яким довіряють — у зручному форматі сайту.',
      ),
      `${name}: expected the static description`,
    )
    assert.ok(
      imageWithSrcSuffix(html, 'hero-bg.jpg'),
      `${name}: expected the bundled Hero background`,
    )
    assert.ok(
      !heading.includes('<a'),
      `${name}: the fallback heading must not be a link in:\n${html}`,
    )
    assert.ok(
      !html.includes('href="/news/'),
      `${name}: the fallback Hero must not link to a news article in:\n${html}`,
    )
    assert.ok(
      !html.includes('Головна подія дня'),
      `${name}: the Hero must not add a separate «Головна подія дня» label`,
    )
  }
})

test('Hero keeps the bundled background when the selected article has no cover', async () => {
  const Hero = await loadHero()
  const mainEvent = makeArticle({
    id: 'main-event-without-cover',
    title: 'Головна подія без обкладинки',
    imageUrl: undefined,
  })
  const html = renderToStaticMarkup(
    <MemoryRouter>
      <Hero mainEvent={mainEvent} />
    </MemoryRouter>,
  )

  assert.ok(html.includes(mainEvent.title), `expected the selected title in:\n${html}`)

  const background = imageWithSrcSuffix(html, 'hero-bg.jpg')
  assert.ok(background, `expected the bundled Hero background in:\n${html}`)
  assert.equal(attribute(background, 'alt'), '')
  assert.equal(attribute(background, 'aria-hidden'), 'true')
  assert.ok(
    classTokens(background).includes('object-cover'),
    'expected the bundled background to keep its cover treatment',
  )
})

test('Hero places the audience block bottom-right on desktop and in flow on mobile', async () => {
  const Hero = await loadHero()
  const html = renderToStaticMarkup(
    <MemoryRouter>
      <Hero />
    </MemoryRouter>,
  )
  const ancestors = openDivAncestors(html, '42 728').map(classTokens)
  const positionedAncestor = ancestors.find(isDesktopBottomRight)

  assert.ok(positionedAncestor, `expected the audience block to use lg:absolute with lg:bottom-* and lg:right-* in:\n${html}`)

  const mobilePositioning = ancestors.flat().filter(
    (token) =>
      !token.includes(':') &&
      /^(absolute|fixed|sticky|bottom-|right-|inset-|top-|left-)/.test(token),
  )
  assert.deepEqual(
    mobilePositioning,
    [],
    `expected the audience block to stay in the document flow on mobile in:\n${html}`,
  )
  assert.ok(html.includes('підписників уже з нами'), 'expected the audience label to remain')
})
