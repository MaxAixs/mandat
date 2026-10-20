import type { Article, StatItem } from '@/types'

/**
 * Mock content used until the Go backend is ready.
 * When the API lands, replace imports of this module with real
 * fetch calls (see index.ts) — the types stay the same.
 */

export const mockArticles: Article[] = [
  {
    id: 'byudzhet-2026',
    category: 'Важливо',
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
    category: 'Аналітика',
    title: 'Чому рішення НБУ важливіше, ніж здається',
    excerpt: 'Наслідки для економіки та гривні.',
    views: '9.1K',
    publishedAt: '09:10',
  },
  {
    id: 'rynok-zemli',
    category: 'Інсайди',
    title: 'Хто лобіює ринок землі насправді',
    excerpt: 'Розкриваємо інтереси та бенефіціарів.',
    views: '15.2K',
    publishedAt: '20:22',
  },
  {
    id: 'tinovi-potoky-2',
    category: 'Інсайди',
    title: 'Тіньові потоки: як працюють схеми у держтендерах',
    excerpt: 'Частина друга — механізми та прізвища.',
    views: '14.6K',
    publishedAt: '18:40',
  },
]

/** Headlines for the scrolling ticker. */
export const mockTickerItems: string[] = [
  'Верховна Рада ухвалила закон про мобілізацію',
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
