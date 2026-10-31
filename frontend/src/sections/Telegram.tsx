import { SITE_TAGLINE, TELEGRAM_CHANNEL } from '@/constants'
import logoM from '@/assets/logo-m.svg'

export default function Telegram() {
  return (
    <section className="border-y border-mandat-line bg-mandat-surface">
      <div className="mx-auto max-w-[1240px] px-6 py-10 flex flex-wrap items-center justify-between gap-8">
        <div className="flex items-center gap-6">
          <img
            src={logoM}
            alt="Логотип МАНДАТ"
            className="w-[72px] h-[72px] rounded-full border border-mandat-line flex-none"
          />
          <div>
            <h2 className="text-[26px] sm:text-[32px] tracking-[2px] leading-none">{TELEGRAM_CHANNEL.name}</h2>
            <p className="text-mandat-muted text-[14px] mt-2">
              {TELEGRAM_CHANNEL.subscribers} підписників · {SITE_TAGLINE}
            </p>
          </div>
        </div>
        <a
          href={TELEGRAM_CHANNEL.url}
          target="_blank"
          rel="noopener noreferrer"
          className="font-display text-[15px] tracking-[2px] uppercase px-10 py-3.5 bg-mandat-red-bright text-mandat-text transition-colors hover:bg-mandat-red no-underline"
        >
          Підписатися
        </a>
      </div>
    </section>
  )
}
