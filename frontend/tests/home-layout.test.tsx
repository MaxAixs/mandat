import { after, test } from 'node:test'
import assert from 'node:assert/strict'
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

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
})

async function loadNewsCard(): Promise<ComponentType<{ article: Article }>> {
  const newsCardModule = await import('../src/components/NewsCard')

  return newsCardModule.default
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
