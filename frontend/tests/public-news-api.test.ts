import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { listPublicNews } from '../src/api/news'
import type { PublicNews, PublicNewsListResponse } from '../src/api/news'
import type { Article } from '../src/types'

// src/api/index.ts imports '@/api/news'; Node needs the alias resolved at runtime.
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

type HomeApi = typeof import('../src/api')
type NewsApi = typeof import('../src/api/news')

// The detail API is specified but not exported yet. Declaring the expected
// signatures locally keeps this suite compiling so the failure stays behavioral.
type PublicNewsDetailPayload = PublicNews & { text: string }
type ArticleDetailPayload = Article & { text: string }

type ExpectedNewsApi = NewsApi & {
  getPublicNews?: (slug: string) => Promise<PublicNewsDetailPayload>
}

type ExpectedHomeApi = HomeApi & {
  getPublicNewsArticle?: (slug: string) => Promise<ArticleDetailPayload>
}

// HomeNewsData.mainEvent is specified but not implemented yet. Declaring the
// expected shape locally keeps this suite compiling so the failure stays behavioral.
type HomeNewsWithMainEvent = Awaited<ReturnType<HomeApi['getHomeNews']>> & {
  mainEvent: Article | null
}

function asHomeNewsWithMainEvent(
  result: Awaited<ReturnType<HomeApi['getHomeNews']>>,
): HomeNewsWithMainEvent {
  return result as HomeNewsWithMainEvent
}

let getHomeNews: HomeApi['getHomeNews']
let getPublicNews: ExpectedNewsApi['getPublicNews']
let getPublicNewsArticle: ExpectedHomeApi['getPublicNewsArticle']

before(async () => {
  const api = (await import('../src/api')) as ExpectedHomeApi
  const newsApi = (await import('../src/api/news')) as ExpectedNewsApi
  getHomeNews = api.getHomeNews
  getPublicNews = newsApi.getPublicNews
  getPublicNewsArticle = api.getPublicNewsArticle
})

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
})

const PUBLIC_NEWS_URL = 'http://localhost:8080/api/news'
const originalFetch = globalThis.fetch

function makeItem(overrides: Partial<PublicNews> = {}): PublicNews {
  return {
    slug: 'test-slug',
    category: 'Новини',
    title: 'Тестова новина',
    description: 'Короткий опис',
    image_url: '',
    featured: false,
    main_event: false,
    views: 0,
    published_at: null,
    ...overrides,
  }
}

function localIsoDate(day: number, hour: number, minute: number): string {
  return new Date(2026, 9, day, hour, minute).toISOString()
}

function mockResponse(body: unknown, status = 200, statusText?: string): void {
  globalThis.fetch = async () => new Response(JSON.stringify(body), { status, statusText })
}

function makeDetail(overrides: Partial<PublicNewsDetailPayload> = {}): PublicNewsDetailPayload {
  return { ...makeItem(), text: 'Повний текст новини', ...overrides }
}

function requireFunction<T>(fn: T | undefined, name: string): T {
  assert.equal(typeof fn, 'function', `${name} must be exported and callable`)
  return fn as T
}

afterEach(() => {
  globalThis.fetch = originalFetch
})

test('listPublicNews uses the public endpoint with default pagination', async () => {
  const item = makeItem({
    slug: 'economy-1',
    category: 'Міжнародні новини',
    title: 'Заголовок',
    description: 'Опис',
    image_url: 'https://cdn.example.com/1.jpg',
    featured: true,
    views: 12400,
    published_at: localIsoDate(25, 14, 35),
  })
  const responseBody: PublicNewsListResponse = { items: [item], page: 1, limit: 20 }
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify(responseBody), { status: 200 })
  }

  const result = await listPublicNews()

  assert.equal(requestedUrl, `${PUBLIC_NEWS_URL}?page=1&limit=20`)
  assert.deepEqual(result, responseBody)
})

