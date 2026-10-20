const items = [
  'Верховна Рада ухвалила закон про мобілізацію',
  'Уряд схвалив новий бюджет на 2026 рік',
  'НБУ змінив облікову ставку',
  'Тіньові потоки: як працюють схеми в держтендерах',
];

export default function Ticker() {
  const doubled = [...items, ...items, ...items, ...items];
  return (
    <div className="overflow-hidden border-y border-[#33373F] py-3" style={{ background: 'var(--mandat-surface)' }}>
      <div className="flex gap-12 whitespace-nowrap animate-[scroll_30s_linear_infinite] w-max">
        {doubled.map((text, i) => (
          <span key={i} className="font-['Oswald'] text-[14px] tracking-[2px] uppercase text-[#8A8D91]">
            <b className="text-[#B3233A] mr-2.5">▮</b>
            {text}
          </span>
        ))}
      </div>
      <style>{`
        @keyframes scroll {
          to { transform: translateX(-50%); }
        }
      `}</style>
    </div>
  );
}
