export default function Digest() {
  return (
    <section
      id="digest"
      className="border-y border-[#33373F]"
      style={{ background: 'linear-gradient(120deg, var(--mandat-surface) 60%, #2a1218)' }}
    >
      <div className="mx-auto max-w-[1240px] px-6 py-[72px]">
        <div className="flex flex-wrap items-center justify-between gap-12">
          <div>
            <h2 className="text-[32px] tracking-wide">Дайджест МАНДАТ</h2>
            <p className="text-[#8A8D91] mt-2">Головне за день — одне повідомлення о 8:00. Без спаму.</p>
          </div>
          <form className="flex flex-wrap gap-0" onSubmit={(e) => e.preventDefault()}>
            <input
              type="email"
              placeholder="ваш@email.com"
              required
              className="w-full sm:w-[300px] px-5 py-3.5 text-[15px] outline-none border border-[#33373F] border-r-0"
              style={{ background: 'var(--mandat-bg)', color: 'var(--mandat-text)' }}
            />
            <button
              type="submit"
              className="w-full sm:w-auto font-['Oswald'] tracking-[2px] uppercase text-[14px] px-8 py-3.5 transition-colors cursor-pointer hover:bg-[#d42a45]"
              style={{ background: '#B3233A', color: 'var(--mandat-text)', border: 'none' }}
            >
              Підписатися
            </button>
          </form>
        </div>
      </div>
    </section>
  );
}
