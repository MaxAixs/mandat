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
/* The format cards themselves live in the database: see src/api/adFormats.ts. */

/** Admin breadcrumb label of the ad format cards section. */
export const ADVERTISING_ADMIN_SECTION = 'Реклама'

export const ADVERTISING_CTA_TITLE = 'Будемо раді співпраці!'
