import { test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter, Route, Routes } from 'react-router'
import type { ComponentType } from 'react'

import { NAV_LINKS, SITE_NAME } from '../src/constants'
import type { Article, ArticleDetail } from '../src/types'

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

interface ArticlePageModule {
  default: ComponentType
  ArticleContent: ComponentType<{ article: ArticleDetail }>
}

function articlePageModulePath(): string {
  // Article.tsx does not exist yet. Keeping the specifier non-literal lets the
  // rest of this suite compile and fail behaviorally before it is implemented.
  return '../src/pages/Article'
}

async function loadArticlePage(): Promise<ArticlePageModule> {
  return (await import(articlePageModulePath())) as ArticlePageModule
}

async function loadNewsCard(): Promise<ComponentType<{ article: Article }>> {
  const newsCardModule = await import('../src/components/NewsCard')

  return newsCardModule.default
}

async function loadHeader(): Promise<ComponentType> {
  const headerModule = await import('../src/sections/Header')

  return headerModule.default
}

function makeArticle(overrides: Partial<Article> = {}): Article {
  return {
    id: 'test-slug',
    category: 'Новини',
    title: 'Тестовий заголовок новини',
    excerpt: 'Короткий опис новини для картки',
    views: '12,4\u00A0тис.',
    publishedAt: '25.10.2026, 14:35',
    comments: '32 коментарі',
    featured: false,
    imageUrl: 'https://cdn.example.com/news-card.jpg',
    ...overrides,
  }
}

function makeArticleDetail(overrides: Partial<ArticleDetail> = {}): ArticleDetail {
  return {
    ...makeArticle({
      id: 'test-slug',
      category: 'Міжнародні новини',
      title: 'Повний заголовок статті',
      excerpt: 'Стислий опис статті',
      featured: true,
      imageUrl: 'https://cdn.example.com/article-cover.jpg',
    }),
    text:
      'Перший абзац із <strong>розміткою</strong>.\n\n' +
      'Другий абзац.\n  Рядок з відступами  \n\n' +
      '<script>alert("xss")</script>',
    ...overrides,
  }
}

function attribute(attributes: string, name: string): string | undefined {
  const match = attributes.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

  return match?.[1]
}

function anchors(html: string): { attributes: string; content: string }[] {
  return Array.from(html.matchAll(/<a\b([^>]*)>([\s\S]*?)<\/a>/g), (match) => ({
    attributes: match[1],
    content: match[2],
  }))
}

function decodeHtml(value: string): string {
  return value
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#x27;/g, "'")
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, '&')
}

function visibleText(html: string): string {
  return decodeHtml(html.replace(/<[^>]*>/g, ' '))
}

function elementsWithClass(html: string, className: string): string[] {
  const pattern = new RegExp(
    `<([a-z][a-z0-9]*)\\b[^>]*\\bclass="[^"]*\\b${className}\\b[^"]*"[^>]*>([\\s\\S]*?)<\\/\\1>`,
    'gi',
  )

  return Array.from(html.matchAll(pattern), (match) => match[2])
}

test('NewsCard wraps the whole card in an encoded article link', async () => {
  const NewsCard = await loadNewsCard()
  const slug = 'світ/економіка 2026?x=1&y=2'
  const article = makeArticle({
    id: slug,
    category: 'Міжнародні новини',
    title: 'Картка зі складним слагом',
    excerpt: 'Опис картки зі складним слагом',
    featured: true,
  })
  const expectedHref = `/news/${encodeURIComponent(slug)}`

  const html = renderToStaticMarkup(
    <MemoryRouter>
      <NewsCard article={article} />
    </MemoryRouter>,
  )

  const link = anchors(html).find(
    (candidate) => attribute(candidate.attributes, 'href') === expectedHref,
  )

  assert.ok(link, `expected a link to ${expectedHref} in:\n${html}`)

  const cardText = visibleText(link.content)
  for (const value of [
    article.title,
    article.excerpt,
    article.category,
    article.views,
    article.publishedAt,
  ]) {
    assert.ok(
      cardText.includes(value),
      `expected the whole card, including ${JSON.stringify(value)}, inside the article link`,
    )
  }
})

test('Header logo links home while keeping navigation and search', async () => {
  const Header = await loadHeader()

  const html = renderToStaticMarkup(
    <MemoryRouter>
      <Header />
    </MemoryRouter>,
  )

  const logo = anchors(html).find((candidate) => visibleText(candidate.content).includes(SITE_NAME))

  assert.ok(logo, `expected the logo link in:\n${html}`)
  assert.equal(attribute(logo.attributes, 'href'), '/')
  assert.match(logo.content, /bg-mandat-red-bright/, 'the logo mark must be kept')

  // React escapes apostrophes as &#x27;, so compare against decoded markup.
  const decodedNavigationHtml = decodeHtml(html)
  for (const link of NAV_LINKS) {
    assert.ok(decodedNavigationHtml.includes(link.label), `expected navigation item ${link.label}`)
  }

  assert.ok(html.includes('type="search"'), 'expected the search input')
  assert.ok(html.includes('placeholder="Пошук новин…"'), 'expected the search placeholder')
  assert.ok(html.includes('aria-label="Пошук новин"'), 'expected the search action')
})

