import { useCallback, useEffect, useRef, useState } from 'react'

import { getCategoryNews } from '@/api'
import type { CategoryNewsData } from '@/api'
import { getNewsErrorMessage } from '@/api/news'
import type { Category } from '@/types'

const CATEGORY_PAGE_LIMIT = 20

export default function useCategoryNews(category: Category) {
  const [data, setData] = useState<CategoryNewsData>({
    articles: [],
    page: 1,
    limit: CATEGORY_PAGE_LIMIT,
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
      const result = await getCategoryNews(category, page, CATEGORY_PAGE_LIMIT)
      if (currentRequest === requestId.current) setData(result)
    } catch (loadError) {
      if (currentRequest === requestId.current) {
        setData({ articles: [], page, limit: CATEGORY_PAGE_LIMIT })
        setError(getNewsErrorMessage(loadError))
      }
    } finally {
      if (currentRequest === requestId.current) setLoading(false)
    }
  }, [category, page])

  useEffect(() => {
    setPage(1)
  }, [category])

  useEffect(() => {
    void loadNews()
  }, [loadNews])

  const hasNextPage = data.articles.length === CATEGORY_PAGE_LIMIT
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
