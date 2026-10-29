import { getPublicNews, listPublicNews } from '@/api/news'
import type { PublicNews } from '@/api/news'
import type { Article, ArticleDetail, LatestNewsItem } from '@/types'

export interface HomeNewsData {
  articles: Article[]
  latestNews: LatestNewsItem[]
  tickerItems: string[]
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

// getHomeNews loads and maps all news-driven content used on the home page.
export async function getHomeNews(): Promise<HomeNewsData> {
  const response = await listPublicNews()
  const chronologicalItems = [...response.items].sort(
    (first, second) => publicationTimestamp(second.published_at) - publicationTimestamp(first.published_at),
  )

  return {
    articles: response.items.map(toArticle),
    latestNews: chronologicalItems.map(toLatestNewsItem),
    tickerItems: chronologicalItems.slice(0, 5).map((news) => news.title),
  }
}
