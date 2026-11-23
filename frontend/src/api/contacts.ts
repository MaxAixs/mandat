import type { Contact, ContactFormValues, ContactMoveDirection } from '@/types'

const ADMIN_CONTACTS_API_URL = '/api/admin/contacts'
const PUBLIC_CONTACTS_API_URL = '/api/contacts'

interface ContactListResponse {
  items: Contact[]
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
    if (response.status === 401 && url.startsWith(ADMIN_CONTACTS_API_URL) && typeof window !== 'undefined') {
      window.dispatchEvent(new Event('mandat:unauthorized'))
    }

    const errorPayload = (await response.json().catch(() => null)) as ApiErrorPayload | null
    throw new Error(errorPayload?.error?.message ?? `Помилка сервера (${response.status}).`)
  }

  const text = await response.text()
  return (text ? JSON.parse(text) : undefined) as T
}

function jsonInit(method: 'POST' | 'PUT', payload: unknown): RequestInit {
  return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) }
}

function contactUrl(id: string): string {
  return `${ADMIN_CONTACTS_API_URL}/${encodeURIComponent(id)}`
}

/** Contacts of the public /contacts page, in display order. */
export async function listContacts(): Promise<Contact[]> {
  const response = await request<ContactListResponse>(PUBLIC_CONTACTS_API_URL)
  return response.items
}

/** Contacts for the admin panel, in display order. */
export async function listAdminContacts(): Promise<Contact[]> {
  const response = await request<ContactListResponse>(ADMIN_CONTACTS_API_URL)
  return response.items
}

export function createContact(values: ContactFormValues): Promise<Contact> {
  return request<Contact>(ADMIN_CONTACTS_API_URL, jsonInit('POST', values))
}

export function updateContact(id: string, values: ContactFormValues): Promise<Contact> {
  return request<Contact>(contactUrl(id), jsonInit('PUT', values))
}

export function deleteContact(id: string): Promise<void> {
  return request<void>(contactUrl(id), { method: 'DELETE' })
}

export function moveContact(id: string, direction: ContactMoveDirection): Promise<void> {
  return request<void>(`${contactUrl(id)}/move`, jsonInit('POST', { direction }))
}
