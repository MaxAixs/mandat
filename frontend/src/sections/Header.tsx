import { useEffect, useRef, useState } from 'react'
import { Search, X } from 'lucide-react'
import { Link, NavLink, useLocation } from 'react-router'

import SearchBox from '@/components/SearchBox'
import MandatLogo from '@/components/MandatLogo'
import { cn } from '@/lib/utils'
import { NAV_LINKS } from '@/constants'

export default function Header() {
  const [searchOpen, setSearchOpen] = useState(false)
  const { pathname } = useLocation()
  const mobileNavRef = useRef<HTMLElement>(null)

  // Keep the active rubric in view in the horizontally scrolling mobile nav.
  useEffect(() => {
    const nav = mobileNavRef.current
    const active = nav?.querySelector<HTMLElement>('[aria-current="page"]')
    if (!nav || !active) return
    nav.scrollLeft = active.offsetLeft - nav.offsetLeft - (nav.clientWidth - active.offsetWidth) / 2
  }, [pathname])

  return (
    // Mobile: solid background on its own compositing layer. iOS WebKit repaints a
    // translucent sticky header with backdrop-filter late during momentum scroll,
    // so it visibly lags and page text shows through. The blur stays on desktop.
    // The ::before block extends the background upwards: iOS draws the page under
    // its floating status/address bar, above where the sticky header pins, and
    // scrolling text would otherwise be visible in that gap.
    <header className="sticky top-0 z-50 transform-gpu border-b border-mandat-line bg-mandat-bg before:pointer-events-none before:absolute before:inset-x-0 before:bottom-full before:h-[100vh] before:bg-mandat-bg before:content-[''] lg:transform-none lg:bg-[color-mix(in_srgb,var(--mandat-bg)_92%,transparent)] lg:backdrop-blur-[12px] lg:before:hidden">
      <div className="w-full px-4 sm:px-6 lg:px-8">
        <div className="grid grid-cols-1 lg:grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-4 lg:gap-x-6 h-[60px] lg:h-[95px]">
          <Link to="/" className="flex min-w-0 items-center gap-4 no-underline text-mandat-text justify-self-center lg:justify-self-start">
            <MandatLogo className="h-7 shrink-0 lg:h-9" />
          </Link>

          <nav className="hidden lg:flex min-w-0 gap-2 xl:gap-4 min-[1440px]:gap-5 min-[1680px]:gap-7 justify-self-center">
            {NAV_LINKS.map((link) => (
              <Link
                key={link.label}
                to={link.href}
                className="relative font-header text-[14px] min-[1440px]:text-[16px] min-[1680px]:text-[18px] font-medium tracking-[0.5px] min-[1680px]:tracking-[1px] uppercase whitespace-nowrap py-2.5 group text-mandat-text"
              >
                {link.label}
                <span className="absolute left-0 bottom-0 h-[2px] w-0 bg-mandat-red-bright transition-all duration-250 group-hover:w-full" />
              </Link>
            ))}
          </nav>
          <div className="hidden xl:flex min-w-0 justify-self-end items-center gap-2.5">
            <div className="w-[220px]">
              <SearchBox />
            </div>
          </div>
          <div className="hidden lg:block xl:hidden justify-self-end">
            <button
              type="button"
              onClick={() => setSearchOpen((prev) => !prev)}
              aria-label="Пошук новин"
              aria-expanded={searchOpen}
              className="w-10 h-10 grid place-items-center border border-mandat-line text-mandat-muted transition-colors hover:border-mandat-red-bright hover:text-mandat-text bg-transparent cursor-pointer"
            >
              {searchOpen ? <X size={18} /> : <Search size={18} />}
            </button>
          </div>
        </div>
        <nav
          ref={mobileNavRef}
          aria-label="Рубрики"
          className="lg:hidden -mx-4 flex gap-5 overflow-x-auto overscroll-x-contain border-t border-mandat-line px-4 sm:-mx-6 sm:gap-6 sm:px-6 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          {NAV_LINKS.map((link) => (
            <NavLink
              key={link.label}
              to={link.href}
              end
              className={({ isActive }) =>
                cn(
                  'relative shrink-0 whitespace-nowrap py-3 font-header text-[13px] font-medium uppercase tracking-[0.5px] no-underline transition-colors',
                  isActive
                    ? 'text-white after:absolute after:inset-x-0 after:bottom-0 after:h-[2px] after:bg-mandat-red-bright'
                    : 'text-mandat-muted hover:text-mandat-text',
                )
              }
            >
              {link.shortLabel ?? link.label}
            </NavLink>
          ))}
        </nav>
        {searchOpen && (
          <div className="hidden lg:block xl:hidden py-3">
            <SearchBox autoFocus onNavigate={() => setSearchOpen(false)} />
          </div>
        )}
      </div>
    </header>
  )
}
