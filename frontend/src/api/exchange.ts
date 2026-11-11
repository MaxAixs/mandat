const EXCHANGE_API_URL = 'http://localhost:8080/api/exchange'

export interface ExchangeRateItem {
  code: string
  rate: number
}

export interface ExchangeRatesResponse {
  base: string
  date: string
  items: ExchangeRateItem[]
}

interface ApiErrorPayload {
  error?: {
    message?: string
  }
}

export function formatRateText(code: string, rate: number): string {
  return `UAH -> ${code} = ${rate.toFixed(2)}`
}

export async function fetchExchangeRates(): Promise<ExchangeRatesResponse> {
  let response: Response

  try {
    response = await fetch(`${EXCHANGE_API_URL}/rates`, {
      credentials: 'include',
    })
  } catch {
    throw new Error('Не вдалося з’єднатися з сервером. Перевірте, чи запущено backend.')
  }

  if (!response.ok) {
    const errorPayload = (await response.json().catch(() => null)) as ApiErrorPayload | null
    throw new Error(errorPayload?.error?.message ?? `Помилка сервера (${response.status}).`)
  }

  const text = await response.text()
  return (text ? JSON.parse(text) : undefined) as ExchangeRatesResponse
}

export function getExchangeErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Сталася невідома помилка. Спробуйте ще раз.'
}
