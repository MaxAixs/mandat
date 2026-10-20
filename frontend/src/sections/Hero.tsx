export default function Hero() {
  return (
    <section className="py-[72px] pb-14 border-b border-mandat-line">
      <div className="mx-auto max-w-[1240px] px-6">
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
          <div className="lg:border-l-[2px] lg:border-mandat-red-bright lg:pl-7">
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
