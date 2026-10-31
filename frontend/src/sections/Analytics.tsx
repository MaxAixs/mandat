import { mockStats } from '@/api/mock'

export default function Analytics() {
  return (
    <section id="analytics" className="border-y border-mandat-line bg-mandat-surface">
      <div className="mx-auto max-w-[1240px] px-6 py-[72px]">
        <div className="flex items-baseline justify-between mb-10">
          <h2 className="text-[20px] sm:text-[30px] tracking-[1px] font-bold">
            <span className="inline-block w-2 h-7 bg-mandat-red-bright mr-3.5 align-middle" />
            Цифра дня
          </h2>
          <a
            href="#"
            className="font-display shrink-0 whitespace-nowrap text-[11px] sm:text-[14px] tracking-[0.5px] sm:tracking-[2px] uppercase text-mandat-muted hover:text-mandat-red-bright transition-colors no-underline"
          >
            Вся аналітика →
          </a>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          {mockStats.map((stat) => (
            <div key={stat.caption} className="border-l-[2px] border-mandat-red-bright pl-4">
              <div className="font-display text-[38px] font-bold leading-none">
                {stat.value}
                {stat.unit && <small className="text-[20px] text-mandat-muted ml-1">{stat.unit}</small>}
              </div>
              <div className="text-mandat-muted text-[13px] mt-2 uppercase tracking-widest">{stat.caption}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
