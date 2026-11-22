import { Link } from 'react-router'
import { ArrowRight, ArrowUpRight } from 'lucide-react'

import type { AdFormat } from '@/types'

interface AdvertisingFormatCardProps {
  format: AdFormat
  /** External or mailto link. */
  href: string
  /** Site route; when set, the card opens this page instead of `href`. */
  to?: string
  ariaLabel: string
}

const cardClassName =
  'group relative isolate flex min-h-[260px] flex-col justify-end overflow-hidden border border-mandat-line bg-mandat-surface-2 p-5 no-underline transition-colors hover:border-mandat-red-bright focus-visible:border-mandat-red-bright focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-mandat-red-bright sm:min-h-[320px] sm:p-6'

// Ad format tile: photo on the background, title and description over a dark scrim at the bottom.
export default function AdvertisingFormatCard({ format, href, to, ariaLabel }: AdvertisingFormatCardProps) {
  const Arrow = to ? ArrowRight : ArrowUpRight
  const content = (
    <>
      {format.image_url ? (
        <img
          src={format.image_url}
          alt=""
          loading="lazy"
          className="absolute inset-0 -z-20 size-full object-cover transition-transform duration-700 ease-out group-hover:scale-105 motion-reduce:transition-none motion-reduce:group-hover:scale-100"
        />
      ) : (
        <span className="absolute inset-0 -z-20 bg-gradient-to-br from-mandat-red to-mandat-surface-2" aria-hidden="true" />
      )}
      <span className="absolute inset-0 -z-10 bg-gradient-to-t from-black/90 via-black/55 to-black/10" aria-hidden="true" />

      <Arrow
        className="absolute right-5 top-5 size-6 text-white/80 transition-colors group-hover:text-white sm:right-6 sm:top-6"
        strokeWidth={1.5}
        aria-hidden="true"
      />

      <span className="block text-[20px] font-bold leading-tight text-white sm:text-[22px]">{format.title}</span>
      <span className="mt-2.5 block max-w-[34ch] text-[14px] leading-relaxed text-white/75">{format.description}</span>
    </>
  )

  return to ? (
    <Link to={to} aria-label={ariaLabel} className={cardClassName}>
      {content}
    </Link>
  ) : (
    <a href={href} aria-label={ariaLabel} className={cardClassName}>
      {content}
    </a>
  )
}
