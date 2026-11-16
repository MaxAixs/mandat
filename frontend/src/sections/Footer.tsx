import MandatLogo from '@/components/MandatLogo'
import SocialIcon from '@/components/SocialIcon'
import { FOOTER_COPYRIGHT, FOOTER_LINKS, FOOTER_USAGE_NOTICE, SOCIAL_LINKS } from '@/constants'

export default function Footer() {
  return (
    <footer className="border-t border-mandat-line bg-mandat-surface pt-14 pb-12">
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="grid grid-cols-1 gap-10 sm:grid-cols-2">
          {FOOTER_LINKS.map((column) => (
            <div key={column.title}>
              <h4 className="font-display text-[14px] tracking-[2px] uppercase mb-4">{column.title}</h4>
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

        <div className="mt-12 flex flex-col items-center border-t border-mandat-line pt-12 text-center">
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

          <p className="mt-8 max-w-[640px] text-[13px] leading-relaxed text-mandat-muted">
            {FOOTER_USAGE_NOTICE}
          </p>
        </div>
      </div>
    </footer>
  )
}
