import { AtSign, Instagram, Send } from 'lucide-react'

import logoUrl from '@/assets/logo-mandat.png'
import { FOOTER_LINKS, SITE_NAME, SITE_TAGLINE, SOCIAL_LINKS } from '@/constants'

const SOCIAL_ICONS: Record<string, typeof Instagram> = {
  instagram: Instagram,
  threads: AtSign,
  telegram: Send,
}

export default function Footer() {
  return (
    <footer className="border-t border-mandat-line py-14 pb-8 bg-mandat-surface">
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="flex flex-col items-center text-center">
          <img src={logoUrl} alt={SITE_NAME} className="h-14 w-auto mix-blend-screen" />
          <p className="text-mandat-muted text-[14px] mt-4">
            Про владу, рішення, гроші та вплив. {SITE_TAGLINE}.
          </p>
          <div className="mt-6 flex items-center gap-4">
            {SOCIAL_LINKS.map((social) => {
              const Icon = SOCIAL_ICONS[social.id]
              return (
                <a
                  key={social.id}
                  href={social.href}
                  aria-label={social.label}
                  className="grid size-11 place-items-center rounded-full border border-mandat-line text-mandat-muted transition-colors hover:border-mandat-red-bright hover:text-mandat-red-bright no-underline"
                >
                  <Icon className="size-5" aria-hidden="true" />
                </a>
              )
            })}
          </div>
          <p className="mt-6 text-mandat-muted text-[13px]">
            © 2026 {SITE_NAME}. Усі права захищено.
          </p>
        </div>
        <div className="mt-8 pt-6 border-t border-mandat-line grid grid-cols-1 sm:grid-cols-2 gap-10">
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
      </div>
    </footer>
  )
}
