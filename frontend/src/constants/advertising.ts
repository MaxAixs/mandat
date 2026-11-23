/** Content of the /advertising page: audience and ad formats. */

export const ADVERTISING_TITLE = 'Реклама на «Мандаті»'
export const ADVERTISING_PATH = '/advertising'
/** Back link label on the pages of individual ad formats. */
export const ADVERTISING_BACK_LABEL = 'Усі формати реклами'

export const ADVERTISING_LEAD =
  'Говоримо з аудиторією, яка ухвалює рішення: про владу, гроші та вплив. Допоможемо розповісти про ваш бренд чесно, без шуму й у форматі, що поважає читача.'

/** Label of the buttons that lead to the contacts page. */
export const ADVERTISING_ORDER_LABEL = 'Замовити рекламу'

export const ADVERTISING_AUDIENCE_TITLE = 'Соціальні мережі'

/**
 * Followers by `SOCIAL_LINKS` id, awaiting real figures. Telegram is not here: its
 * count comes live from the API (refreshed hourly from the channel page).
 */
export const ADVERTISING_AUDIENCE: Record<string, string> = {
  instagram: '—',
  threads: '—',
  facebook: '—',
}

export const ADVERTISING_FORMATS_TITLE = 'Формати, з якими ми працюємо'
export const ADVERTISING_FORMATS_LEAD =
  'Редакція «Мандату» працює з різними форматами реклами: нативна та пряма реклама, банери й проєкти, створені під клієнта.'
/* The format cards themselves live in the database: see src/api/adFormats.ts. */

/** Admin breadcrumb label of the ad format cards section. */
export const ADVERTISING_ADMIN_SECTION = 'Реклама'

export const ADVERTISING_CTA_TITLE = 'Будемо раді співпраці!'
