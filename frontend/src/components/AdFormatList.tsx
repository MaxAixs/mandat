import type { AdFormatItem } from '@/types'

interface AdFormatListProps {
  items: AdFormatItem[]
}

/** Numbered list of the variants of an ad format: title on the left, description on the right. */
export default function AdFormatList({ items }: AdFormatListProps) {
  return (
    <ol className="m-0 mt-8 list-none border-t border-mandat-line p-0">
      {items.map((item, index) => (
        <li
          key={item.title + (item.subtitle ?? '')}
          className="grid gap-2 border-b border-mandat-line py-6 sm:grid-cols-[260px_1fr] sm:gap-8 md:py-7"
        >
          <h3 className="flex items-baseline gap-4 text-[20px] font-semibold text-white md:text-[22px]">
            <span className="w-7 shrink-0 text-[15px] font-medium tabular-nums text-mandat-red-bright">
              {String(index + 1).padStart(2, '0')}
            </span>
            <span>
              {item.title}
              {item.subtitle && (
                <span className="mt-1 block text-[15px] font-medium text-mandat-muted">{item.subtitle}</span>
              )}
            </span>
          </h3>
          <p className="m-0 max-w-[60ch] pl-11 text-[16px] leading-relaxed text-mandat-muted sm:pl-0 md:text-[17px]">
            {item.text}
          </p>
        </li>
      ))}
    </ol>
  )
}
