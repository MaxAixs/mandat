import type { AdminNews, Category, NewsFormValues, NewsStatus, NewsVideo } from '@/types'

const NEWS_API_URL = '/api/admin/news'
const PUBLIC_NEWS_API_URL = '/api/news'

// DEFAULT_SEARCH_LIMIT is the suggestion count used by the header search box.
export const DEFAULT_SEARCH_LIMIT = 7

// MAX_VIDEO_BYTES mirrors the backend cap on one uploaded video (500 MB).
export const MAX_VIDEO_BYTES = 500 * 1024 * 1024
// VIDEO_TYPES are the accepted video file types.
export const VIDEO_TYPES = ['video/mp4', 'video/webm']

// MAX_IMAGES mirrors the backend cap on extra images per article.
export const MAX_IMAGES = 10
// MAX_IMAGE_BYTES mirrors the backend cap on one image (10 MB).
export const MAX_IMAGE_BYTES = 10 * 1024 * 1024
// IMAGE_TYPES are the accepted image file types.
export const IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp', 'image/gif']

const UPLOAD_CANCELLED = 'Завантаження скасовано.'

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
  main_event: boolean
  views: number
  published_at: string | null
}

export interface PublicNewsDetail extends PublicNews {
  text: string
  source?: string
  source_url?: string
  video: NewsVideo | null
  images: string[]
}

/** A presigned direct upload: PUT the file to upload_url, then send key with the article. */
export interface VideoUpload {
  key: string
  upload_url: string
  expires_at: string
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
    response = await fetch(`${baseUrl}${path}`, {
      credentials: 'include',
      ...init,
    })
  } catch {
    throw new Error('Не вдалося з’єднатися з сервером. Перевірте, чи запущено backend.')
  }

  if (!response.ok) {
    if (response.status === 401 && baseUrl === NEWS_API_URL && typeof window !== 'undefined') {
      window.dispatchEvent(new Event('mandat:unauthorized'))
    }

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

export function listNews(
  page = 1,
  limit = 20,
  status?: NewsStatus,
  category?: Category,
): Promise<NewsListResponse> {
  const query = new URLSearchParams({
    page: page.toString(),
    limit: limit.toString(),
  })
  if (status) query.set('status', status)
  if (category) query.set('category', category)

  return request<NewsListResponse>(NEWS_API_URL, `?${query.toString()}`)
}

export function listPublicNews(page = 1, limit = 20, category?: Category): Promise<PublicNewsListResponse> {
  const query = new URLSearchParams({
    page: page.toString(),
    limit: limit.toString(),
  })
  if (category) query.set('category', category)

  return request<PublicNewsListResponse>(PUBLIC_NEWS_API_URL, `?${query.toString()}`)
}

export function getPublicNews(slug: string): Promise<PublicNewsDetail> {
  return request<PublicNewsDetail>(PUBLIC_NEWS_API_URL, `/${encodeURIComponent(slug)}`)
}

export function searchPublicNews(
  query: string,
  limit = DEFAULT_SEARCH_LIMIT,
  opts?: { signal?: AbortSignal },
): Promise<PublicNewsListResponse> {
  const params = new URLSearchParams({
    q: query,
    limit: limit.toString(),
  })

  return request<PublicNewsListResponse>(PUBLIC_NEWS_API_URL, `/search?${params.toString()}`, {
    signal: opts?.signal,
  })
}

export async function recordPublicNewsView(slug: string): Promise<number> {
  const response = await request<{ views: number }>(
    PUBLIC_NEWS_API_URL,
    `/${encodeURIComponent(slug)}/view`,
    { method: 'POST' },
  )

  return response.views
}

export function createNews(payload: NewsFormValues): Promise<AdminNews> {
  return request<AdminNews>(NEWS_API_URL, '', jsonRequest('POST', payload))
}

export function updateNews(slug: string, payload: NewsFormValues): Promise<AdminNews> {
  return request<AdminNews>(NEWS_API_URL, `/${encodeURIComponent(slug)}`, jsonRequest('PUT', payload))
}

// requestVideoUpload asks the backend for a presigned URL to upload file directly to storage.
export function requestVideoUpload(file: { type: string; size: number }): Promise<VideoUpload> {
  return request<VideoUpload>(NEWS_API_URL, '/video-uploads', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content_type: file.type, size: file.size }),
  })
}

// requestImageUpload asks the backend for a presigned URL to upload a image directly to storage.
export function requestImageUpload(file: { type: string; size: number }): Promise<VideoUpload> {
  return request<VideoUpload>(NEWS_API_URL, '/image-uploads', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content_type: file.type, size: file.size }),
  })
}

// uploadFile PUTs file to a presigned URL, reporting progress as a 0..1 fraction.
// XHR is used because fetch cannot report upload progress.
export function uploadFile(
  uploadUrl: string,
  file: Blob,
  onProgress?: (fraction: number) => void,
  signal?: AbortSignal,
): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new Error(UPLOAD_CANCELLED))
      return
    }

    const xhr = new XMLHttpRequest()
    const onAbort = () => xhr.abort()

    xhr.open('PUT', uploadUrl)
    xhr.setRequestHeader('Content-Type', file.type)
    xhr.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable && event.total > 0) onProgress?.(event.loaded / event.total)
    })
    xhr.addEventListener('load', () => {
      signal?.removeEventListener('abort', onAbort)
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve()
      } else {
        reject(new Error(`Не вдалося завантажити файл (${xhr.status}).`))
      }
    })
    xhr.addEventListener('error', () => {
      signal?.removeEventListener('abort', onAbort)
      reject(new Error('Не вдалося завантажити файл: помилка мережі.'))
    })
    xhr.addEventListener('abort', () => {
      signal?.removeEventListener('abort', onAbort)
      reject(new Error(UPLOAD_CANCELLED))
    })
    signal?.addEventListener('abort', onAbort)
    xhr.send(file)
  })
}

export function deleteNews(slug: string): Promise<void> {
  return request<void>(NEWS_API_URL, `/${encodeURIComponent(slug)}`, { method: 'DELETE' })
}

export function getNewsErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Сталася невідома помилка. Спробуйте ще раз.'
}
