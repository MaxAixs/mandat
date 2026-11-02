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

// The `large` prop is specified but not implemented yet. Declaring the expected
// prop locally keeps this suite compiling so the failure stays behavioral.
type NewsCardProps = { article: Article; large?: boolean }

async function loadNewsCard(): Promise<ComponentType<NewsCardProps>> {
  const newsCardModule = await import('../src/components/NewsCard')

  return newsCardModule.default as unknown as ComponentType<NewsCardProps>
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

function renderCard(NewsCard: ComponentType<NewsCardProps>, props: NewsCardProps): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <NewsCard {...props} />
    </MemoryRouter>,
  )
}

test('NewsCard renders the large-block classes for the large prop without featured data', async () => {
  const NewsCard = await loadNewsCard()
  const html = renderCard(NewsCard, {
    article: makeArticle({ id: 'large-without-featured', featured: false }),
    large: true,
  })

  const linkClasses = classTokens(openingAttributes(html, 'a'))
  const titleClasses = classTokens(openingAttributes(html, 'h3'))

  assert.ok(
    linkClasses.includes('lg:col-span-2'),
    `expected the large outer link to include lg:col-span-2 in:\n${html}`,
  )
  assert.ok(
    linkClasses.includes('lg:row-span-2'),
    `expected the large outer link to include lg:row-span-2 in:\n${html}`,
  )
  assert.ok(
    titleClasses.includes('text-[24px]'),
    `expected the large card title to use text-[24px] in:\n${html}`,
  )
  assert.ok(
    html.includes('lg:flex-1'),
    `expected the large card image container to expand with lg:flex-1 in:\n${html}`,
  )
})

test('NewsCard keeps the regular classes without the large prop and featured data', async () => {
  const NewsCard = await loadNewsCard()
  const html = renderCard(NewsCard, {
    article: makeArticle({ id: 'regular', featured: false }),
  })

  const linkClasses = classTokens(openingAttributes(html, 'a'))
  const titleClasses = classTokens(openingAttributes(html, 'h3'))

  for (const className of ['lg:col-span-2', 'lg:row-span-2']) {
    assert.ok(
      !linkClasses.includes(className),
      `expected the regular outer link not to include ${className} in:\n${html}`,
    )
  }
  assert.ok(
    titleClasses.includes('text-[17px]'),
    `expected the regular card title to keep text-[17px] in:\n${html}`,
  )
  assert.ok(
    !html.includes('lg:flex-1'),
    `expected the regular card image container not to expand in:\n${html}`,
  )
})
