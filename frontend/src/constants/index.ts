import type { NavLink } from '@/types'

export const SITE_NAME = 'МАНДАТ'
export const SITE_TAGLINE = 'новини · інсайди · аналітика'

/** Telegram channel promoted on the site. */
export const TELEGRAM_CHANNEL = {
  name: 'МАНДАТ у Telegram',
  url: 'https://t.me/mandat_ua',
  subscribers: '42 728',
}

/** Header navigation. */
export const NAV_LINKS: NavLink[] = [
  { label: 'Новини', href: '/' },
  { label: 'Міжнародні новини', href: '#' },
  { label: 'Спорт', href: '#' },
  { label: 'Інтерв\'ю', href: '#' },
  { label: 'Про нас', href: '#about' },
]

/** Footer link columns. */
export const FOOTER_LINKS: { title: string; links: NavLink[] }[] = [
  {
    title: 'Розділи',
    links: [
      { label: 'Новини', href: '#news' },
      { label: 'Аналітика', href: '#analytics' },
      { label: 'Інсайди', href: '#insight' },
      { label: 'Розслідування', href: '#insight' },
    ],
  },
  {
    title: 'Про проєкт',
    links: [
      { label: 'Про нас', href: '#about' },
      { label: 'Редакційна політика', href: '#' },
      { label: 'Реклама', href: '#' },
      { label: 'Контакти', href: '#' },
    ],
  },
  {
    title: 'Соцмережі',
    links: [
      { label: 'Telegram', href: '#' },
      { label: 'Facebook', href: '#' },
      { label: 'YouTube', href: '#' },
      { label: 'RSS', href: '#' },
    ],
  },
]
