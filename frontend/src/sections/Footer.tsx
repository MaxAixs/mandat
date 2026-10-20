import { FOOTER_LINKS, SITE_NAME, SITE_TAGLINE } from '@/constants'

export default function Footer() {
  return (
    <footer className="border-t border-mandat-line py-14 pb-8 bg-mandat-surface">
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-[1.4fr_1fr_1fr_1fr] gap-10">
          <div>
            <a href="#" className="flex items-center gap-3 no-underline text-mandat-text">
              <span className="inline-block w-[12px] h-[34px] bg-mandat-red-bright" />
              <span className="font-['Oswald'] text-[22px] font-bold tracking-[2px]">{SITE_NAME}</span>
            </a>
            <p className="text-mandat-muted text-[14px] mt-4 max-w-[280px]">
              Про владу, рішення, гроші та вплив. {SITE_TAGLINE}.
            </p>
          </div>
          {FOOTER_LINKS.map((column) => (
            <div key={column.title}>
              <h4 className="font-['Oswald'] text-[14px] tracking-[2px] uppercase mb-4">{column.title}</h4>
              <ul className="grid gap-2.5 list-none p-0">
                {column.links.map((link) => (
                  <li key={link.label}>
                    <a
                      href={link.href}
                      className="text-mandat-muted text-[14px] hover:text-mandat-red-bright transition-colors no-underline"
                    >
                      {link.label}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <div className="mt-12 pt-6 border-t border-mandat-line flex flex-wrap justify-between text-mandat-muted text-[13px] gap-3">
          <span>© 2026 {SITE_NAME}. Усі права захищено.</span>
          <span>Чіткість та структура · Мінімалізм · Контраст та ієрархія</span>
        </div>
      </div>
    </footer>
  )
}
