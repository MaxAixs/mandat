import type { NavLink } from '@/types'

export const SITE_NAME = 'МАНДАТ'
export const SITE_TAGLINE = 'новини · інсайди · аналітика'

/** Header navigation. href = anchor of the target section. */
export const NAV_LINKS: NavLink[] = [
  { label: 'Новини', href: '#news' },
  { label: 'Аналітика', href: '#analytics' },
  { label: 'Інсайди', href: '#insight' },
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
