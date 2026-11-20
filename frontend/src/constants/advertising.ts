/** Content of the /advertising page: audience, ad formats and contacts. */

import { TELEGRAM_CHANNEL } from '@/constants'

export const ADVERTISING_TITLE = 'Реклама на «Мандаті»'

export const ADVERTISING_LEAD =
  'Говоримо з аудиторією, яка ухвалює рішення: про владу, гроші та вплив. Допоможемо розповісти про ваш бренд чесно, без шуму й у форматі, що поважає читача.'

/** Placeholder address until the sales mailbox is confirmed. */
export const ADVERTISING_EMAIL = 'ads@mandat.ua'
export const ADVERTISING_ORDER_LABEL = 'Замовити рекламу'
export const ADVERTISING_ORDER_SUBJECT = 'Замовлення реклами на «Мандаті»'

export const ADVERTISING_AUDIENCE_TITLE = 'Соціальні мережі'

/** Followers by `SOCIAL_LINKS` id; only Telegram is confirmed, the rest await real figures. */
export const ADVERTISING_AUDIENCE: Record<string, string> = {
  instagram: '—',
  threads: '—',
  telegram: TELEGRAM_CHANNEL.subscribers,
  facebook: '—',
}

export const ADVERTISING_FORMATS_TITLE = 'Формати, з якими ми працюємо'
export const ADVERTISING_FORMATS_LEAD =
  'Редакція «Мандату» працює з різними форматами реклами: нативна та пряма реклама, банери й проєкти, створені під клієнта.'
export interface AdvertisingFormat {
  id: string
  title: string
  description: string
  /**
   * Background photo for the card. Put the file in `src/assets/formats/` and import it here;
   * until then the card shows its fallback pattern.
   */
  image?: string
}

export const ADVERTISING_FORMATS: AdvertisingFormat[] = [
  {
    id: 'special-projects',
    title: 'Спецпроєкти',
    description: 'Окрема сторінка з власним дизайном, що глибоко розкриває одну тему разом із брендом.',
  },
  {
    id: 'longreads',
    title: 'Лонгріди',
    description: 'Великі тексти, репортажі та розслідування, у яких бренд стає частиною історії.',
  },
  {
    id: 'video',
    title: 'Відеоінтеграції',
    description: 'Згадка або окремий блок про бренд у наших відео для соцмереж і сайту.',
  },
  {
    id: 'podcasts',
    title: 'Подкасти',
    description: 'Партнерський випуск або інтеграція в розмову з гостями «Мандату».',
  },
  {
    id: 'interactive',
    title: 'Інтерактивні формати',
    description: 'Тести, калькулятори та інші механіки, з якими читач взаємодіє сам.',
  },
  {
    id: 'custom',
    title: 'Проєкти під клієнта',
    description: 'Придумаємо формат з нуля під вашу задачу, аудиторію та бюджет.',
  },
  {
    id: 'banners',
    title: 'Банерна реклама',
    description: 'Розміщення банерів на сайті з гнучким таргетингом за розділами.',
  },
  {
    id: 'partner',
    title: 'Партнерський матеріал',
    description: 'Стаття від редакції на тему бренду з позначкою «Партнерський матеріал».',
  },
]

export const ADVERTISING_CTA_TITLE = 'Будемо раді співпраці!'
