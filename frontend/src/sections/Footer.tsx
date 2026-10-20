export default function Footer() {
  return (
    <footer className="border-t border-[#33373F] py-14 pb-8" style={{ background: 'var(--mandat-surface)' }}>
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-[1.4fr_1fr_1fr_1fr] gap-10">
          <div>
            <a href="#" className="flex items-center gap-3 no-underline" style={{ color: 'var(--mandat-text)' }}>
              <span className="inline-block w-[12px] h-[34px] bg-[#B3233A]" />
              <span className="font-['Oswald'] text-[22px] font-bold tracking-[2px]">МАНДАТ</span>
            </a>
            <p className="text-[#8A8D91] text-[14px] mt-4 max-w-[280px]">
              Про владу, рішення, гроші та вплив. Новини, інсайди, аналітика.
            </p>
          </div>
          <div>
            <h4 className="font-['Oswald'] text-[14px] tracking-[2px] uppercase mb-4">Розділи</h4>
            <ul className="grid gap-2.5 list-none p-0">
              {['Новини', 'Аналітика', 'Інсайди', 'Розслідування'].map((item) => (
                <li key={item}>
                  <a href="#" className="text-[#8A8D91] text-[14px] hover:text-[#B3233A] transition-colors no-underline">
                    {item}
                  </a>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h4 className="font-['Oswald'] text-[14px] tracking-[2px] uppercase mb-4">Про проєкт</h4>
            <ul className="grid gap-2.5 list-none p-0">
              {['Про нас', 'Редакційна політика', 'Реклама', 'Контакти'].map((item) => (
                <li key={item}>
                  <a href="#" className="text-[#8A8D91] text-[14px] hover:text-[#B3233A] transition-colors no-underline">
                    {item}
                  </a>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h4 className="font-['Oswald'] text-[14px] tracking-[2px] uppercase mb-4">Соцмережі</h4>
            <ul className="grid gap-2.5 list-none p-0">
              {['Telegram', 'Facebook', 'YouTube', 'RSS'].map((item) => (
                <li key={item}>
                  <a href="#" className="text-[#8A8D91] text-[14px] hover:text-[#B3233A] transition-colors no-underline">
                    {item}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </div>
        <div className="mt-12 pt-6 border-t border-[#33373F] flex flex-wrap justify-between text-[#8A8D91] text-[13px] gap-3">
          <span>© 2026 МАНДАТ. Усі права захищено.</span>
          <span>Чіткість та структура · Мінімалізм · Контраст та ієрархія</span>
        </div>
      </div>
    </footer>
  );
}
