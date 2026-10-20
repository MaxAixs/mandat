import { NAV_LINKS, SITE_NAME, SITE_TAGLINE } from '@/constants'

export default function Header() {
  return (
    <header
      className="sticky top-0 z-50 border-b border-mandat-line"
      style={{ background: 'rgba(23,25,28,0.92)', backdropFilter: 'blur(12px)' }}
    >
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="flex items-center justify-between h-[72px]">
          <a href="#" className="flex items-center gap-3 no-underline text-mandat-text">
            <span className="inline-block w-[12px] h-[34px] bg-mandat-red-bright" />
            <span>
              <span className="font-['Oswald'] text-[26px] font-bold tracking-[2px] leading-none block">
                {SITE_NAME}
              </span>
              <span className="text-[10px] tracking-[3px] text-mandat-muted uppercase block mt-0.5">
                {SITE_TAGLINE}
              </span>
            </span>
          </a>

          <nav className="hidden md:flex gap-9">
            {NAV_LINKS.map((link) => (
              <a
                key={link.label}
                href={link.href}
                className="relative font-['Oswald'] text-[15px] tracking-[2px] uppercase py-1.5 group text-mandat-text"
              >
                {link.label}
                <span className="absolute left-0 bottom-0 h-[2px] w-0 bg-mandat-red-bright transition-all duration-250 group-hover:w-full" />
              </a>
            ))}
          </nav>
        </div>
      </div>
    </header>
  )
}
