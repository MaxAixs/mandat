export default function Ticker({ headlines }: { headlines: string[] }) {
  if (headlines.length === 0) return null

  // Repeat the list so the 50% translate loop looks seamless.
  const items = [...headlines, ...headlines, ...headlines, ...headlines]

  return (
    <div className="overflow-hidden border-y border-mandat-line py-3 bg-mandat-surface mt-16">
      <div className="flex gap-12 whitespace-nowrap animate-[scroll_60s_linear_infinite] w-max">
        {items.map((text, i) => (
          <span key={i} className="font-['Oswald'] text-[14px] tracking-[2px] uppercase text-mandat-muted">
            <b className="text-mandat-red-bright mr-2.5">▮</b>
            {text}
          </span>
        ))}
      </div>
    </div>
  )
}
