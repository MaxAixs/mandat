import type { TelegramChannelStats } from '@/types'

const TELEGRAM_CHANNEL_API_URL = '/api/telegram/channel'

interface ApiErrorPayload {
  error?: {
    message?: string
  }
}

/** Subscriber count of the Telegram channel; rejects while the backend has no value yet. */
export async function getTelegramChannel(): Promise<TelegramChannelStats> {
  let response: Response

  try {
    response = await fetch(TELEGRAM_CHANNEL_API_URL, { credentials: 'include' })
  } catch {
    throw new Error('Не вдалося з’єднатися з сервером. Перевірте, чи запущено backend.')
  }

  if (!response.ok) {
    const errorPayload = (await response.json().catch(() => null)) as ApiErrorPayload | null
    throw new Error(errorPayload?.error?.message ?? `Помилка сервера (${response.status}).`)
  }

  const stats = (await response.json()) as Partial<TelegramChannelStats> | null
  // A malformed answer must not reach the page as «NaN»: the caller keeps its placeholder.
  if (typeof stats?.subscribers !== 'number' || !Number.isFinite(stats.subscribers)) {
    throw new Error('Некоректна відповідь сервера.')
  }

  return stats as TelegramChannelStats
}