test('listPublicNews forwards custom page and limit', async () => {
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items: [], page: 3, limit: 5 }), { status: 200 })
  }

  const result = await listPublicNews(3, 5)

  assert.equal(requestedUrl, `${PUBLIC_NEWS_URL}?page=3&limit=5`)
  assert.deepEqual(result, { items: [], page: 3, limit: 5 })
})

test('listPublicNews uses the backend error message', async () => {
  mockResponse({ error: { message: 'News article not found' } }, 404)

  await assert.rejects(listPublicNews(), /News article not found/)
})

test('listPublicNews falls back to a status message for non-JSON errors', async () => {
  globalThis.fetch = async () => new Response('internal failure', { status: 500 })

  await assert.rejects(listPublicNews(), /Помилка сервера \(500\)\./)
})

test('listPublicNews reports network failures', async () => {
  globalThis.fetch = async () => {
    throw new TypeError('fetch failed')
  }

  await assert.rejects(listPublicNews(), /Не вдалося з’єднатися з сервером/)
})

test('getHomeNews maps public news into home page sections', async () => {
  const items = [
    makeItem({
      slug: 'important-1',
      category: "Інтерв'ю",
      title: 'Важлива новина',
      description: 'Стислий опис',
      image_url: 'https://cdn.example.com/important.jpg',
      featured: true,
      views: 12400,
      published_at: localIsoDate(25, 14, 35),
    }),
    makeItem({
      slug: 'news-2',
      category: 'Новини',
      title: 'Звичайна новина',
      description: 'Інший опис',
      image_url: '',
      featured: false,
      views: 0,
      published_at: localIsoDate(24, 9, 5),
    }),
  ]
  let requestedUrl = ''
  let calls = 0
  globalThis.fetch = async (input) => {
    calls += 1
    requestedUrl = String(input)
    return new Response(JSON.stringify({ items, page: 1, limit: 20 }), { status: 200 })
  }

  const result = await getHomeNews()

  assert.equal(calls, 1)
  assert.equal(requestedUrl, `${PUBLIC_NEWS_URL}?page=1&limit=20`)
  assert.deepEqual(result.articles, [
    {
      id: 'important-1',
      category: "Інтерв'ю",
      title: 'Важлива новина',
      excerpt: 'Стислий опис',
      views: '12,4\u00A0тис.',
      publishedAt: '25.10.2026, 14:35',
      featured: true,
      mainEvent: false,
      imageUrl: 'https://cdn.example.com/important.jpg',
    },
    {
      id: 'news-2',
      category: 'Новини',
      title: 'Звичайна новина',
      excerpt: 'Інший опис',
      views: '0',
      publishedAt: '24.10.2026, 09:05',
      featured: false,
      mainEvent: false,
      imageUrl: undefined,
    },
  ])
  assert.deepEqual(result.latestNews, [
    {
      id: 'important-1',
      time: '14:35',
      source: "Інтерв'ю",
      important: true,
      title: 'Важлива новина',
    },
    {
      id: 'news-2',
      time: '09:05',
      source: 'Новини',
      important: false,
      title: 'Звичайна новина',
    },
  ])
  assert.deepEqual(result.tickerItems, ['Важлива новина', 'Звичайна новина'])
})

test('getHomeNews keeps featured articles first but secondary feeds chronological', async () => {
  const olderFeatured = makeItem({
    slug: 'featured-older',
    category: "Інтерв'ю",
    title: 'Старіша важлива новина',
    featured: true,
    published_at: localIsoDate(20, 8, 15),
  })
  const newerRegular = makeItem({
    slug: 'regular-newer',
    title: 'Новіша звичайна новина',
    published_at: localIsoDate(25, 16, 45),
  })
  mockResponse({ items: [olderFeatured, newerRegular], page: 1, limit: 20 })

  const result = await getHomeNews()

  assert.deepEqual(
    result.articles.map((article) => article.id),
    ['featured-older', 'regular-newer'],
  )
  assert.deepEqual(
    result.latestNews.map((item) => item.id),
    ['regular-newer', 'featured-older'],
  )
  assert.deepEqual(result.tickerItems, ['Новіша звичайна новина', 'Старіша важлива новина'])
})

