import { Search } from 'lucide-react'
import { Link } from 'react-router'

import { NAV_LINKS, SITE_NAME } from '@/constants'

export default function Header() {
  return (
    <header
      className="sticky top-0 z-50 border-b border-mandat-line"
      style={{ background: 'rgba(23,25,28,0.92)', backdropFilter: 'blur(12px)' }}
    >
      <div className="w-full px-8">
        <div className="grid grid-cols-[auto_1fr_auto] lg:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-x-4 lg:gap-x-6 h-[72px]">
          <Link to="/" className="flex min-w-0 items-center gap-4 no-underline text-mandat-text justify-self-start">
            <span className="inline-block w-[16px] h-[44px] bg-mandat-red-bright" />
            <span className="font-display text-[21px] lg:text-[24px] font-bold tracking-[1px] lg:tracking-[1.5px] leading-none">
              {SITE_NAME}
            </span>
          </Link>

          <nav className="hidden md:flex min-w-0 gap-3 lg:gap-6 justify-self-center">
            {NAV_LINKS.map((link) => (
              <a
                key={link.label}
                href={link.href}
                className="relative font-display text-[13px] lg:text-[16px] font-medium tracking-[1px] uppercase whitespace-nowrap py-2.5 group text-mandat-text"
              >
                {link.label}
                <span className="absolute left-0 bottom-0 h-[2px] w-0 bg-mandat-red-bright transition-all duration-250 group-hover:w-full" />
              </a>
            ))}
          </nav>
          <div className="hidden md:flex min-w-0 justify-self-end items-center gap-2.5">
            <input
              type="search"
              placeholder="Пошук новин…"
              className="hidden xl:block w-[220px] h-10 px-4 text-[13px] text-mandat-text bg-mandat-bg outline-none border border-mandat-line focus:border-mandat-red-bright transition-colors"
            />
            <a
              href="#news"
              aria-label="Пошук новин"
              title="Пошук новин"
              className="w-10 h-10 grid place-items-center border border-mandat-line text-mandat-muted transition-colors hover:border-mandat-red-bright hover:text-mandat-text bg-transparent cursor-pointer"
            >
              <Search size={18} />
            </a>
          </div>
        </div>
      </div>
    </header>
  )
}
