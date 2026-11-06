import { useCallback, useEffect, useRef, useState } from 'react'

import { getFeedNews } from '@/api'
import type { CategoryNewsData } from '@/api'
import { getNewsErrorMessage } from '@/api/news'
import type { Category } from '@/types'

const DEFAULT_PAGE_LIMIT = 20

export default function useCategoryNews(category: Category | null, limit = DEFAULT_PAGE_LIMIT) {
  const [data, setData] = useState<CategoryNewsData>({
    articles: [],
    page: 1,
    limit,
  })
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const requestId = useRef(0)

  const loadNews = useCallback(async () => {
    const currentRequest = ++requestId.current
    setLoading(true)
    setError(null)

    try {
      const result = await getFeedNews(category, page, limit)
      if (currentRequest === requestId.current) setData(result)
    } catch (loadError) {
      if (currentRequest === requestId.current) {
        setData({ articles: [], page, limit })
        setError(getNewsErrorMessage(loadError))
      }
    } finally {
      if (currentRequest === requestId.current) setLoading(false)
    }
  }, [category, page, limit])

  useEffect(() => {
    setPage(1)
  }, [category])

  useEffect(() => {
    void loadNews()
  }, [loadNews])

  const hasNextPage = data.articles.length === limit
  const nextPage = useCallback(() => {
    if (hasNextPage) setPage((current) => current + 1)
  }, [hasNextPage])
  const previousPage = useCallback(() => {
    setPage((current) => Math.max(1, current - 1))
  }, [])

  return {
    ...data,
    loading,
    error,
    page,
    hasNextPage,
    retry: loadNews,
    nextPage,
    previousPage,
  }
}
