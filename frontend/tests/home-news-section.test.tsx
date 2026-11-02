import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

import type { Article } from '../src/types'

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the other frontend tests do.
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

interface NewsProps {
  articles: Article[]
  loading: boolean
  error: string | null
  onRetry: () => void
}

async function loadNews(): Promise<ComponentType<NewsProps>> {
  const newsModule = await import('../src/sections/News')

  return newsModule.default as ComponentType<NewsProps>
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

function renderNews(News: ComponentType<NewsProps>, articles: Article[]): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <News articles={articles} loading={false} error={null} onRetry={() => {}} />
    </MemoryRouter>,
  )
}

function attribute(attributes: string, name: string): string | undefined {
  const match = attributes.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

  return match?.[1]
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

function cardClasses(html: string, id: string): string[] | undefined {
  const anchor = anchors(html).find(
    (candidate) => attribute(candidate.attributes, 'href') === `/news/${encodeURIComponent(id)}`,
  )

  return anchor ? classTokens(anchor.attributes) : undefined
}

// A large NewsCard marks its outer link with lg:col-span-2 lg:row-span-2.
function largeCardCount(html: string): number {
  return html.split('lg:col-span-2').length - 1
}

test('News renders only the first featured article as the large card', async () => {
  const News = await loadNews()
  const first = makeArticle({ id: 'first-featured', featured: true })
  const second = makeArticle({ id: 'second-featured', featured: true })
  const third = makeArticle({ id: 'regular-third' })
  const html = renderNews(News, [first, second, third])

  assert.equal(
    largeCardCount(html),
    1,
    `expected at most one large mosaic card, got ${largeCardCount(html)} in:\n${html}`,
  )

  const firstClasses = cardClasses(html, first.id)
  assert.ok(firstClasses, `expected a card for ${first.id} in:\n${html}`)
  assert.ok(
    firstClasses.includes('lg:col-span-2'),
    `expected the first featured article to render large in:\n${html}`,
  )

  for (const article of [second, third]) {
    const classes = cardClasses(html, article.id)
    assert.ok(classes, `expected a card for ${article.id} in:\n${html}`)
    assert.ok(
      !classes.includes('lg:col-span-2'),
      `expected ${article.id} at index > 0 not to render large in:\n${html}`,
    )
    assert.ok(
      !classes.includes('lg:row-span-2'),
      `expected ${article.id} at index > 0 not to span two rows in:\n${html}`,
    )
  }
})

test('News renders the mosaic leader large when it is the main event', async () => {
  const News = await loadNews()
  const leader = makeArticle({ id: 'mosaic-leader', mainEvent: true, featured: false })
  const featuredLater = makeArticle({ id: 'featured-later', featured: true })
  const mainEventLater = makeArticle({ id: 'main-event-later', mainEvent: true })
  const regular = makeArticle({ id: 'regular' })
  const html = renderNews(News, [leader, featuredLater, mainEventLater, regular])

  assert.equal(
    largeCardCount(html),
    1,
    `expected exactly one large mosaic card, got ${largeCardCount(html)} in:\n${html}`,
  )

  const leaderClasses = cardClasses(html, leader.id)
  assert.ok(leaderClasses, `expected a card for ${leader.id} in:\n${html}`)
  for (const className of ['lg:col-span-2', 'lg:row-span-2']) {
    assert.ok(
      leaderClasses.includes(className),
      `expected the main-event leader to render large with ${className} in:\n${html}`,
    )
  }

  for (const later of [featuredLater, mainEventLater]) {
    const laterClasses = cardClasses(html, later.id)
    assert.ok(laterClasses, `expected a card for ${later.id} in:\n${html}`)
    assert.ok(
      !laterClasses.includes('lg:col-span-2'),
      `expected ${later.id} at index > 0 not to render large in:\n${html}`,
    )
  }
})

test('News keeps every card regular when the leader is neither featured nor a main event', async () => {
  const News = await loadNews()
  const leader = makeArticle({ id: 'plain-leader', featured: false, mainEvent: false })
  const featuredLater = makeArticle({ id: 'featured-later', featured: true })
  const regular = makeArticle({ id: 'regular' })
  const html = renderNews(News, [leader, featuredLater, regular])

  assert.equal(
    largeCardCount(html),
    0,
    `expected no large mosaic card when the leader is plain, got ${largeCardCount(html)} in:\n${html}`,
  )
})
