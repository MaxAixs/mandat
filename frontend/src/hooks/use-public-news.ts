import { useCallback, useEffect, useRef, useState } from 'react'

import { getHomeNews } from '@/api'
import type { HomeNewsData } from '@/api'
import { getNewsErrorMessage } from '@/api/news'

const emptyNews: HomeNewsData = {
  articles: [],
  latestNews: [],
  tickerItems: [],
  mainEvent: null,
}

export default function usePublicNews() {
  const [data, setData] = useState<HomeNewsData>(emptyNews)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const requestId = useRef(0)

  const loadNews = useCallback(async () => {
    const currentRequest = ++requestId.current
    setLoading(true)
    setError(null)

    try {
      const result = await getHomeNews()
      if (currentRequest === requestId.current) setData(result)
    } catch (loadError) {
      if (currentRequest === requestId.current) {
        setData(emptyNews)
        setError(getNewsErrorMessage(loadError))
      }
    } finally {
      if (currentRequest === requestId.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadNews()
  }, [loadNews])

  return { ...data, loading, error, retry: loadNews }
}
