import type { Article } from '@/types'

export default function NewsCard({ article }: { article: Article }) {
  const featured = article.featured ?? false

  return (
    <article
      className={`border border-mandat-line bg-mandat-surface transition-all duration-200 hover:border-mandat-red-bright hover:-translate-y-1 ${
        featured ? 'lg:col-span-2 lg:row-span-2 flex flex-col' : ''
      }`}
    >
      <div className={`relative overflow-hidden ${featured ? 'h-[340px]' : 'h-[180px]'}`}>
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
        <span className="absolute top-3.5 left-3.5 z-10 font-['Oswald'] text-[11px] tracking-[2px] uppercase px-3 py-1 bg-mandat-red text-mandat-text">
          {article.category}
        </span>
      </div>
      <div className="p-5 pb-4 flex flex-col flex-1">
        <h3 className={`font-semibold leading-tight tracking-wide ${featured ? 'text-[30px]' : 'text-[19px]'}`}>
          {article.title}
        </h3>
        <p className={`text-mandat-muted mt-2.5 ${featured ? 'text-[15px]' : 'text-[14px]'}`}>{article.excerpt}</p>
        <div className="flex gap-4 mt-4 pt-3.5 border-t border-mandat-line text-mandat-muted text-[12.5px] tracking-wider uppercase mt-auto">
          <span className="text-mandat-red-bright">◉ {article.views}</span>
          <span>{article.publishedAt}</span>
          {article.comments && <span>{article.comments}</span>}
        </div>
      </div>
    </article>
  )
}
