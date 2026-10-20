export default function Header() {
  return (
    <header className="sticky top-0 z-50 border-b border-[#33373F]" style={{ background: 'rgba(23,25,28,0.92)', backdropFilter: 'blur(12px)' }}>
      <div className="mx-auto max-w-[1240px] px-6">
        <div className="flex items-center justify-between h-[72px]">
          <a href="#" className="flex items-center gap-3 no-underline" style={{ color: 'var(--mandat-text)' }}>
            <span className="inline-block w-[12px] h-[34px] bg-[#B3233A]" />
            <span>
              <span className="font-['Oswald'] text-[26px] font-bold tracking-[2px] leading-none block">МАНДАТ</span>
              <span className="text-[10px] tracking-[3px] text-[#8A8D91] uppercase block mt-0.5">новини · інсайди · аналітика</span>
            </span>
          </a>

          <nav className="hidden md:flex gap-9">
            {['Новини', 'Аналітика', 'Інсайди', 'Про нас'].map((item) => (
              <a
                key={item}
                href={`#${item.toLowerCase()}`}
                className="relative font-['Oswald'] text-[15px] tracking-[2px] uppercase py-1.5 group"
                style={{ color: 'var(--mandat-text)' }}
              >
                {item}
                <span className="absolute left-0 bottom-0 h-[2px] w-0 bg-[#B3233A] transition-all duration-250 group-hover:w-full" />
              </a>
            ))}
          </nav>

          <div className="flex items-center gap-4">
            <button
              aria-label="Пошук"
              className="w-10 h-10 grid place-items-center border border-[#33373F] text-[#8A8D91] transition-colors hover:border-[#B3233A] hover:text-[var(--mandat-text)] bg-transparent cursor-pointer"
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="11" cy="11" r="7" />
                <path d="m21 21-4-4" />
              </svg>
            </button>
            <button className="font-['Oswald'] text-[14px] tracking-[2px] uppercase px-6 py-2.5 transition-colors cursor-pointer" style={{ background: '#8E1B2D', color: 'var(--mandat-text)' }} onMouseEnter={(e) => (e.currentTarget.style.background = '#B3233A')} onMouseLeave={(e) => (e.currentTarget.style.background = '#8E1B2D')}>
              Підписатися
            </button>
          </div>
        </div>
      </div>
    </header>
  );
}
