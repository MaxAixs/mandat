import { useCallback, useEffect, useRef, useState } from 'react'

import { fetchFuelRates, getFuelErrorMessage } from '@/api/fuel'
import type { FuelRateItem } from '@/api/fuel'

const emptyRates: FuelRateItem[] = []

export default function useFuelPrices() {
  const [rates, setRates] = useState<FuelRateItem[]>(emptyRates)
  const [date, setDate] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const requestId = useRef(0)

  const loadRates = useCallback(async () => {
    const currentRequest = ++requestId.current
    setLoading(true)
    setError(null)

    try {
      const result = await fetchFuelRates()
      if (currentRequest === requestId.current) {
        setRates(result.items)
        setDate(result.date)
      }
    } catch (loadError) {
      if (currentRequest === requestId.current) {
        setRates(emptyRates)
        setDate('')
        setError(getFuelErrorMessage(loadError))
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
