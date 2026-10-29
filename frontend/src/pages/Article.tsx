import { useEffect } from 'react'
import { CalendarDays, Eye, LoaderCircle, RefreshCw } from 'lucide-react'
import { useParams } from 'react-router'

import usePublicArticle from '@/hooks/use-public-article'
import Footer from '@/sections/Footer'
import Header from '@/sections/Header'
import type { ArticleDetail } from '@/types'

export function ArticleContent({ article }: { article: ArticleDetail }) {
  return (
    <article className="w-full pb-8 md:pb-12">
      <div className="mx-auto w-full max-w-[900px] px-6">
        <div className="relative mt-6 h-[280px] w-full overflow-hidden border-y border-mandat-line bg-mandat-surface-2 sm:h-[340px] md:mt-8 md:h-[380px]">
          {article.imageUrl ? (
            <img
              src={article.imageUrl}
              alt={article.title}
              className="h-full w-full object-cover"
            />
          ) : (
            <>
              <div
                className="absolute inset-0"
                style={{
                  background: `
                    linear-gradient(160deg, rgba(23,25,28,0.2), rgba(23,25,28,0.85)),
                    repeating-linear-gradient(45deg, #262a31 0 14px, #1d2025 14px 28px)
                  `,
                }}
              />
              <div
                className="absolute inset-0"
                style={{ background: 'radial-gradient(circle at 80% 20%, rgba(179,35,58,0.35), transparent 55%)' }}
              />
            </>
          )}
        </div>

        <h1 className="mt-6 text-[32px] font-semibold leading-tight tracking-wide md:mt-8 md:text-[48px]">
          {article.title}
        </h1>

        <div className="mt-5 flex flex-wrap items-center justify-end gap-x-6 gap-y-2 border-b border-mandat-line pb-6 text-[13px] uppercase tracking-wider text-mandat-muted">
          <span className="inline-flex items-center gap-2">
            <CalendarDays className="size-4" aria-hidden="true" />
            {article.publishedAt}
          </span>
          <span className="inline-flex items-center gap-2 text-mandat-red-bright">
            <Eye className="size-4" aria-hidden="true" />
            {article.views}
          </span>
        </div>
        <div className="mt-8 whitespace-pre-line text-[17px] leading-8 text-mandat-text">
          {article.text}
        </div>
      </div>
    </article>
  )
}

export default function Article() {
  const { slug } = useParams<{ slug: string }>()
  const { article, loading, error, retry } = usePublicArticle(slug)

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'instant' })
  }, [slug])

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />
      <main className="min-h-[60vh]">
        {loading ? (
          <div role="status" aria-live="polite" className="flex min-h-[50vh] items-center justify-center gap-3 px-6 text-mandat-muted">
            <LoaderCircle className="size-5 animate-spin" aria-hidden="true" />
            <span>Завантаження новини…</span>
          </div>
        ) : error ? (
          <div className="mx-auto flex min-h-[50vh] max-w-[900px] flex-col items-center justify-center gap-5 px-6 text-center">
            <p role="alert" className="text-mandat-muted">{error}</p>
            <button
              type="button"
              disabled={loading}
              onClick={() => void retry()}
              className="inline-flex items-center gap-2 border border-mandat-line px-4 py-2 text-mandat-text transition-colors hover:border-mandat-red-bright disabled:cursor-not-allowed disabled:opacity-50"
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              Спробувати ще раз
            </button>
          </div>
        ) : article ? (
          <ArticleContent article={article} />
        ) : null}
      </main>
      <Footer />
    </div>
  )
}
