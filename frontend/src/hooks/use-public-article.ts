import { useCallback, useEffect, useRef, useState } from 'react'

import { formatViews, getPublicNewsArticle } from '@/api'
import { getNewsErrorMessage, recordPublicNewsView } from '@/api/news'
import type { ArticleDetail } from '@/types'

export default function usePublicArticle(slug: string | undefined) {
  const [article, setArticle] = useState<ArticleDetail | null>(null)
  const [loading, setLoading] = useState(Boolean(slug))
  const [error, setError] = useState<string | null>(slug ? null : 'Новину не знайдено.')
  const requestId = useRef(0)

  const loadArticle = useCallback(async () => {
    const currentRequest = ++requestId.current

    if (!slug) {
      setArticle(null)
      setLoading(false)
      setError('Новину не знайдено.')
      return
    }

    setArticle(null)
    setLoading(true)
    setError(null)

    try {
      const result = await getPublicNewsArticle(slug)
      if (currentRequest === requestId.current) {
        setArticle(result)

        void recordPublicNewsView(slug)
          .then((views) => {
            if (currentRequest !== requestId.current) return

            setArticle((current) =>
              current?.id === result.id
                ? { ...current, views: formatViews(views) }
                : current,
            )
          })
          .catch(() => undefined)
      }
    } catch (loadError) {
      if (currentRequest === requestId.current) {
        setArticle(null)
        setError(getNewsErrorMessage(loadError))
      }
    } finally {
      if (currentRequest === requestId.current) setLoading(false)
    }
  }, [slug])

  useEffect(() => {
    void loadArticle()
  }, [loadArticle])

  return { article, loading, error, retry: loadArticle }
}
