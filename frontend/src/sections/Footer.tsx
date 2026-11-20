import MandatLogo from '@/components/MandatLogo'
import SocialIcon from '@/components/SocialIcon'
import { FOOTER_COPYRIGHT, FOOTER_LINKS, SOCIAL_LINKS } from '@/constants'
import { cn } from '@/lib/utils'

export default function Footer() {
  return (
    <footer className="border-t border-mandat-line bg-mandat-surface pt-14 pb-12">
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="flex flex-col items-center text-center">
          <a href="/" className="no-underline">
            <MandatLogo className="h-14 sm:h-[72px]" />
          </a>

          <p className="mt-7 text-[13px] font-medium uppercase tracking-[0.04em] text-white/85">
            {FOOTER_COPYRIGHT}
          </p>

          <div className="mt-7 flex flex-wrap items-center justify-center gap-3">
            {SOCIAL_LINKS.map((social) => (
              <a
                key={social.id}
                href={social.href}
                aria-label={social.label}
                target="_blank"
                rel="noopener noreferrer"
                className="grid size-12 place-items-center rounded-full border-2 border-white/80 text-white/90 no-underline transition-colors hover:border-mandat-red-bright hover:text-mandat-red-bright"
              >
                <SocialIcon id={social.id} className="size-[22px]" />
              </a>
            ))}
          </div>
        </div>

        <div className="mt-4 grid grid-cols-2 gap-8 border-t border-mandat-line pt-4 sm:grid-cols-3 sm:gap-x-[1.5cm] sm:gap-y-10">
          {FOOTER_LINKS.map((column) => {
            const wide = column.title === 'Розділи'
            return (
              <div key={column.title} className={cn(wide && 'sm:col-span-2')}>
                <h4 className="font-display text-[14px] tracking-[2px] uppercase mb-2">{column.title}</h4>
                <ul className={cn('grid gap-2.5 list-none p-0', wide && 'sm:grid-flow-col sm:grid-rows-4 sm:justify-start sm:gap-x-4')}>
                  {column.links.map((link) => (
                    <li key={link.label}>
                      <a
                        href={link.href}
                        className="text-mandat-muted text-[14px] hover:text-mandat-red-bright transition-colors no-underline"
                      >
                        {link.shortLabel ?? link.label}
                      </a>
                    </li>
                  ))}
                </ul>
              </div>
            )
          })}
        </div>
      </div>
    </footer>
  )
}
