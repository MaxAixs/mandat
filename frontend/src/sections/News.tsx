import { ChevronLeft, ChevronRight, LoaderCircle, RefreshCw } from 'lucide-react'

import NewsCard from '@/components/NewsCard'
import { Button } from '@/components/ui/button'
import useCategoryNews from '@/hooks/use-category-news'

const HOME_NEWS_PAGE_LIMIT = 6

export default function News() {
  const { articles, loading, error, page, hasNextPage, retry, nextPage, previousPage } =
    useCategoryNews(null, HOME_NEWS_PAGE_LIMIT)

  return (
    <section id="news" className="py-[72px]">
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="flex items-baseline justify-between mb-6">
          <h2 className="text-[20px] sm:text-[30px] tracking-[1px] font-bold">
            <span className="inline-block w-2 h-7 bg-mandat-red-bright mr-3.5 align-middle" />
            Головні новини
          </h2>
        </div>

        {loading ? (
          <div className="flex items-center justify-center gap-3 min-h-40 border border-mandat-line bg-mandat-surface text-mandat-muted">
            <LoaderCircle className="size-5 animate-spin" aria-hidden="true" />
            <span>Завантаження новин…</span>
          </div>
        ) : error ? (
          <div className="flex flex-col items-center justify-center gap-4 min-h-40 border border-mandat-line bg-mandat-surface px-6 text-center">
            <p role="alert" className="text-mandat-muted">
              {error}
            </p>
            <button
              type="button"
              onClick={() => void retry()}
              className="inline-flex items-center gap-2 border border-mandat-line px-4 py-2 text-mandat-text transition-colors hover:border-mandat-red-bright"
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              Спробувати ще раз
            </button>
          </div>
        ) : articles.length === 0 ? (
          <div className="flex items-center justify-center min-h-40 border border-mandat-line bg-mandat-surface px-6 text-center text-mandat-muted">
            Опублікованих новин поки немає.
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {articles.map((article, index) => (
              <NewsCard
                key={article.id}
                article={article}
                large={index === 0 ? Boolean(article.featured || article.mainEvent) : false}
              />
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
      </div>
    </section>
  )
}
