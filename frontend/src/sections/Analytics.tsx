const stats = [
  { val: '7,8', unit: 'млрд грн', cap: 'направлено на відновлення регіонів станом на жовтень 2026' },
  { val: '+4,2', unit: '%', cap: 'зростання ВВП за три квартали поспіль' },
  { val: '1 240', unit: '', cap: 'депутатських запитів за поточну сесію' },
  { val: '92', unit: '%', cap: 'законів, ухвалених із порушенням регламенту — дослідження' },
];

export default function Analytics() {
  return (
    <section id="analytics" className="border-y border-[#33373F]" style={{ background: 'var(--mandat-surface)' }}>
      <div className="mx-auto max-w-[1240px] px-6 py-[72px]">
        <div className="flex items-baseline justify-between mb-10">
          <h2 className="text-[36px] tracking-[2px] font-semibold">
            <span className="inline-block w-2 h-7 bg-[#B3233A] mr-3.5 align-middle" />
            Цифра дня
          </h2>
          <a href="#" className="font-['Oswald'] text-[14px] tracking-[2px] uppercase text-[#8A8D91] hover:text-[#B3233A] transition-colors no-underline">
            Вся аналітика →
          </a>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          {stats.map((s, i) => (
            <div key={i} className="border-l-[2px] border-[#B3233A] pl-4">
              <div className="font-['Oswald'] text-[44px] font-bold leading-none">
                {s.val}
                {s.unit && <small className="text-[20px] text-[#8A8D91] ml-1">{s.unit}</small>}
              </div>
              <div className="text-[#8A8D91] text-[13px] mt-2 uppercase tracking-widest">{s.cap}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
