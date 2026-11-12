const FUEL_API_URL = 'http://localhost:8080/api/fuel'

export interface FuelRateItem {
  code: string
  rate: number
}

export interface FuelRatesResponse {
  base: string
  unit: string
  date: string
  items: FuelRateItem[]
}

interface ApiErrorPayload {
  error?: {
    message?: string
  }
}

export async function fetchFuelRates(): Promise<FuelRatesResponse> {
  let response: Response

  try {
    response = await fetch(`${FUEL_API_URL}/rates`, {
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
  return (text ? JSON.parse(text) : undefined) as FuelRatesResponse
}

export function getFuelErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Сталася невідома помилка. Спробуйте ще раз.'
}
