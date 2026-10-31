import { cn } from '@/lib/utils'
import type { LatestNewsItem } from '@/types'

export default function LatestNews({ items }: { items: LatestNewsItem[] }) {
  if (items.length === 0) return null

  return (
    <section id="latest" className="pb-[72px]">
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="flex items-baseline justify-between mb-10">
          <h2 className="text-[20px] sm:text-[30px] tracking-[1px] font-bold">
            <span className="inline-block w-2 h-7 bg-mandat-red-bright mr-3.5 align-middle" />
            Останні новини
          </h2>
          <a
            href="#"
            className="font-display shrink-0 whitespace-nowrap text-[11px] sm:text-[14px] tracking-[0.5px] sm:tracking-[2px] uppercase text-mandat-muted hover:text-mandat-red-bright transition-colors no-underline"
          >
            Всі новини →
          </a>
        </div>

        <div className="bg-mandat-surface border border-mandat-line divide-y divide-mandat-line">
          {items.map((item) => (
            <a
              key={item.id}
              href="#"
              className="group block px-6 py-4 no-underline transition-colors hover:bg-mandat-surface-2"
            >
              <div className="text-[12px] tracking-[1px] text-mandat-muted uppercase">
                <span>{item.time}</span>
                {item.source && <span> — {item.source}</span>}
              </div>
              <div className="mt-1.5 text-[15px] font-semibold leading-snug text-mandat-text group-hover:text-mandat-red-bright transition-colors">
                {item.important && (
                  <span className="inline-block align-middle mr-2 px-2 py-0.5 rounded-full bg-mandat-red-bright text-mandat-text text-[10px] font-display uppercase tracking-[1px]">
                    Важливо
                  </span>
                )}
                {item.badge && (
                  <span
                    className={cn(
                      'inline-block align-middle mr-2 px-2 py-0.5 rounded-full',
                      'border border-mandat-muted text-mandat-muted',
                      "text-[10px] font-display uppercase tracking-[1px]"
                    )}
                  >
                    {item.badge}
                  </span>
                )}
                {item.title}
              </div>
            </a>
          ))}
        </div>
      </div>
    </section>
  )
}
