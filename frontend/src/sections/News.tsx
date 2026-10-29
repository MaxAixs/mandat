import { LoaderCircle, RefreshCw } from 'lucide-react'

import NewsCard from '@/components/NewsCard'
import type { Article } from '@/types'

interface NewsProps {
  articles: Article[]
  loading: boolean
  error: string | null
  onRetry: () => void
}

export default function News({ articles, loading, error, onRetry }: NewsProps) {
  return (
    <section id="news" className="py-[72px]">
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="flex items-baseline justify-between mb-10">
          <h2 className="text-[36px] tracking-[2px] font-semibold">
            <span className="inline-block w-2 h-7 bg-mandat-red-bright mr-3.5 align-middle" />
            Головні новини
          </h2>
          <a
            href="#"
            className="font-['Oswald'] text-[14px] tracking-[2px] uppercase text-mandat-muted hover:text-mandat-red-bright transition-colors no-underline"
          >
            Всі новини →
          </a>
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
              onClick={() => void onRetry()}
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
          <div className="grid grid-cols-1 items-start gap-6 md:grid-cols-2 lg:grid-cols-3">
            {articles.map((article) => (
              <NewsCard key={article.id} article={article} />
            ))}
          </div>
        )}
      </div>
    </section>
  )
}
