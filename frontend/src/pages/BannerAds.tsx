import { useEffect } from 'react'
import { Link } from 'react-router'
import { ArrowLeft } from 'lucide-react'

import HeadingMarker from '@/components/HeadingMarker'
import MandatLogo from '@/components/MandatLogo'
import { ADVERTISING_EMAIL, ADVERTISING_ORDER_LABEL } from '@/constants/advertising'
import {
  BANNER_ADS_BACK_LABEL,
  BANNER_ADS_CTA_TITLE,
  BANNER_ADS_EXAMPLE_LABEL,
  BANNER_ADS_LEAD,
  BANNER_ADS_ORDER_SUBJECT,
  BANNER_ADS_PLACEMENTS,
  BANNER_ADS_PLACEMENTS_TITLE,
  BANNER_ADS_PRINCIPLE,
  BANNER_ADS_SETTINGS,
  BANNER_ADS_SETTINGS_TITLE,
  BANNER_ADS_SPECS,
  BANNER_ADS_SPECS_TITLE,
  BANNER_ADS_TITLE,
} from '@/constants/banner-ads'
import { cn } from '@/lib/utils'
import Footer from '@/sections/Footer'
import Header from '@/sections/Header'

const orderHref = `mailto:${ADVERTISING_EMAIL}?subject=${encodeURIComponent(BANNER_ADS_ORDER_SUBJECT)}`

function SectionTitle({ children }: { children: string }) {
  return (
    <h2 className="text-[26px] font-bold leading-tight tracking-[0.3px] text-white md:text-[32px]">
      <HeadingMarker />
      {children}
    </h2>
  )
}

export default function BannerAds() {
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'instant' })
    document.title = BANNER_ADS_TITLE
  }, [])

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />

      <section className="mx-auto max-w-[1080px] px-6 pb-10 pt-10 md:pb-14 md:pt-14">
        <Link
          to="/advertising"
          className="inline-flex items-center gap-2 text-[15px] text-mandat-muted no-underline transition-colors hover:text-white"
        >
          <ArrowLeft className="size-4" strokeWidth={1.75} aria-hidden="true" />
          {BANNER_ADS_BACK_LABEL}
        </Link>
        <h1 className="mt-6 text-[34px] font-bold leading-[1.1] text-white md:text-[54px]">{BANNER_ADS_TITLE}</h1>
        <p className="mt-6 max-w-[680px] text-[16px] leading-relaxed text-mandat-muted md:text-[18px]">
          {BANNER_ADS_LEAD}
        </p>
      </section>

      <main className="mx-auto max-w-[1080px] px-6 pb-20">
        <p className="m-0 max-w-[860px] border-l-4 border-mandat-red-bright bg-mandat-surface py-6 pl-6 pr-6 text-[20px] font-medium leading-snug text-white md:py-8 md:pl-8 md:text-[26px]">
          {BANNER_ADS_PRINCIPLE}
        </p>

        <section className="mt-16 md:mt-20">
          <SectionTitle>{BANNER_ADS_SETTINGS_TITLE}</SectionTitle>
          <div className="mt-8 grid gap-3 md:grid-cols-3">
            {BANNER_ADS_SETTINGS.map((setting) => (
              <div key={setting.title} className="flex flex-col border border-mandat-line bg-mandat-surface p-5 md:p-6">
                <h3 className="text-[20px] font-semibold text-white">{setting.title}</h3>
                <p className="mt-3 text-[15px] leading-relaxed text-mandat-muted">{setting.text}</p>
                {setting.options && (
                  <ul className="m-0 mt-auto flex list-none flex-wrap gap-2 p-0 pt-5">
                    {setting.options.map((option) => (
                      <li key={option} className="border border-mandat-line px-3 py-1.5 text-[14px] text-white">
                        {option}
                      </li>
                    ))}
                  </ul>
                )}
                {setting.example && (
                  <p className="mb-0 mt-auto pt-5 text-[15px] text-white">
                    <span className="text-mandat-muted">{BANNER_ADS_EXAMPLE_LABEL} </span>
                    {setting.example}
                  </p>
                )}
              </div>
            ))}
          </div>
        </section>

        <section className="mt-16 md:mt-20">
          <SectionTitle>{BANNER_ADS_PLACEMENTS_TITLE}</SectionTitle>
          <div className="mt-8 grid gap-14 md:gap-16">
            {BANNER_ADS_PLACEMENTS.map((placement) => (
              <figure
                key={placement.caption}
                className={cn('m-0', placement.mobile && 'md:flex md:items-center md:gap-12')}
              >
                <img
                  src={placement.image}
                  alt={placement.alt}
                  loading="lazy"
                  className={cn(
                    'block h-auto border border-mandat-line',
                    placement.mobile ? 'w-full max-w-[340px] shrink-0 border-0' : 'w-full',
                  )}
                />
                <figcaption className={cn('mt-4', placement.mobile && 'md:mt-0')}>
                  <span className="block text-[15px] text-mandat-muted">{placement.caption}</span>
                  <ul className="m-0 mt-3 grid list-none gap-2 p-0">
                    {placement.slots.map((slot) => (
                      <li key={slot.size} className="text-[16px] leading-relaxed">
                        <strong className="font-semibold text-white">{slot.size}</strong> — {slot.text}
                      </li>
                    ))}
                  </ul>
                </figcaption>
              </figure>
            ))}
          </div>
        </section>

        <section className="mt-16 md:mt-20">
          <SectionTitle>{BANNER_ADS_SPECS_TITLE}</SectionTitle>
          <dl className="m-0 mt-8 max-w-[860px] border-t border-mandat-line">
            {BANNER_ADS_SPECS.map((spec) => (
              <div
                key={spec.label}
                className="grid gap-1 border-b border-mandat-line py-5 sm:grid-cols-[220px_1fr] sm:gap-6"
              >
                <dt className="text-[15px] text-mandat-muted">{spec.label}</dt>
                <dd className="m-0">
                  <span className="block text-[17px] font-medium tabular-nums text-white">{spec.value}</span>
                  {spec.note && (
                    <span className="mt-1.5 block max-w-[56ch] text-[15px] leading-relaxed text-mandat-muted">
                      {spec.note}
                    </span>
                  )}
                </dd>
              </div>
            ))}
          </dl>
        </section>
      </main>

      <section className="bg-mandat-red">
        <div className="mx-auto flex max-w-[1080px] flex-col gap-8 px-6 py-12 md:flex-row md:items-end md:justify-between md:py-16">
          <MandatLogo className="h-8 self-start md:h-10 md:self-auto" barClassName="fill-white" />
          <div className="md:text-right">
            <p className="text-[28px] font-bold leading-tight text-white md:text-[36px]">{BANNER_ADS_CTA_TITLE}</p>
            <a
              href={orderHref}
              className="mt-6 inline-flex h-12 items-center justify-center bg-white px-7 text-[15px] font-medium text-mandat-bg no-underline transition-colors hover:bg-white/90"
            >
              {ADVERTISING_ORDER_LABEL}
            </a>
          </div>
        </div>
      </section>

      <Footer />
    </div>
  )
}
