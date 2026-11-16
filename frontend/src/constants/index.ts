import type { Category, NavLink } from '@/types'

export const SITE_NAME = 'МАНДАТ'
export const SITE_TAGLINE = 'новини · інсайди · аналітика'

/** Telegram channel promoted on the site. */
export const TELEGRAM_CHANNEL = {
  name: 'МАНДАТ у Telegram',
  url: 'https://t.me/mandat_ua',
  subscribers: '42 728',
}

/** Legal lines shown at the very bottom of the footer. */
export const FOOTER_COPYRIGHT = `© ${new Date().getFullYear()}, ${SITE_NAME}. Усі права захищено.`
export const FOOTER_USAGE_NOTICE =
  'Використання матеріалів сайту лише за умови посилання (для інтернет-видань — гіперпосилання) на «Мандат» не нижче третього абзацу.'

/** Social profiles linked from the footer. */
export const SOCIAL_LINKS = [
  { id: 'instagram', label: 'Instagram', href: '#' },
  { id: 'threads', label: 'Threads', href: '#' },
  { id: 'telegram', label: 'Telegram', href: TELEGRAM_CHANNEL.url },
]

/** Categories available for news articles. */
export const CATEGORIES: Category[] = ['Новини', 'Міжнародні новини', 'Спорт', 'Інтерв\'ю', 'Про нас', 'Політика', 'Регіональні новини']

/** Header navigation. */
export const NAV_LINKS: NavLink[] = [
  { label: 'Новини', href: '/' },
  { label: 'Політика', href: '/politics' },
  { label: 'Міжнародні новини', href: '/international' },
  { label: 'Регіональні новини', href: '/regional' },
  { label: 'Спорт', href: '/sport' },
  { label: 'Інтерв\'ю', href: '/interview' },
  { label: 'Про нас', href: '/about' },
]

/** Footer link columns. The "Розділи" column mirrors the header navigation. */
export const FOOTER_LINKS: { title: string; links: NavLink[] }[] = [
  {
    title: 'Розділи',
    links: NAV_LINKS,
  },
  {
    title: 'Про проєкт',
    links: [
      { label: 'Про нас', href: '/about' },
      { label: 'Редакційна політика', href: '#' },
      { label: 'Реклама', href: '#' },
      { label: 'Контакти', href: '#' },
    ],
  },
]
