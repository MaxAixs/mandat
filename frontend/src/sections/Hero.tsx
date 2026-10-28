import heroBg from '@/assets/hero-bg.jpg'

export default function Hero() {
  return (
    <section className="relative flex min-h-[240px] items-center overflow-hidden isolate border-b border-mandat-line py-[72px] pb-14 sm:min-h-[360px] lg:min-h-[480px]">
      {/* Background photo with a dark gradient so the text stays readable */}
      <img
        src={heroBg}
        alt=""
        aria-hidden="true"
        className="absolute inset-0 w-full h-full object-cover object-center -z-10"
      />
      <div className="absolute inset-0 -z-10 bg-gradient-to-r from-mandat-bg via-mandat-bg/80 to-mandat-bg/40" />
      <div className="absolute inset-0 -z-10 bg-gradient-to-t from-mandat-bg via-transparent to-mandat-bg/70" />

      <div className="w-full px-8">
        <div className="grid grid-cols-1 lg:grid-cols-[1.1fr_0.9fr] gap-12 items-end">
          <div>
            <h1 className="text-[clamp(44px,7vw,96px)] font-bold leading-[0.95] tracking-[2px]">
              Про владу, рішення,
              <br />
              гроші та <span className="text-mandat-red-bright">вплив.</span>
            </h1>
            <p className="max-w-[520px] text-mandat-muted text-[17px] mt-7">
              Факти без емоцій. Аналітика без шуму. Новини та інсайди, яким довіряють — у зручному форматі сайту.
            </p>
          </div>
          <div className="lg:border-l-[2px] lg:border-mandat-red-bright lg:pl-7 lg:justify-self-end">
            <div className="font-['Oswald'] text-[72px] font-bold leading-none">42 728</div>
            <div className="text-mandat-muted uppercase tracking-[2px] text-[13px] mt-2">
              підписників уже з нами
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