test('getHomeNews limits the main news to six while keeping five ticker headlines', async () => {
  const items = Array.from({ length: 7 }, (_, index) =>
    makeItem({
      slug: `news-${index + 1}`,
      title: `Новина ${index + 1}`,
      published_at: localIsoDate(25, 10, index),
    }),
  )
  mockResponse({ items, page: 1, limit: 20 })

  const result = await getHomeNews()

  // All seven items are regular, so the featured-first order is chronological.
  assert.equal(result.articles.length, 6, 'expected the main news list to be limited to 6')
  assert.deepEqual(
    result.articles.map((article) => article.id),
    ['news-7', 'news-6', 'news-5', 'news-4', 'news-3', 'news-2'],
    'expected the six main news to be the newest six in chronological order',
  )
  assert.deepEqual(result.tickerItems, [
    'Новина 7',
    'Новина 6',
    'Новина 5',
    'Новина 4',
    'Новина 3',
  ])
})

test('getHomeNews returns empty sections and no main event for an empty feed', async () => {
  mockResponse({ items: [], page: 1, limit: 20 })

  const result = await getHomeNews()

  assert.deepEqual(result, {
    articles: [],
    latestNews: [],
    tickerItems: [],
    mainEvent: null,
  })
})

test('getHomeNews maps only the main_event article into mainEvent', async () => {
  const featuredNewer = makeItem({
    slug: 'featured-newer',
    category: 'Міжнародні новини',
    title: 'Новіша важлива новина',
    description: 'Опис важливої новини',
    image_url: 'https://cdn.example.com/featured.jpg',
    featured: true,
    main_event: false,
    views: 900,
    published_at: localIsoDate(25, 18, 0),
  })
  const selectedMainEvent = makeItem({
    slug: 'main-event',
    category: 'Новини',
    title: 'Головна подія дня',
    description: 'Стислий опис головної події',
    image_url: 'https://cdn.example.com/main-event.jpg',
    featured: false,
    main_event: true,
    views: 0,
    published_at: localIsoDate(23, 8, 10),
  })
  const regular = makeItem({
    slug: 'regular',
    category: 'Спорт',
    title: 'Звичайна новина',
    description: 'Опис звичайної новини',
    main_event: false,
    published_at: localIsoDate(24, 12, 30),
  })
  mockResponse({
    items: [featuredNewer, selectedMainEvent, regular],
    page: 1,
    limit: 20,
  })

  const result = asHomeNewsWithMainEvent(await getHomeNews())
  const mainEvent = result.mainEvent

  assert.ok(mainEvent, 'expected getHomeNews to map the item with main_event=true into mainEvent')
  assert.deepEqual(mainEvent, {
    id: 'main-event',
    category: 'Новини',
    title: 'Головна подія дня',
    excerpt: 'Стислий опис головної події',
    views: '0',
    publishedAt: '23.10.2026, 08:10',
    featured: false,
    mainEvent: true,
    imageUrl: 'https://cdn.example.com/main-event.jpg',
  })
  assert.notEqual(
    mainEvent.id,
    'featured-newer',
    'a featured article must not become the main event by itself',
  )

  // The hero is excluded from the mosaic, so it cannot repeat on the home page.
  // «Головні новини» keeps featured items first, then regulars chronologically.
  assert.deepEqual(
    result.articles.map((article) => article.id),
    ['featured-newer', 'regular'],
  )
  assert.ok(
    result.articles.every((article) => article.id !== mainEvent.id),
    'the hero article must not repeat inside the mosaic',
  )
  assert.deepEqual(
    result.latestNews.map((item) => item.id),
    ['featured-newer', 'regular', 'main-event'],
  )
  assert.deepEqual(result.tickerItems, [
    'Новіша важлива новина',
    'Звичайна новина',
    'Головна подія дня',
  ])
})

