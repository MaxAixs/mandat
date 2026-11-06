import { Link } from 'react-router'

import type { Article } from '@/types'

const HERO_NEWS_COUNT = 5

interface HeroProps {
  articles: Article[]
}

export default function Hero({ articles }: HeroProps) {
  const heroArticles = articles.slice(0, HERO_NEWS_COUNT)

  if (heroArticles.length === 0) return null

  return (
    <section aria-label="Головні новини" className="border-b border-mandat-line p-4 sm:p-6 lg:p-8">
      <h1 className="sr-only">Головні новини</h1>
      <div className="grid gap-3 md:[@media(hover:hover)]:flex md:[@media(hover:hover)]:h-[420px] lg:[@media(hover:hover)]:h-[500px]">
        {heroArticles.map((article) => (
          <Link
            key={article.id}
            to={`/news/${encodeURIComponent(article.id)}`}
            aria-label={`Читати новину: ${article.title}`}
            className="group relative min-h-[300px] overflow-hidden border border-mandat-line bg-mandat-surface text-mandat-text no-underline transition-[flex-grow] duration-500 ease-out motion-reduce:transition-none focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-mandat-red-bright md:[@media(hover:hover)]:min-h-0 md:[@media(hover:hover)]:flex-1 md:[@media(hover:hover)]:hover:flex-[3] md:[@media(hover:hover)]:focus-visible:flex-[3]"
          >
            {article.imageUrl && (
              <img
                src={article.imageUrl}
                alt=""
                className="absolute inset-0 h-full w-full object-cover transition-transform duration-700 ease-out motion-reduce:transition-none group-hover:scale-105 group-focus-visible:scale-105 motion-reduce:group-hover:scale-100 motion-reduce:group-focus-visible:scale-100"
              />
            )}
            <div className="absolute inset-0 bg-gradient-to-t from-mandat-bg to-transparent" />

            <div className="absolute inset-x-0 bottom-0 p-5 lg:p-6">
              <div className="md:[@media(hover:hover)]:translate-y-4 md:[@media(hover:hover)]:opacity-0 transition-all md:duration-300 motion-reduce:transition-none md:group-hover:translate-y-0 md:group-hover:opacity-100 md:group-focus-visible:translate-y-0 md:group-focus-visible:opacity-100">
                <span className="mb-3 inline-block bg-mandat-red px-3 py-1 font-display text-[11px] uppercase tracking-[2px] text-mandat-text">
                  {article.category}
                </span>
                <h2 className="break-words text-[22px] font-bold leading-[1.15] lg:text-[26px]">{article.title}</h2>
                <p className="mt-3 line-clamp-3 max-w-[560px] text-[15px] text-mandat-muted">
                  {article.excerpt}
                </p>
              </div>
            </div>
          </Link>
        ))}
      </div>
    </section>
  )
}
