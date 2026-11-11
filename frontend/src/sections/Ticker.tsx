export default function Ticker({ items }: { items: string[] }) {
  if (items.length === 0) return null

  // Repeat the list so the 50% translate loop looks seamless.
  const repeated = [...items, ...items, ...items, ...items]

  return (
    <div className="overflow-hidden border-y border-mandat-line py-1 bg-mandat-surface mt-16">
      <div className="flex gap-12 whitespace-nowrap animate-[scroll_90s_linear_infinite] w-max">
        {repeated.map((text, i) => (
          <span key={i} className="font-display text-[12px] tracking-[1.5px] uppercase text-mandat-muted">
            <b className="text-mandat-red-bright mr-2.5">▮</b>
            {text}
          </span>
        ))}
      </div>
    </div>
  )
}