test('getHomeNews makes the second-newest main event lead the mosaic without repeating the hero', async () => {
  const newestMain = makeItem({
    slug: 'main-newest',
    category: 'Новини',
    title: 'Найновіша головна подія',
    main_event: true,
    published_at: localIsoDate(25, 12, 0),
  })
  const secondMain = makeItem({
    slug: 'main-second',
    category: 'Спорт',
    title: 'Друга головна подія',
    main_event: true,
    published_at: localIsoDate(24, 12, 0),
  })
  const regularNewer = makeItem({
    slug: 'regular-newer',
    title: 'Звичайна новіша новина',
    published_at: localIsoDate(23, 12, 0),
  })
  const featuredOlder = makeItem({
    slug: 'featured-older',
    category: "Інтерв'ю",
    title: 'Старіша важлива новина',
    featured: true,
    published_at: localIsoDate(22, 12, 0),
  })
  // Response order is deliberately shuffled: ordering must come from published_at.
  mockResponse({
    items: [regularNewer, featuredOlder, secondMain, newestMain],
    page: 1,
    limit: 20,
  })

  const result = asHomeNewsWithMainEvent(await getHomeNews())

  assert.ok(result.mainEvent, 'expected the newest main_event article to become the hero')
  assert.equal(result.mainEvent.id, 'main-newest', 'expected the newest main event as the hero')
  assert.deepEqual(
    result.articles.map((article) => article.id),
    ['main-second', 'featured-older', 'regular-newer'],
    'expected the second-newest main event to lead the mosaic, then featured-first items',
  )
  assert.deepEqual(
    result.articles.map((article) => article.mainEvent),
    [true, false, false],
    'the second main event keeps its mainEvent flag as a regular mosaic card',
  )
  assert.ok(
    result.articles.every((article) => article.id !== result.mainEvent?.id),
    'no mosaic article may repeat the hero',
  )
})

test('getHomeNews keeps the third main event in the mosaic as a regular card', async () => {
  const firstMain = makeItem({
    slug: 'main-first',
    title: 'Перша головна подія',
    main_event: true,
    published_at: localIsoDate(25, 12, 0),
  })
  const secondMain = makeItem({
    slug: 'main-second',
    title: 'Друга головна подія',
    main_event: true,
    published_at: localIsoDate(24, 12, 0),
  })
  const thirdMain = makeItem({
    slug: 'main-third',
    title: 'Третя головна подія',
    main_event: true,
    published_at: localIsoDate(23, 12, 0),
  })
  const regular = makeItem({
    slug: 'regular',
    title: 'Звичайна новина',
    published_at: localIsoDate(22, 12, 0),
  })
  mockResponse({
    items: [thirdMain, regular, firstMain, secondMain],
    page: 1,
    limit: 20,
  })

  const result = asHomeNewsWithMainEvent(await getHomeNews())

  assert.ok(result.mainEvent, 'expected the newest main_event article to become the hero')
  assert.equal(result.mainEvent.id, 'main-first', 'expected the newest main event as the hero')
  assert.deepEqual(
    result.articles.map((article) => article.id),
    ['main-second', 'main-third', 'regular'],
    'expected the second main event to lead and the third to stay in the mosaic pool',
  )
  assert.equal(
    result.articles[1]?.id,
    'main-third',
    'the third main event must remain visible as a regular mosaic card',
  )
  assert.equal(result.articles[1]?.mainEvent, true)

  const mosaicIds = result.articles.map((article) => article.id)
  assert.equal(new Set(mosaicIds).size, mosaicIds.length, 'the mosaic must not repeat an article')
})

