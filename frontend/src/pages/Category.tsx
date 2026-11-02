import { useEffect, useState } from 'react'
import { ChevronLeft, ChevronRight, LoaderCircle, RefreshCw } from 'lucide-react'

import { getCategoryLatestNews } from '@/api'
import NewsCard from '@/components/NewsCard'
import { Button } from '@/components/ui/button'
import useCategoryNews from '@/hooks/use-category-news'
import Footer from '@/sections/Footer'
import Header from '@/sections/Header'
import LatestNews from '@/sections/LatestNews'
import type { Category, LatestNewsItem } from '@/types'

export default function CategoryPage({ category }: { category: Category }) {
  const {
    articles,
    loading,
    error,
    page,
    hasNextPage,
    retry,
    nextPage,
    previousPage,
  } = useCategoryNews(category)
  const [latest, setLatest] = useState<{ category: Category; items: LatestNewsItem[] } | null>(null)

  useEffect(() => {
    let cancelled = false

    getCategoryLatestNews(category).then(
      (items) => {
        if (!cancelled) setLatest({ category, items })
      },
      () => {
        // The strip stays hidden when the feed is unavailable.
      },
    )

    return () => {
      cancelled = true
    }
  }, [category])

  const main = articles.find((article) => article.mainEvent)

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />
      <main className="mx-auto min-h-[60vh] max-w-[1240px] px-6 py-[72px]">
        <div className="mb-10">
          <div className="mb-2 text-[11px] font-semibold tracking-[0.22em] text-mandat-red-bright">
            КАТЕГОРІЯ
          </div>
          <h1 className="text-[30px] font-bold tracking-[1px] md:text-[42px]">{category}</h1>
        </div>

        {loading ? (
          <div role="status" aria-live="polite" className="flex min-h-40 items-center justify-center gap-3 border border-mandat-line bg-mandat-surface text-mandat-muted">
            <LoaderCircle className="size-5 animate-spin" aria-hidden="true" />
            <span>Завантаження новин…</span>
          </div>
        ) : error ? (
          <div className="flex min-h-40 flex-col items-center justify-center gap-4 border border-mandat-line bg-mandat-surface px-6 text-center">
            <p role="alert" className="text-mandat-muted">{error}</p>
            <Button
              variant="outline"
              onClick={() => void retry()}
              className="rounded-none border-mandat-line bg-mandat-bg text-mandat-text hover:bg-mandat-surface-2"
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              Спробувати ще раз
            </Button>
          </div>
        ) : articles.length === 0 ? (
          <div className="flex min-h-40 items-center justify-center border border-mandat-line bg-mandat-surface px-6 text-center text-mandat-muted">
            Опублікованих новин поки немає.
          </div>
        ) : (
          <>
            {main && (
              <div className="mb-6">
                <NewsCard article={main} large />
              </div>
            )}
            <div className="grid grid-cols-1 items-start gap-6 md:grid-cols-2 lg:grid-cols-3">
              {articles.map((article) => (
                <NewsCard key={article.id} article={article} />
              ))}
            </div>
          </>
        )}

        <div className="mt-8 flex items-center justify-between border-t border-mandat-line pt-5 text-sm text-mandat-muted">
          <Button
            variant="outline"
            size="sm"
            disabled={page === 1 || loading}
            onClick={previousPage}
            className="rounded-none border-mandat-line bg-mandat-bg text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
          >
            <ChevronLeft className="size-4" />
            Назад
          </Button>
          <span>Сторінка {page}</span>
          <Button
            variant="outline"
            size="sm"
            disabled={!hasNextPage || loading}
            onClick={nextPage}
            className="rounded-none border-mandat-line bg-mandat-bg text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
          >
            Далі
            <ChevronRight className="size-4" />
          </Button>
        </div>
      </main>
      <LatestNews items={latest !== null && latest.category === category ? latest.items : []} />
      <Footer />
    </div>
  )
}
