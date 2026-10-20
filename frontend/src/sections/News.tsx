import { mockArticles } from '@/api/mock'
import NewsCard from '@/components/NewsCard'

export default function News() {
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
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {mockArticles.map((article) => (
            <NewsCard key={article.id} article={article} />
          ))}
        </div>
      </div>
    </section>
  )
}