test('getHomeNews still caps the deduplicated mosaic at six articles', async () => {
  const items = [
    makeItem({
      slug: 'main-first',
      title: 'Головна подія 1',
      main_event: true,
      published_at: localIsoDate(30, 9, 0),
    }),
    makeItem({
      slug: 'main-second',
      title: 'Головна подія 2',
      main_event: true,
      published_at: localIsoDate(29, 9, 0),
    }),
    ...Array.from({ length: 6 }, (_, index) =>
      makeItem({
        slug: `regular-${index + 3}`,
        title: `Звичайна новина ${index + 3}`,
        published_at: localIsoDate(28 - index, 9, 0),
      }),
    ),
  ]
  mockResponse({ items, page: 1, limit: 20 })

  const result = asHomeNewsWithMainEvent(await getHomeNews())

  assert.ok(result.mainEvent, 'expected the newest main_event article to become the hero')
  assert.equal(result.mainEvent.id, 'main-first')
  assert.equal(result.articles.length, 6, 'expected the mosaic to stay limited to six articles')
  assert.deepEqual(
    result.articles.map((article) => article.id),
    ['main-second', 'regular-3', 'regular-4', 'regular-5', 'regular-6', 'regular-7'],
    'expected the second main event to lead a six-card mosaic without the hero',
  )
  assert.ok(
    result.articles.every((article) => article.id !== result.mainEvent?.id),
    'the hero must stay out of the limited mosaic',
  )

  // Only the mosaic changes: the secondary feeds keep the full page.
  assert.equal(result.latestNews.length, 8, 'expected latestNews to keep every fetched item')
  assert.deepEqual(
    result.tickerItems,
    ['Головна подія 1', 'Головна подія 2', 'Звичайна новина 3', 'Звичайна новина 4', 'Звичайна новина 5'],
    'expected the ticker to stay unchanged',
  )
})

test('getHomeNews returns null mainEvent when no article is selected', async () => {
  const items = [
    makeItem({
      slug: 'featured-only',
      title: 'Лише на головній',
      featured: true,
      main_event: false,
      published_at: localIsoDate(25, 9, 0),
    }),
    makeItem({
      slug: 'regular-only',
      title: 'Без головної події',
      main_event: false,
      published_at: localIsoDate(24, 9, 0),
    }),
  ]
  mockResponse({ items, page: 1, limit: 20 })

  const result = asHomeNewsWithMainEvent(await getHomeNews())

  assert.equal(result.mainEvent, null, 'expected mainEvent to be null without a selected article')
  assert.deepEqual(
    result.articles.map((article) => article.id),
    ['featured-only', 'regular-only'],
  )
  assert.deepEqual(
    result.latestNews.map((item) => item.id),
    ['featured-only', 'regular-only'],
  )
  assert.deepEqual(result.tickerItems, ['Лише на головній', 'Без головної події'])
})

test('getHomeNews selects the newest main_event article regardless of response order', async () => {
  const olderMainEvent = makeItem({
    slug: 'main-event-older',
    category: 'Новини',
    title: 'Старіша головна подія',
    description: 'Опис старішої головної події',
    main_event: true,
    published_at: localIsoDate(22, 9, 0),
  })
  const newerMainEvent = makeItem({
    slug: 'main-event-newer',
    category: 'Спорт',
    title: 'Новіша головна подія',
    description: 'Опис новішої головної події',
    featured: false,
    main_event: true,
    published_at: localIsoDate(25, 17, 30),
  })
  const regular = makeItem({
    slug: 'regular-middle',
    title: 'Звичайна новина',
    main_event: false,
    published_at: localIsoDate(24, 12, 0),
  })

  // Backend ordering is per category, so an older main event can arrive before
  // a newer one; the Hero must use the newest main event, not the first found.
  mockResponse({
    items: [olderMainEvent, newerMainEvent, regular],
    page: 1,
    limit: 20,
  })

  const result = asHomeNewsWithMainEvent(await getHomeNews())

  assert.ok(
    result.mainEvent,
    'expected getHomeNews to map one of the main_event articles into mainEvent',
  )
  assert.equal(
    result.mainEvent.id,
    'main-event-newer',
    'expected the newest main_event article to become the main event',
  )
  assert.equal(result.mainEvent.title, 'Новіша головна подія')
  assert.deepEqual(
    result.articles.map((article) => article.id),
    ['main-event-older', 'regular-middle'],
    'expected the second main event to lead the mosaic while the hero stays out',
  )
})

