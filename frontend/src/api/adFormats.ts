import type { AdFormat, AdFormatFormValues, AdFormatMoveDirection } from '@/types'

const ADMIN_AD_FORMATS_API_URL = '/api/admin/ad-formats'
const PUBLIC_AD_FORMATS_API_URL = '/api/ad-formats'

export interface AdFormatListResponse {
  items: AdFormat[]
}

interface ApiErrorPayload {
  error?: {
    message?: string
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  let response: Response

  try {
    response = await fetch(url, { credentials: 'include', ...init })
  } catch {
    throw new Error('Не вдалося з’єднатися з сервером. Перевірте, чи запущено backend.')
  }

  if (!response.ok) {
    if (response.status === 401 && url.startsWith(ADMIN_AD_FORMATS_API_URL) && typeof window !== 'undefined') {
      window.dispatchEvent(new Event('mandat:unauthorized'))
    }

    const errorPayload = (await response.json().catch(() => null)) as ApiErrorPayload | null
    throw new Error(errorPayload?.error?.message ?? `Помилка сервера (${response.status}).`)
  }

  const text = await response.text()
  return (text ? JSON.parse(text) : undefined) as T
}

/** Multipart body of a card; the browser sets the Content-Type with its boundary. */
function formBody(values: AdFormatFormValues): FormData {
  const body = new FormData()
  body.append('title', values.title)
  body.append('description', values.description)
  if (values.image) body.append('image', values.image)
  if (values.removeImage) body.append('remove_image', 'true')

  return body
}

function cardUrl(id: string): string {
  return `${ADMIN_AD_FORMATS_API_URL}/${encodeURIComponent(id)}`
}

/** Cards of the public advertising page, in display order. */
export async function listAdFormats(): Promise<AdFormat[]> {
  const response = await request<AdFormatListResponse>(PUBLIC_AD_FORMATS_API_URL)
  return response.items
}

/** Cards for the admin panel, in display order. */
export async function listAdminAdFormats(): Promise<AdFormat[]> {
  const response = await request<AdFormatListResponse>(ADMIN_AD_FORMATS_API_URL)
  return response.items
}

export function createAdFormat(values: AdFormatFormValues): Promise<AdFormat> {
  return request<AdFormat>(ADMIN_AD_FORMATS_API_URL, { method: 'POST', body: formBody(values) })
}

export function updateAdFormat(id: string, values: AdFormatFormValues): Promise<AdFormat> {
  return request<AdFormat>(cardUrl(id), { method: 'PUT', body: formBody(values) })
}

export function deleteAdFormat(id: string): Promise<void> {
  return request<void>(cardUrl(id), { method: 'DELETE' })
}

export function moveAdFormat(id: string, direction: AdFormatMoveDirection): Promise<void> {
  return request<void>(`${cardUrl(id)}/move`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ direction }),
  })
}
