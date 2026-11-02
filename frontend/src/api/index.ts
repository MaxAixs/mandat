import { getPublicNews, listPublicNews } from '@/api/news'
import type { PublicNews } from '@/api/news'
import type { Article, ArticleDetail, Category, LatestNewsItem } from '@/types'

export interface CategoryNewsData {
  articles: Article[]
  page: number
  limit: number
}

export interface HomeNewsData {
  articles: Article[]
  latestNews: LatestNewsItem[]
  tickerItems: string[]
  mainEvent: Article | null
}

const publishedAtFormatter = new Intl.DateTimeFormat('uk-UA', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

const timeFormatter = new Intl.DateTimeFormat('uk-UA', {
  hour: '2-digit',
  minute: '2-digit',
})

const viewsFormatter = new Intl.NumberFormat('uk-UA', {
  notation: 'compact',
  maximumFractionDigits: 1,
})

// «Головні новини» fills a 3-column mosaic: one featured 2×2 card plus five regular ones.
const MAIN_NEWS_LIMIT = 6
// The ticker keeps its own limit of five newest headlines.
const TICKER_LIMIT = 5

function formatDate(value: string | null, formatter: Intl.DateTimeFormat): string {
  if (!value) return ''

  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : formatter.format(date)
}

export function formatViews(views: number): string {
  return viewsFormatter.format(views)
}

function toArticle(news: PublicNews): Article {
  return {
    id: news.slug,
    category: news.category,
    title: news.title,
    excerpt: news.description,
    views: formatViews(news.views),
    publishedAt: formatDate(news.published_at, publishedAtFormatter),
    featured: news.featured,
    mainEvent: news.main_event,
    imageUrl: news.image_url || undefined,
  }
}

function publicationTimestamp(value: string | null): number {
  if (!value) return Number.NEGATIVE_INFINITY

  const timestamp = Date.parse(value)
  return Number.isNaN(timestamp) ? Number.NEGATIVE_INFINITY : timestamp
}

function toLatestNewsItem(news: PublicNews): LatestNewsItem {
  return {
    id: news.slug,
    time: formatDate(news.published_at, timeFormatter),
    source: news.category,
    important: news.category === 'Інтерв\'ю',
    title: news.title,
  }
}

// getPublicNewsArticle loads and maps one complete public article.
export async function getPublicNewsArticle(slug: string): Promise<ArticleDetail> {
  const news = await getPublicNews(slug)

  return { ...toArticle(news), text: news.text }
}

// getCategoryNews loads one page of a single public rubric.
export async function getCategoryNews(
  category: Category,
  page = 1,
  limit = 20,
): Promise<CategoryNewsData> {
  const response = await listPublicNews(page, limit, category)

  return { articles: response.items.map(toArticle), page: response.page, limit: response.limit }
}

// getCategoryLatestNews loads the newest articles of one rubric for the
// «Останні новини» strip on the category page.
const CATEGORY_LATEST_LIMIT = 5

export async function getCategoryLatestNews(category: Category): Promise<LatestNewsItem[]> {
  const response = await listPublicNews(1, CATEGORY_LATEST_LIMIT, category)

  return response.items.map(toLatestNewsItem)
}

// getHomeNews loads and maps all news-driven content used on the home page.
export async function getHomeNews(): Promise<HomeNewsData> {
  const response = await listPublicNews()
  const chronologicalItems = [...response.items].sort(
    (first, second) => publicationTimestamp(second.published_at) - publicationTimestamp(first.published_at),
  )
  const mainEvent = response.items
    .filter((news) => news.main_event)
    .sort(
      (first, second) => publicationTimestamp(second.published_at) - publicationTimestamp(first.published_at),
    )
    .at(0)
  const featuredFirstItems = [
    ...chronologicalItems.filter((news) => news.featured),
    ...chronologicalItems.filter((news) => !news.featured),
  ]

  return {
    articles: featuredFirstItems.slice(0, MAIN_NEWS_LIMIT).map(toArticle),
    latestNews: chronologicalItems.map(toLatestNewsItem),
    tickerItems: chronologicalItems.slice(0, TICKER_LIMIT).map((news) => news.title),
    mainEvent: mainEvent ? toArticle(mainEvent) : null,
  }
}
