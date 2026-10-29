import type { AuthUser } from '@/types'

const AUTH_API_URL = 'http://localhost:8080/api/auth'

interface ApiErrorPayload {
  error?: {
    message?: string
  }
}

class UnauthorizedError extends Error {}

async function authRequest<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response

  try {
    response = await fetch(`${AUTH_API_URL}${path}`, {
      credentials: 'include',
      ...init,
    })
  } catch {
    throw new Error('Не вдалося з’єднатися з сервером. Спробуйте ще раз.')
  }

  if (!response.ok) {
    if (response.status === 401) throw new UnauthorizedError()

    const errorPayload = (await response.json().catch(() => null)) as ApiErrorPayload | null
    throw new Error(errorPayload?.error?.message ?? `Помилка сервера (${response.status}).`)
  }

  if (response.status === 204) return undefined as T

  return (await response.json()) as T
}

export function login(login: string, password: string): Promise<AuthUser> {
  return authRequest<AuthUser>('/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login, password }),
  })
}

export function logout(): Promise<void> {
  return authRequest<void>('/logout', { method: 'POST' })
}

export async function getCurrentUser(): Promise<AuthUser | null> {
  try {
    return await authRequest<AuthUser>('/me')
  } catch (error) {
    if (error instanceof UnauthorizedError) return null
    throw error
  }
}
