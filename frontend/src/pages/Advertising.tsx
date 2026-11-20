import { useEffect } from 'react'

import AdvertisingFormatCard from '@/components/AdvertisingFormatCard'
import HeadingMarker from '@/components/HeadingMarker'
import MandatLogo from '@/components/MandatLogo'
import SocialIcon from '@/components/SocialIcon'
import { SOCIAL_LINKS } from '@/constants'
import {
  ADVERTISING_AUDIENCE,
  ADVERTISING_AUDIENCE_TITLE,
  ADVERTISING_CTA_TITLE,
  ADVERTISING_EMAIL,
  ADVERTISING_FORMATS,
  ADVERTISING_FORMATS_LEAD,
  ADVERTISING_FORMATS_TITLE,
  ADVERTISING_LEAD,
  ADVERTISING_ORDER_LABEL,
  ADVERTISING_ORDER_SUBJECT,
  ADVERTISING_TITLE,
} from '@/constants/advertising'
import { cn } from '@/lib/utils'
import Footer from '@/sections/Footer'
import Header from '@/sections/Header'

// Logos per marquee half; the half must stay wider than a 2560px screen.
const MARQUEE_ITEMS = 12
const MARQUEE_SECONDS = 60

const orderHref = `mailto:${ADVERTISING_EMAIL}?subject=${encodeURIComponent(ADVERTISING_ORDER_SUBJECT)}`
const orderButtonClassName =
  'inline-flex h-12 items-center justify-center px-7 text-[15px] font-medium no-underline transition-colors'

export default function Advertising() {
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'instant' })
    document.title = ADVERTISING_TITLE
  }, [])

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />

      <section className="mx-auto max-w-[1240px] px-6 pb-10 pt-12 md:pb-14 md:pt-16">
        <h1 className="max-w-[820px] text-[34px] font-bold leading-[1.1] text-white md:text-[54px]">
          {ADVERTISING_TITLE}
        </h1>
        <p className="mt-6 max-w-[680px] text-[16px] leading-relaxed text-mandat-muted md:text-[18px]">
          {ADVERTISING_LEAD}
        </p>
        <a href={orderHref} className={cn(orderButtonClassName, 'mt-8 bg-mandat-red-bright text-white hover:bg-mandat-red')}>
          {ADVERTISING_ORDER_LABEL}
        </a>
      </section>

      <div className="overflow-hidden bg-mandat-red py-6 md:py-8" aria-hidden="true">
        <div
          className="flex w-max items-center motion-reduce:!animate-none"
          style={{ animation: `scroll ${MARQUEE_SECONDS}s linear infinite` }}
        >
          {Array.from({ length: MARQUEE_ITEMS * 2 }, (_, i) => (
            <MandatLogo key={i} className="mx-8 h-6 shrink-0 md:mx-12 md:h-8" barClassName="fill-white" />
          ))}
        </div>
      </div>

      <main className="mx-auto max-w-[1240px] px-6 pb-20 pt-12 md:pt-16">
        <h2 className="text-[26px] font-bold leading-tight tracking-[0.3px] text-white md:text-[32px]">
          <HeadingMarker />
          {ADVERTISING_AUDIENCE_TITLE}
        </h2>
        <div className="mt-8 grid grid-cols-2 gap-3 lg:grid-cols-4">
          {SOCIAL_LINKS.map((social) => (
            <a
              key={social.id}
              href={social.href}
              target="_blank"
              rel="noopener noreferrer"
              className="group flex flex-col justify-between gap-6 border border-mandat-line bg-mandat-surface p-4 no-underline transition-colors hover:border-mandat-red-bright sm:flex-row sm:items-end sm:p-5"
            >
              <span>
                <span className="block text-[22px] font-bold leading-none tabular-nums text-white sm:text-[26px]">
                  {ADVERTISING_AUDIENCE[social.id] ?? '—'}
                </span>
                <span className="mt-3 block text-[14px] text-mandat-muted">{social.label}</span>
              </span>
              <span className="grid size-10 shrink-0 place-items-center rounded-full border border-mandat-line text-white/80 transition-colors group-hover:border-mandat-red-bright group-hover:text-mandat-red-bright">
                <SocialIcon id={social.id} className="size-[18px]" />
              </span>
            </a>
          ))}
        </div>

        <h2 className="mt-16 text-[26px] font-bold leading-tight tracking-[0.3px] text-white md:mt-20 md:text-[32px]">
          <HeadingMarker />
          {ADVERTISING_FORMATS_TITLE}
        </h2>
        <p className="mt-4 max-w-[760px] text-[16px] leading-relaxed text-mandat-muted md:text-[17px]">
          {ADVERTISING_FORMATS_LEAD}
        </p>
        <div className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {ADVERTISING_FORMATS.map((format) => (
            <AdvertisingFormatCard
              key={format.id}
              format={format}
              href={`mailto:${ADVERTISING_EMAIL}?subject=${encodeURIComponent(`${ADVERTISING_ORDER_SUBJECT}: ${format.title}`)}`}
              ariaLabel={`${ADVERTISING_ORDER_LABEL}: ${format.title}`}
            />
          ))}
        </div>
      </main>

      <section className="bg-mandat-red">
        <div className="mx-auto flex max-w-[1240px] flex-col gap-8 px-6 py-12 md:flex-row md:items-end md:justify-between md:py-16">
          <MandatLogo className="h-8 self-start md:h-10 md:self-auto" barClassName="fill-white" />
          <div className="md:text-right">
            <p className="text-[28px] font-bold leading-tight text-white md:text-[36px]">{ADVERTISING_CTA_TITLE}</p>
            <a href={orderHref} className={cn(orderButtonClassName, 'mt-6 bg-white text-mandat-bg hover:bg-white/90')}>
              {ADVERTISING_ORDER_LABEL}
            </a>
          </div>
        </div>
      </section>

      <Footer />
    </div>
  )
}
