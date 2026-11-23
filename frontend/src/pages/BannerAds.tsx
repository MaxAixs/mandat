import AdFormatPage from '@/components/AdFormatPage'
import AdFormatSectionTitle from '@/components/AdFormatSectionTitle'
import {
  BANNER_ADS_CTA_TITLE,
  BANNER_ADS_EXAMPLE_LABEL,
  BANNER_ADS_LEAD,
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

export default function BannerAds() {
  return (
    <AdFormatPage
      title={BANNER_ADS_TITLE}
      lead={BANNER_ADS_LEAD}
      principle={BANNER_ADS_PRINCIPLE}
      ctaTitle={BANNER_ADS_CTA_TITLE}
    >
      <section className="mt-16 md:mt-20">
        <AdFormatSectionTitle>{BANNER_ADS_SETTINGS_TITLE}</AdFormatSectionTitle>
        <div className="mt-8 grid gap-3 md:grid-cols-2">
          {BANNER_ADS_SETTINGS.map((setting) => (
            <div key={setting.title} className="flex flex-col border border-mandat-line bg-mandat-surface p-5 md:p-6">
              <h3 className="text-[20px] font-semibold text-white">{setting.title}</h3>
              <p className="mt-3 text-[15px] leading-relaxed text-mandat-muted">{setting.text}</p>
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
        <AdFormatSectionTitle>{BANNER_ADS_PLACEMENTS_TITLE}</AdFormatSectionTitle>
        <div className="mt-8 grid gap-16 md:gap-20">
          {BANNER_ADS_PLACEMENTS.map((placement) => (
            <div key={placement.title}>
              <h3 className="text-[22px] font-semibold text-white md:text-[24px]">{placement.title}</h3>
              <p className="mt-2 max-w-[680px] text-[16px] leading-relaxed text-mandat-muted">{placement.text}</p>
              <div className="mt-8 grid gap-12 md:gap-14">
                {placement.shots.map((shot) => (
                  <figure
                    key={shot.caption}
                    className={cn('m-0', shot.mobile && 'md:flex md:items-center md:gap-12')}
                  >
                    <img
                      src={shot.image}
                      alt={shot.alt}
                      loading="lazy"
                      className={cn(
                        'block h-auto border border-mandat-line',
                        shot.mobile ? 'w-full max-w-[340px] shrink-0 border-0' : 'w-full',
                      )}
                    />
                    <figcaption className={cn('mt-4', shot.mobile && 'md:mt-0')}>
                      <span className="block text-[15px] text-mandat-muted">{shot.caption}</span>
                      <ul className="m-0 mt-3 grid list-none gap-2 p-0">
                        {shot.slots.map((slot) => (
                          <li key={slot.size} className="text-[16px] leading-relaxed">
                            <strong className="font-semibold text-white">{slot.size}</strong> — {slot.text}
                          </li>
                        ))}
                      </ul>
                    </figcaption>
                  </figure>
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="mt-16 md:mt-20">
        <AdFormatSectionTitle>{BANNER_ADS_SPECS_TITLE}</AdFormatSectionTitle>
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
    </AdFormatPage>
  )
}
