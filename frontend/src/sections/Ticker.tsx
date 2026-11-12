import { Droplet, Flame, Fuel } from 'lucide-react'

import { cn } from '@/lib/utils'

export type TickerIcon = 'fuel' | 'droplet' | 'flame'

export interface TickerRate {
  code: string
  name: string
  flag?: string
  icon?: TickerIcon
  value: string
  unit?: string
  group?: string
}

// Seconds one card stays on screen; the loop shows two copies per pass.
const SECONDS_PER_CARD = 16

const tickerIcons = {
  fuel: Fuel,
  droplet: Droplet,
  flame: Flame,
} as const

export default function Ticker({
  items,
  ariaLabel,
  reverse = false,
  className,
}: {
  items: TickerRate[]
  ariaLabel?: string
  reverse?: boolean
  className?: string
}) {
  if (items.length === 0) return null

  // Repeat the list so the 50% translate loop looks seamless.
  const repeated = [...items, ...items, ...items, ...items]
  const duration = Math.max(20, items.length * SECONDS_PER_CARD)
  const animation = `scroll ${duration}s linear infinite${reverse ? ' reverse' : ''}`

  return (
    <div className={cn('overflow-hidden', className)}>
      {ariaLabel && <span className="sr-only">{ariaLabel}</span>}
      <div
        className="flex w-max items-stretch whitespace-nowrap"
        style={{ animation }}
      >
        {repeated.map((rate, i) => {
          const Icon = rate.icon ? tickerIcons[rate.icon] : null

          return (
            <div key={`${rate.code}-${i}`} className="flex items-center">
              {rate.group && rate.group !== repeated[i - 1]?.group && (
                <span className="ml-6 select-none border-l border-mandat-line pl-5 pr-2 font-display text-[10px] uppercase tracking-[2px] text-mandat-muted">
                  {rate.group}
                </span>
              )}
              <div
                aria-label={`${rate.name} (${rate.code}): ${rate.value} ${rate.unit ?? 'гривень'}`}
                className="flex items-center gap-2.5 px-6 py-1.5"
              >
                <span
                  aria-hidden="true"
                  className="flex h-5 w-5 shrink-0 items-center justify-center text-mandat-muted"
                >
                  {Icon ? (
                    <Icon className="h-4 w-4" strokeWidth={1.75} />
                  ) : (
                    <span className="text-base leading-none">{rate.flag}</span>
                  )}
                </span>
                <span className="flex flex-col">
                  <span className="font-display text-[11px] leading-tight text-mandat-text">
                    {rate.name}
                  </span>
                  <span className="font-display text-[9px] tracking-[1.5px] leading-tight text-mandat-muted">
                    {rate.code}
                  </span>
                </span>
                <span className="font-display text-[15px] font-semibold tabular-nums text-mandat-text">
                  {rate.value}
                </span>
                <span className="self-start pt-0.5 font-display text-[10px] text-mandat-muted">
                  {rate.unit ?? '₴'}
                </span>
                <span aria-hidden="true" className="ml-4 h-3.5 w-px bg-mandat-line" />
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
