import { SITE_TAGLINE, TELEGRAM_CHANNEL } from '@/constants'

export default function Telegram() {
  return (
    <section className="border-y border-mandat-line bg-mandat-surface">
      <div className="mx-auto max-w-[1240px] px-6 py-5 flex flex-wrap items-center justify-between gap-6">
        <div className="flex items-center gap-5">
          <span className="w-[52px] h-[52px] rounded-full bg-mandat-bg border border-mandat-line grid place-items-center font-['Oswald'] text-[22px] font-bold flex-none">
            М<span className="text-mandat-red-bright">.</span>
          </span>
          <div>
            <h2 className="text-[22px] tracking-[2px] leading-none">{TELEGRAM_CHANNEL.name}</h2>
            <p className="text-mandat-muted text-[13px] mt-1.5">
              {TELEGRAM_CHANNEL.subscribers} підписників · {SITE_TAGLINE}
            </p>
          </div>
        </div>
        <a
          href={TELEGRAM_CHANNEL.url}
          target="_blank"
          rel="noopener noreferrer"
          className="font-['Oswald'] text-[14px] tracking-[2px] uppercase px-8 py-2.5 bg-mandat-red-bright text-mandat-text transition-colors hover:bg-mandat-red no-underline"
        >
          Підписатися
        </a>
      </div>
    </section>
  )
}