test('Article route keeps the site chrome and starts in a loading state', async () => {
  const { default: ArticlePage } = await loadArticlePage()
  assert.equal(typeof ArticlePage, 'function', 'Article.tsx must default-export the page')

  const originalFetch = globalThis.fetch
  let fetchCalls = 0
  globalThis.fetch = (async () => {
    fetchCalls += 1
    throw new Error('fetch must not run during SSR')
  }) as typeof fetch

  let html = ''
  try {
    html = renderToStaticMarkup(
      <MemoryRouter initialEntries={['/news/test-slug']}>
        <Routes>
          <Route path="/news/:slug" element={<ArticlePage />} />
        </Routes>
      </MemoryRouter>,
    )
  } finally {
    globalThis.fetch = originalFetch
  }

  assert.equal(fetchCalls, 0, 'SSR must not run data-loading effects')
  assert.match(html, /<header\b/, 'expected the site header')
  assert.match(html, /<footer\b/, 'expected the site footer')
  assert.ok(html.includes(SITE_NAME), 'expected the header logo text')
  const decodedNavigationHtml = decodeHtml(html)
  for (const link of NAV_LINKS) {
    assert.ok(decodedNavigationHtml.includes(link.label), `expected navigation item ${link.label}`)
  }
  assert.ok(html.includes('Усі права захищено'), 'expected the footer content')

  const loadingStatus = /role="status"|aria-live="polite"|Завантаж/
  assert.ok(loadingStatus.test(html), `expected a loading status in:\n${html}`)
  assert.ok(!html.includes('Новину не знайдено.'), 'initial SSR must not show a not-found error')
})

test('ArticleContent renders a complete article as escaped plain text', async () => {
  const { ArticleContent } = await loadArticlePage()
  assert.equal(typeof ArticleContent, 'function', 'Article.tsx must export ArticleContent')

  const article = makeArticleDetail()

  const html = renderToStaticMarkup(
    <MemoryRouter>
      <ArticleContent article={article} />
    </MemoryRouter>,
  )

  const cover = Array.from(html.matchAll(/<img\b([^>]*)>/g), (match) => match[1]).find(
    (attributes) => decodeHtml(attribute(attributes, 'src') ?? '') === article.imageUrl,
  )
  assert.ok(cover, `expected the cover image ${article.imageUrl} in:\n${html}`)

  const alt = decodeHtml(attribute(cover, 'alt') ?? '')
  assert.ok(alt.length > 0, 'the cover image must have a non-empty alt')
  assert.ok(
    alt.includes(article.title),
    `expected the cover alt to describe the article, got ${JSON.stringify(alt)}`,
  )

  const text = visibleText(html)
  assert.ok(text.includes(article.title), 'expected the article title as visible text')
  assert.ok(text.includes(article.publishedAt), 'expected the publication date')
  assert.ok(text.includes(article.views), 'expected the view count')

  const textBlocks = elementsWithClass(html, 'whitespace-pre-line')
  assert.ok(
    textBlocks.some((block) => decodeHtml(block) === article.text),
    `expected the full multiline text in a whitespace-pre-line element, got:\n${html}`,
  )

  assert.ok(!html.includes('<strong>'), 'article HTML must not become a <strong> element')
  assert.ok(!/<script\b/i.test(html), 'article script must not become a <script> element')
  assert.ok(html.includes('&lt;strong&gt;'), 'expected escaped article HTML')
  assert.ok(html.includes('&lt;script&gt;'), 'expected escaped article script')
})

test('ArticleContent renders the pattern cover fallback when the image is missing', async () => {
  const { ArticleContent } = await loadArticlePage()
  const article = makeArticleDetail({ imageUrl: undefined })

  const html = renderToStaticMarkup(
    <MemoryRouter>
      <ArticleContent article={article} />
    </MemoryRouter>,
  )

  // The responsive cover sits above the title. Without an image it must keep
  // the site's diagonal/radial pattern instead of collapsing into an empty band.
  const titleIndex = html.search(/<h1\b/)
  assert.ok(titleIndex > 0, `expected the cover before the article title in:\n${html}`)

  const coverMarkup = html.slice(0, titleIndex)

  assert.ok(
    coverMarkup.includes('repeating-linear-gradient'),
    `expected the diagonal pattern fallback in the cover markup:\n${coverMarkup}`,
  )
  assert.ok(
    coverMarkup.includes('radial-gradient'),
    `expected the radial glow fallback in the cover markup:\n${coverMarkup}`,
  )
  assert.ok(
    !/<img\b/i.test(coverMarkup),
    `expected no cover image when imageUrl is absent, got:\n${coverMarkup}`,
  )
})
