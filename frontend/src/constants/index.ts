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
/** Social profiles linked from the footer. */
export const SOCIAL_LINKS = [
  { id: 'instagram', label: 'Instagram', href: '#' },
  { id: 'threads', label: 'Threads', href: '#' },
  { id: 'telegram', label: 'Telegram', href: TELEGRAM_CHANNEL.url },
  { id: 'facebook', label: 'Facebook', href: '#' },
]

/** Categories available for news articles. */
export const CATEGORIES: Category[] = ['Новини', 'Міжнародні новини', 'Спорт', 'Інтерв\'ю', 'Економіка', 'Політика', 'Регіональні новини']

/** Header navigation. */
export const NAV_LINKS: NavLink[] = [
  { label: 'Новини', href: '/' },
  { label: 'Політика', href: '/politics' },
  { label: 'Економіка', href: '/economics' },
  { label: 'Міжнародні новини', shortLabel: 'Міжнародні', href: '/international' },
  { label: 'Регіональні новини', shortLabel: 'Регіональні', href: '/regional' },
  { label: 'Інтерв\'ю', href: '/interview' },
  { label: 'Спорт', href: '/sport' },
]

/** Footer link columns. The "Розділи" column mirrors the header navigation. */
export const FOOTER_LINKS: { title: string; links: NavLink[] }[] = [
  {
    title: 'Про проєкт',
    links: [
      { label: 'Умови використання матеріалів', href: '/rules' },
      { label: 'Реклама', href: '/advertising' },
      { label: 'Контакти', href: '#' },
    ],
  },
  {
    title: 'Розділи',
    links: NAV_LINKS,
  },
]
