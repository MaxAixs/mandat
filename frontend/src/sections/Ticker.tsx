export interface TickerRate {
  code: string
  name: string
  flag: string
  value: string
  unit?: string
}

export default function Ticker({ items }: { items: TickerRate[] }) {
  if (items.length === 0) return null

  // Repeat the list so the 50% translate loop looks seamless.
  const repeated = [...items, ...items, ...items, ...items]

  return (
    <div className="overflow-hidden border-y border-mandat-line bg-mandat-surface mt-16">
      <div className="flex items-stretch whitespace-nowrap animate-[scroll_90s_linear_infinite] w-max">
        {repeated.map((rate, i) => (
          <div
            key={`${rate.code}-${i}`}
            aria-label={`${rate.name} (${rate.code}): ${rate.value} гривень`}
            className="flex items-center gap-3 px-8 py-2.5"
          >
            <span aria-hidden="true" className="text-xl leading-none">
              {rate.flag}
            </span>
            <span className="flex flex-col">
              <span className="font-display text-[12px] leading-tight text-mandat-text">
                {rate.name}
              </span>
              <span className="font-display text-[10px] tracking-[1.5px] leading-tight text-mandat-muted">
                {rate.code}
              </span>
            </span>
            <span className="font-display text-lg font-semibold tabular-nums text-mandat-text">
              {rate.value}
            </span>
            <span className="self-start pt-0.5 font-display text-[11px] text-mandat-muted">{rate.unit ?? '₴'}</span>
            <span aria-hidden="true" className="ml-5 h-5 w-px bg-mandat-line" />
          </div>
        ))}
      </div>
    </div>
  )
}
