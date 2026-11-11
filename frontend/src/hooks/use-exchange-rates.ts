import { useCallback, useEffect, useRef, useState } from 'react'

import { fetchExchangeRates, formatRateText, getExchangeErrorMessage } from '@/api/exchange'

const emptyItems: string[] = []

export default function useExchangeRates() {
  const [items, setItems] = useState<string[]>(emptyItems)
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
        setItems(result.items.map((item) => formatRateText(item.code, item.rate)))
        setDate(result.date)
      }
    } catch (loadError) {
      if (currentRequest === requestId.current) {
        setItems(emptyItems)
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

  return { items, date, loading, error, retry: loadRates }
}
