import type { AdminNews, Category, NewsFormValues, NewsStatus } from '@/types'

const NEWS_API_URL = 'http://localhost:8080/api/admin/news'
const PUBLIC_NEWS_API_URL = 'http://localhost:8080/api/news'

export interface NewsListResponse {
  items: AdminNews[]
  page: number
  limit: number
}

export interface PublicNews {
  slug: string
  category: Category
  title: string
  description: string
  image_url: string
  featured: boolean
  views: number
  published_at: string | null
}

export interface PublicNewsDetail extends PublicNews {
  text: string
}

export interface PublicNewsListResponse {
  items: PublicNews[]
  page: number
  limit: number
}

interface ApiErrorPayload {
  error?: {
    message?: string
  }
}

async function request<T>(baseUrl: string, path: string, init?: RequestInit): Promise<T> {
  let response: Response

  try {
    response = await fetch(`${baseUrl}${path}`, init)
  } catch {
    throw new Error('Не вдалося з’єднатися з сервером. Перевірте, чи запущено backend.')
  }

  if (!response.ok) {
    const errorPayload = (await response.json().catch(() => null)) as ApiErrorPayload | null
    throw new Error(errorPayload?.error?.message ?? `Помилка сервера (${response.status}).`)
  }

  const text = await response.text()
  return (text ? JSON.parse(text) : undefined) as T
}

function jsonRequest(method: 'POST' | 'PUT', payload: NewsFormValues): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  }
}

export function listNews(page = 1, limit = 20, status?: NewsStatus): Promise<NewsListResponse> {
  const query = new URLSearchParams({
    page: page.toString(),
    limit: limit.toString(),
  })
  if (status) query.set('status', status)

  return request<NewsListResponse>(NEWS_API_URL, `?${query.toString()}`)
}

export function listPublicNews(page = 1, limit = 20): Promise<PublicNewsListResponse> {
  const query = new URLSearchParams({
    page: page.toString(),
    limit: limit.toString(),
  })

  return request<PublicNewsListResponse>(PUBLIC_NEWS_API_URL, `?${query.toString()}`)
}

export function getPublicNews(slug: string): Promise<PublicNewsDetail> {
  return request<PublicNewsDetail>(PUBLIC_NEWS_API_URL, `/${encodeURIComponent(slug)}`)
}

export function createNews(payload: NewsFormValues): Promise<AdminNews> {
  return request<AdminNews>(NEWS_API_URL, '', jsonRequest('POST', payload))
}

export function updateNews(slug: string, payload: NewsFormValues): Promise<AdminNews> {
  return request<AdminNews>(NEWS_API_URL, `/${encodeURIComponent(slug)}`, jsonRequest('PUT', payload))
}

export function deleteNews(slug: string): Promise<void> {
  return request<void>(NEWS_API_URL, `/${encodeURIComponent(slug)}`, { method: 'DELETE' })
}

export function getNewsErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Сталася невідома помилка. Спробуйте ще раз.'
}
