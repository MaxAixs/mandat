export default function Digest() {
  return (
    <section
      id="digest"
      className="border-y border-mandat-line"
      style={{ background: 'linear-gradient(120deg, var(--mandat-surface) 60%, #2a1218)' }}
    >
      <div className="mx-auto max-w-[1240px] px-6 py-[72px]">
        <div className="flex flex-wrap items-center justify-between gap-12">
          <div>
            <h2 className="text-[32px] tracking-wide">Дайджест МАНДАТ</h2>
            <p className="text-mandat-muted mt-2">Головне за день — одне повідомлення о 8:00. Без спаму.</p>
          </div>
          <form className="flex flex-wrap gap-0" onSubmit={(e) => e.preventDefault()}>
            <input
              type="email"
              placeholder="ваш@email.com"
              required
              className="w-full sm:w-[300px] px-5 py-3.5 text-[15px] outline-none border border-mandat-line border-r-0 bg-mandat-bg text-mandat-text"
            />
            <button
              type="submit"
              className="w-full sm:w-auto font-['Oswald'] tracking-[2px] uppercase text-[14px] px-8 py-3.5 transition-colors cursor-pointer bg-mandat-red-bright text-mandat-text border-none hover:bg-mandat-red"
            >
              Підписатися
            </button>
          </form>
        </div>
      </div>
    </section>
  )
}
