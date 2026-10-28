import type { Article, LatestNewsItem, StatItem } from '@/types'

/**
 * Mock content used until the Go backend is ready.
 * When the API lands, replace imports of this module with real
 * fetch calls (see index.ts) — the types stay the same.
 */

export const mockArticles: Article[] = [
  {
    id: 'byudzhet-2026',
    category: "Інтерв'ю",
    title: 'Уряд схвалив новий бюджет на 2026 рік',
    excerpt:
      'Ключові цифри, розподіл та пріоритети. Розбираємо, що зміниться для економіки, бізнесу та кожного з нас.',
    views: '12.4K',
    publishedAt: '12:30',
    comments: '32 коментарі',
    featured: true,
  },
  {
    id: 'zakon-mobilizatsiya',
    category: 'Новини',
    title: 'Верховна Рада ухвалила закон про мобілізацію',
    excerpt: 'Що змінюється — коротко про головне.',
    views: '8.7K',
    publishedAt: '11:45',
  },
  {
    id: 'nbu-stavka',
    category: 'Міжнародні новини',
    title: 'Чому рішення НБУ важливіше, ніж здається',
    excerpt: 'Наслідки для економіки та гривні.',
    views: '9.1K',
    publishedAt: '09:10',
  },
  {
    id: 'rynok-zemli',
    category: 'Спорт',
    title: 'Хто лобіює ринок землі насправді',
    excerpt: 'Розкриваємо інтереси та бенефіціарів.',
    views: '15.2K',
    publishedAt: '20:22',
  },
  {
    id: 'tinovi-potoky-2',
    category: 'Спорт',
    title: 'Тіньові потоки: як працюють схеми у держтендерах',
    excerpt: 'Частина друга — механізми та прізвища.',
    views: '14.6K',
    publishedAt: '18:40',
  },
  {
    id: 'korrupciya-indeks',
    category: 'Новини',
    title: 'Україна піднялася у рейтингу сприйняття корупції',
    excerpt: 'Підсумки року — позиція порівняно з сусідами по регіону.',
    views: '11.3K',
    publishedAt: '16:05',
  },
]

/** Entries for the "Останні новини" feed. */
export const mockLatestNews: LatestNewsItem[] = [
  {
    id: 'ln-1',
    time: '13:59',
    source: 'Парламент',
    title: 'У Раді зареєстрували законопроєкт про зміни до регламенту',
  },
  {
    id: 'ln-2',
    time: '13:49',
    source: 'Економіка',
    title: 'НБУ опублікував новий макропрогноз: що змінилося для курсу гривні',
  },
  {
    id: 'ln-3',
    time: '13:48',
    badge: 'Фото',
    title: 'На кордоні затримали чоловіка, який одружився на майбутній тещі, аби виїхати за кордон',
  },
  {
    id: 'ln-4',
    time: '13:40',
    important: true,
    title: 'По цій довідці знижка 20% на підписку «Мандату». Долучайтеся',
  },
  {
    id: 'ln-5',
    time: '13:27',
    source: 'Економіка',
    title: 'ФДМУ виставив на продаж завод у Броварах Київської області',
  },
  {
    id: 'ln-6',
    time: '13:24',
    title: 'У найближчі дні в Україні будуть дощі',
  },
  {
    id: 'ln-7',
    time: '12:58',
    source: 'Економіка',
    title: 'Укрзалізниця встановила у Києві та області 33 мобільні укриття',
  },
  {
    id: 'ln-8',
    time: '12:50',
    badge: 'Відео',
    title: 'Як працюють мобільні укриття: пояснюємо за дві хвилини',
  },
  {
    id: 'ln-9',
    time: '12:46',
    title: 'Прем\'єр Польщі заявив, що рішення про проголошення 2027 року роком УПА шкодить Україні',
  },
  {
    id: 'ln-10',
    time: '12:31',
    source: 'Економіка',
    title: 'Нафта подорожчала понад 110 доларів за барель',
  },
]

/** Headlines for the scrolling ticker. */
export const mockTickerItems: string[] = [
  'Уряд схвалив новий бюджет на 2026 рік',
  'НБУ змінив облікову ставку',
  'Тіньові потоки: як працюють схеми в держтендерах',
]

/** "Цифра дня" statistics. */
export const mockStats: StatItem[] = [
  { value: '7,8', unit: 'млрд грн', caption: 'направлено на відновлення регіонів станом на жовтень 2026' },
  { value: '+4,2', unit: '%', caption: 'зростання ВВП за три квартали поспіль' },
  { value: '1 240', caption: 'депутатських запитів за поточну сесію' },
  { value: '92', unit: '%', caption: 'законів, ухвалених із порушенням регламенту — дослідження' },
]
