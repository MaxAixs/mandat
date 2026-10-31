import { Link } from 'react-router'

import heroBg from '@/assets/hero-bg.jpg'
import type { Article } from '@/types'

const headingClassName = 'text-[clamp(34px,4.8vw,68px)] font-bold leading-[1.05] tracking-[0.5px]'
const mainEventHeadingClassName = 'text-[clamp(28px,3.8vw,52px)] font-bold leading-[1.05] tracking-[0.5px]'
const fallbackDescription = 'Факти без емоцій. Аналітика без шуму. Новини та інсайди, яким довіряють — у зручному форматі сайту.'

interface HeroProps {
  mainEvent?: Article | null
}

export default function Hero({ mainEvent }: HeroProps) {
  const backgroundImage = mainEvent?.imageUrl || heroBg

  return (
    <section className="relative flex min-h-[240px] items-center overflow-hidden isolate border-b border-mandat-line py-[72px] pb-14 sm:min-h-[360px] lg:min-h-[480px]">
      {/* Background photo with a dark gradient so the text stays readable */}
      <img
        src={backgroundImage}
        alt=""
        aria-hidden="true"
        className="absolute inset-0 w-full h-full object-cover object-center -z-10"
      />
      <div className="absolute inset-0 -z-10 bg-gradient-to-r from-mandat-bg via-mandat-bg/80 to-mandat-bg/40" />
      <div className="absolute inset-0 -z-10 bg-gradient-to-t from-mandat-bg via-transparent to-mandat-bg/70" />

      <div className="w-full px-8">
        <div className="grid grid-cols-1 lg:grid-cols-[1.1fr_0.9fr] gap-12 items-end">
          <div>
            {mainEvent ? (
              <Link
                to={`/news/${encodeURIComponent(mainEvent.id)}`}
                className="inline-block text-inherit no-underline transition-colors hover:text-mandat-red-bright focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-mandat-red-bright"
              >
                <h1 className={mainEventHeadingClassName}>{mainEvent.title}</h1>
              </Link>
            ) : (
              <h1 className={headingClassName}>
                Про владу, рішення,
                <br />
                гроші та <span className="text-mandat-red-bright">вплив.</span>
              </h1>
            )}
            <p className="max-w-[520px] text-mandat-muted text-[17px] mt-7">
              {mainEvent?.excerpt ?? fallbackDescription}
            </p>
          </div>
          <div className="lg:absolute lg:bottom-8 lg:right-8 lg:border-l-[2px] lg:border-mandat-red-bright lg:pl-7 lg:justify-self-end">
            <div className="font-display text-[56px] font-bold leading-none">42 728</div>
            <div className="text-mandat-muted uppercase tracking-[2px] text-[13px] mt-2">
              підписників уже з нами
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
