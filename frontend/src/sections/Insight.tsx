const INSIGHT_POINTS = [
  'Факти без емоцій — тільки перевірена інформація',
  'Аналітика без шуму — суть, а не клікбейт',
  'Інсайди, яким довіряють',
]

export default function Insight() {
  return (
    <section id="insight" className="py-[72px]">
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="grid grid-cols-1 lg:grid-cols-[0.9fr_1.1fr] gap-12 items-center">
          <div className="h-[380px] border border-mandat-line relative overflow-hidden">
            <div
              className="absolute inset-0"
              style={{
                background: `
                  linear-gradient(200deg, rgba(23,25,28,0.1), rgba(23,25,28,0.9)),
                  repeating-radial-gradient(circle at 30% 30%, #262a31 0 2px, transparent 2px 26px),
                  linear-gradient(120deg, #2a2e35, #1a1d21)
                `,
              }}
            />
            <div className="absolute top-6 left-6 right-6 bottom-6 border border-[rgba(179,35,58,0.4)]" />
            <span className="absolute top-3.5 left-3.5 font-display text-[12px] tracking-[2px] uppercase text-mandat-red-bright">
              Мандат / Розслідування
            </span>
            <span className="absolute bottom-3.5 right-3.5 font-display text-[12px] tracking-[2px] uppercase text-mandat-red-bright">
              Ексклюзив
            </span>
          </div>
          <div>
            <h2 className="text-[38px] tracking-wide leading-tight">
              Інсайди, які не публікують у пресрелізах
            </h2>
            <p className="text-mandat-muted mt-5 max-w-[480px]">
              Влада, рішення, гроші та вплив. Ми працюємо з джерелами всередині системи та перевіряємо кожен факт.
            </p>
            <ul className="mt-7 grid gap-3.5">
              {INSIGHT_POINTS.map((point) => (
                <li key={point} className="flex gap-3.5 items-baseline text-[15px] list-none">
                  <span className="w-3.5 h-0.5 bg-mandat-red-bright flex-none translate-y-[-4px]" />
                  {point}
                </li>
              ))}
            </ul>
            <a
              href="#"
              className="inline-block mt-8 px-9 py-3.5 border border-mandat-red-bright font-display text-[15px] tracking-[2px] uppercase transition-colors hover:bg-mandat-red-bright no-underline text-mandat-text"
            >
              Читати розслідування
            </a>
          </div>
        </div>
      </div>
    </section>
  )
}
