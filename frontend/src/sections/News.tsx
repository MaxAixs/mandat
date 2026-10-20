const news = [
  {
    badge: 'Важливо',
    title: 'Уряд схвалив новий бюджет на 2026 рік',
    desc: 'Ключові цифри, розподіл та пріоритети. Розбираємо, що зміниться для економіки, бізнесу та кожного з нас.',
    views: '12.4K',
    time: '12:30',
    comments: '32 коментарі',
    featured: true,
  },
  {
    badge: 'Новини',
    title: 'Верховна Рада ухвалила закон про мобілізацію',
    desc: 'Що змінюється — коротко про головне.',
    views: '8.7K',
    time: '11:45',
    featured: false,
  },
  {
    badge: 'Аналітика',
    title: 'Чому рішення НБУ важливіше, ніж здається',
    desc: 'Наслідки для економіки та гривні.',
    views: '9.1K',
    time: '09:10',
    featured: false,
  },
  {
    badge: 'Інсайди',
    title: 'Хто лобіює ринок землі насправді',
    desc: 'Розкриваємо інтереси та бенефіціарів.',
    views: '15.2K',
    time: '20:22',
    featured: false,
  },
  {
    badge: 'Інсайди',
    title: 'Тіньові потоки: як працюють схеми у держтендерах',
    desc: 'Частина друга — механізми та прізвища.',
    views: '14.6K',
    time: '18:40',
    featured: false,
  },
];

function Card({ item }: { item: typeof news[0] }) {
  return (
    <article
      className={`border border-[#33373F] transition-all duration-200 hover:border-[#B3233A] hover:-translate-y-1 ${
        item.featured ? 'lg:col-span-2 lg:row-span-2 flex flex-col' : ''
      }`}
      style={{ background: 'var(--mandat-surface)' }}
    >
      <div className={`relative overflow-hidden ${item.featured ? 'h-[340px]' : 'h-[180px]'}`}>
        <div
          className="absolute inset-0"
          style={{
            background: `
              linear-gradient(160deg, rgba(23,25,28,0.2), rgba(23,25,28,0.85)),
              repeating-linear-gradient(45deg, #262a31 0 14px, #1d2025 14px 28px)
            `,
          }}
        />
        <div
          className="absolute inset-0"
          style={{
            background: 'radial-gradient(circle at 80% 20%, rgba(179,35,58,0.35), transparent 55%)',
          }}
        />
        <span
          className="absolute top-3.5 left-3.5 z-10 font-['Oswald'] text-[11px] tracking-[2px] uppercase px-3 py-1"
          style={{ background: '#8E1B2D', color: 'var(--mandat-text)' }}
        >
          {item.badge}
        </span>
      </div>
      <div className="p-5 pb-4 flex flex-col flex-1">
        <h3 className={`font-semibold leading-tight tracking-wide ${item.featured ? 'text-[30px]' : 'text-[19px]'}`}>
          {item.title}
        </h3>
        <p className={`text-[#8A8D91] mt-2.5 ${item.featured ? 'text-[15px]' : 'text-[14px]'}`}>{item.desc}</p>
        <div className="flex gap-4 mt-4 pt-3.5 border-t border-[#33373F] text-[#8A8D91] text-[12.5px] tracking-wider uppercase mt-auto">
          <span className="text-[#B3233A]">◉ {item.views}</span>
          <span>{item.time}</span>
          {item.comments && <span>{item.comments}</span>}
        </div>
      </div>
    </article>
  );
}

export default function News() {
  return (
    <section id="news" className="py-[72px]">
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="flex items-baseline justify-between mb-10">
          <h2 className="text-[36px] tracking-[2px] font-semibold">
            <span className="inline-block w-2 h-7 bg-[#B3233A] mr-3.5 align-middle" />
            Головні новини
          </h2>
          <a href="#" className="font-['Oswald'] text-[14px] tracking-[2px] uppercase text-[#8A8D91] hover:text-[#B3233A] transition-colors no-underline">
            Всі новини →
          </a>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {news.map((item, i) => (
            <Card key={i} item={item} />
          ))}
        </div>
      </div>
    </section>
  );
}