test('getHomeNews prefers a dated main_event over an undated one', async () => {
  // The undated main event comes first, so taking the first main_event is wrong.
  const undatedMainEvent = makeItem({
    slug: 'main-event-undated',
    title: 'Головна подія без дати',
    main_event: true,
    published_at: null,
  })
  const datedMainEvent = makeItem({
    slug: 'main-event-dated',
    category: 'Міжнародні новини',
    title: 'Головна подія з датою',
    main_event: true,
    published_at: localIsoDate(21, 10, 0),
  })
  mockResponse({
    items: [undatedMainEvent, datedMainEvent],
    page: 1,
    limit: 20,
  })

  const result = asHomeNewsWithMainEvent(await getHomeNews())

  assert.ok(
    result.mainEvent,
    'expected getHomeNews to map one of the main_event articles into mainEvent',
  )
  assert.equal(
    result.mainEvent.id,
    'main-event-dated',
    'an undated main_event article must lose to a dated one',
  )
  assert.deepEqual(
    result.articles.map((article) => article.id),
    ['main-event-undated'],
    'expected the undated second main event to lead the mosaic instead of the hero',
  )
})

test('getHomeNews formats missing or invalid publication dates as empty strings', async () => {
  mockResponse({
    items: [
      makeItem({ slug: 'null-date', published_at: null }),
      makeItem({ slug: 'invalid-date', published_at: 'not-a-date' }),
    ],
    page: 1,
    limit: 20,
  })

  const result = await getHomeNews()

  assert.deepEqual(
    result.articles.map((article) => article.publishedAt),
    ['', ''],
  )
  assert.deepEqual(
    result.latestNews.map((item) => item.time),
    ['', ''],
  )
})

test('getPublicNews uses the detail endpoint and encodes the slug', async () => {
  const cases = [
    { slug: 'economy-1', expectedUrl: `${PUBLIC_NEWS_URL}/economy-1` },
    {
      slug: 'світ/економіка 2026?x=1&y=2',
      expectedUrl: `${PUBLIC_NEWS_URL}/${encodeURIComponent('світ/економіка 2026?x=1&y=2')}`,
    },
  ]

  for (const { slug, expectedUrl } of cases) {
    const detail = makeDetail({
      slug,
      category: 'Міжнародні новини',
      title: 'Заголовок деталі',
      description: 'Опис деталі',
      image_url: 'https://cdn.example.com/detail.jpg',
      featured: true,
      views: 321,
      published_at: localIsoDate(25, 14, 35),
      text: 'Перший абзац.\n\nДругий абзац.',
    })
    const requestedUrls: string[] = []
    globalThis.fetch = async (input) => {
      requestedUrls.push(String(input))
      return new Response(JSON.stringify(detail), { status: 200 })
    }

    const result = await requireFunction(getPublicNews, 'getPublicNews')(slug)

    assert.deepEqual(requestedUrls, [expectedUrl])
    assert.deepEqual(result, detail)
  }
})

test('getPublicNews reports request errors', async () => {
  const getDetail = requireFunction(getPublicNews, 'getPublicNews')

  mockResponse({ error: { message: 'News article not found' } }, 404)
  await assert.rejects(getDetail('missing-slug'), /News article not found/)

  globalThis.fetch = async () => new Response('internal failure', { status: 500 })
  await assert.rejects(getDetail('broken-slug'), /Помилка сервера \(500\)\./)

  globalThis.fetch = async () => {
    throw new TypeError('fetch failed')
  }
  await assert.rejects(getDetail('offline-slug'), /Не вдалося з’єднатися з сервером/)
})

