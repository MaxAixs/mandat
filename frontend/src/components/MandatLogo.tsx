import { SITE_NAME } from '@/constants'
import { cn } from '@/lib/utils'

interface MandatLogoProps {
  className?: string
}

/**
 * Text wordmark: «МАНДАТ» in Unbounded Black with a red bar underneath.
 * Pure markup on a transparent background, so it sits cleanly on any surface
 * and scales with `font-size` (set it through `className`, e.g. `text-5xl`).
 */
export default function MandatLogo({ className }: MandatLogoProps) {
  return (
    <span
      role="img"
      aria-label={SITE_NAME}
      className={cn('inline-flex flex-col items-center font-logo font-black leading-none text-white', className)}
    >
      <span aria-hidden="true" className="tracking-[0.02em]">
        {SITE_NAME}
      </span>
      <span aria-hidden="true" className="-mt-[0.12em] h-[0.12em] w-[40%] rounded-full bg-mandat-red" />
    </span>
  )
}
