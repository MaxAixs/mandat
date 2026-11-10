import { useState } from 'react'
import { ChevronLeft, ChevronRight, LoaderCircle, RefreshCw } from 'lucide-react'
import { Link } from 'react-router'

import { Button } from '@/components/ui/button'
import { CATEGORIES } from '@/constants'
import useCategoryNews from '@/hooks/use-category-news'
import { cn } from '@/lib/utils'
import Footer from '@/sections/Footer'
import Header from '@/sections/Header'
import type { Category } from '@/types'

type FeedFilter = Category | null

// Rubric filter buttons: every news rubric except «Про нас» (an about page,
// not news). Single source is CATEGORIES, so the next rubric appears here
// automatically.
const FILTERS: { label: string; value: FeedFilter }[] = [
  { label: 'Всі новини', value: null },
  ...CATEGORIES.filter((c) => c !== 'Про нас').map((c) => ({ label: c, value: c })),
]

export default function Feed() {
  const [filter, setFilter] = useState<FeedFilter>(null)
  const {
    articles,
    loading,
    error,
    page,
    hasNextPage,
    retry,
    nextPage,
    previousPage,
  } = useCategoryNews(filter)

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />
      <main className="mx-auto min-h-[60vh] max-w-[1240px] px-6 py-[72px]">
        <div className="mb-10">
          <h1 className="text-[30px] font-bold tracking-[1px] md:text-[42px]">
            <span className="inline-block w-2 h-8 bg-mandat-red-bright mr-3.5 align-middle md:h-10" />
            {filter ?? 'Всі новини'}
          </h1>
        </div>

        <div className="mb-10 flex flex-wrap gap-2.5" role="group" aria-label="Фільтр новин">
          {FILTERS.map((option) => (
            <button
              key={option.label}
              type="button"
              aria-pressed={filter === option.value}
              onClick={() => setFilter(option.value)}
              className={cn(
                'border px-4 py-2 font-display text-[13px] uppercase tracking-[1px] transition-colors',
                filter === option.value
                  ? 'border-mandat-red-bright text-mandat-text'
                  : 'border-mandat-line text-mandat-muted hover:border-mandat-red-bright hover:text-mandat-text',
              )}
            >
              {option.label}
            </button>
          ))}
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
          <div className="bg-mandat-surface border border-mandat-line divide-y divide-mandat-line">
            {articles.map((article) => (
              <Link
                key={article.id}
                to={`/news/${encodeURIComponent(article.id)}`}
                className="group block px-6 py-4 no-underline transition-colors hover:bg-mandat-surface-2"
              >
                <div className="text-[12px] tracking-[1px] text-mandat-muted uppercase">
                  <span>{article.publishedAt}</span>
                  <span> — {article.category}</span>
                </div>
                <div className="mt-1.5 text-[15px] font-semibold leading-snug text-mandat-text group-hover:text-mandat-red-bright transition-colors">
                  {article.title}
                </div>
              </Link>
            ))}
          </div>
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
      <Footer />
    </div>
  )
}
