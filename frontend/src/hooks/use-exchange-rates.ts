import { useCallback, useEffect, useRef, useState } from 'react'

import { fetchExchangeRates, getExchangeErrorMessage } from '@/api/exchange'
import type { ExchangeRateItem } from '@/api/exchange'

const emptyRates: ExchangeRateItem[] = []

export default function useExchangeRates() {
  const [rates, setRates] = useState<ExchangeRateItem[]>(emptyRates)
  const [date, setDate] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const requestId = useRef(0)

  const loadRates = useCallback(async () => {
    const currentRequest = ++requestId.current
    setLoading(true)
    setError(null)

    try {
      const result = await fetchExchangeRates()
      if (currentRequest === requestId.current) {
        setRates(result.items)
        setDate(result.date)
      }
    } catch (loadError) {
      if (currentRequest === requestId.current) {
        setRates(emptyRates)
        setDate('')
        setError(getExchangeErrorMessage(loadError))
      }
    } finally {
      if (currentRequest === requestId.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadRates()
  }, [loadRates])

  return { rates, date, loading, error, retry: loadRates }
}