test('getPublicNewsArticle maps a complete article', async () => {
  const text =
    'Перший абзац із <strong>розміткою</strong>.\r\n\r\nДругий абзац.\n  Рядок з відступами  \n\n'
  const detail = makeDetail({
    slug: 'analytics-complete',
    category: 'Міжнародні новини',
    title: 'Повний матеріал',
    description: 'Стислий опис матеріалу',
    image_url: 'https://cdn.example.com/complete.jpg',
    featured: true,
    views: 12400,
    published_at: localIsoDate(25, 14, 35),
    text,
  })
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify(detail), { status: 200 })
  }

  const article = await requireFunction(getPublicNewsArticle, 'getPublicNewsArticle')(detail.slug)

  assert.equal(requestedUrl, `${PUBLIC_NEWS_URL}/analytics-complete`)
  assert.deepEqual(article, {
    id: 'analytics-complete',
    category: 'Міжнародні новини',
    title: 'Повний матеріал',
    excerpt: 'Стислий опис матеріалу',
    views: '12,4\u00A0тис.',
    publishedAt: '25.10.2026, 14:35',
    featured: true,
    mainEvent: false,
    imageUrl: 'https://cdn.example.com/complete.jpg',
    text,
  })
  assert.equal(article.text, text)
  assert.equal(article.text.length, text.length)
})

test('getPublicNewsArticle encodes the slug in the detail endpoint', async () => {
  const slug = 'ринок/огляд 2026?'
  let requestedUrl = ''
  globalThis.fetch = async (input) => {
    requestedUrl = String(input)
    return new Response(JSON.stringify(makeDetail({ slug })), { status: 200 })
  }

  const article = await requireFunction(getPublicNewsArticle, 'getPublicNewsArticle')(slug)

  assert.equal(requestedUrl, `${PUBLIC_NEWS_URL}/${encodeURIComponent(slug)}`)
  assert.equal(article.id, slug)
})

test('getPublicNewsArticle formats views compactly', async () => {
  const cases = [
    { views: 0, expected: '0' },
    { views: 999, expected: '999' },
    { views: 1000, expected: '1\u00A0тис.' },
    { views: 12400, expected: '12,4\u00A0тис.' },
    { views: 1200000, expected: '1,2\u00A0млн' },
  ]

  for (const { views, expected } of cases) {
    mockResponse(makeDetail({ views }))

    const article = await requireFunction(getPublicNewsArticle, 'getPublicNewsArticle')('views-slug')

    assert.equal(article.views, expected)
  }
})

test('getPublicNewsArticle handles missing images and dates', async () => {
  const getArticle = requireFunction(getPublicNewsArticle, 'getPublicNewsArticle')

  mockResponse(makeDetail({ image_url: '', published_at: null, views: 0, text: '' }))
  let article = await getArticle('no-image')

  assert.equal('imageUrl' in article, true)
  assert.equal(article.imageUrl, undefined)
  assert.equal(article.publishedAt, '')
  assert.equal(article.text, '')

  mockResponse(makeDetail({ published_at: 'not-a-date' }))
  article = await getArticle('invalid-date')

  assert.equal(article.publishedAt, '')
})

test('getPublicNewsArticle reports request errors', async () => {
  const getArticle = requireFunction(getPublicNewsArticle, 'getPublicNewsArticle')

  mockResponse({ error: { message: 'News article not found' } }, 404)
  await assert.rejects(getArticle('missing-slug'), /News article not found/)

  globalThis.fetch = async () => {
    throw new TypeError('fetch failed')
  }
  await assert.rejects(getArticle('offline-slug'), /Не вдалося з’єднатися з сервером/)
})
