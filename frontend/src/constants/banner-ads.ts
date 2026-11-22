/** Content of the /advertising/banners page: what a banner buys, targeting and file requirements. */

import articleDesktop from '@/assets/banners/article-desktop.jpg'
import homeDesktop from '@/assets/banners/home-desktop.jpg'
import homeMobile from '@/assets/banners/home-mobile.jpg'

export const BANNER_ADS_PATH = '/advertising/banners'

/**
 * Title of the ad format card (managed in the admin panel) that opens this page.
 * If the card is renamed in the admin panel, update it here too.
 */
export const BANNER_ADS_FORMAT_TITLE = 'Банерна реклама'

export const BANNER_ADS_TITLE = 'Банерна реклама'
export const BANNER_ADS_BACK_LABEL = 'Усі формати реклами'
export const BANNER_ADS_LEAD =
  'Як працює розміщення банерів на «Мандаті»: що саме ви купуєте, які налаштування показу доступні та яким має бути макет.'

export const BANNER_ADS_PRINCIPLE =
  'Ви купуєте покази банера на сторінках наших сайтів, а не доступ до певної аудиторії. Тому налаштувати показ за віком, інтересами чи іншими ознаками читачів не вийде.'

export const BANNER_ADS_SETTINGS_TITLE = 'Що можна налаштувати'
export const BANNER_ADS_EXAMPLE_LABEL = 'Наприклад:'

export interface BannerAdsSetting {
  title: string
  text: string
  example?: string
  options?: string[]
}

export const BANNER_ADS_SETTINGS: BannerAdsSetting[] = [
  {
    title: 'Географія',
    text: 'Банер можна показувати лише читачам з певної території.',
    options: ['Країна', 'Область', 'Велике місто'],
  },
  {
    title: 'Частота',
    text: 'Скільки разів одна людина побачить банер за обраний період.',
    example: 'Не більше двох показів на тиждень',
  },
  {
    title: 'Розклад',
    text: 'У які дні тижня й години доби банер буде в ротації.',
    example: 'Тільки у вихідні, з 9:00 до 21:00',
  },
]

export const BANNER_ADS_PLACEMENTS_TITLE = 'Де показуються банери'

export interface BannerAdsPlacement {
  image: string
  /** Short description of the screenshot for screen readers. */
  alt: string
  caption: string
  /** Mobile screenshots are rendered narrower. */
  mobile?: boolean
  slots: { size: string; text: string }[]
}

export const BANNER_ADS_PLACEMENTS: BannerAdsPlacement[] = [
  {
    image: homeDesktop,
    alt: 'Головна сторінка «Мандату» на комп’ютері: банери 300×250 і 300×600 праворуч від стрічки новин',
    caption: 'Головна сторінка, комп’ютер',
    slots: [
      { size: '300×250', text: 'у другому екрані головної сторінки, праворуч від новин' },
      { size: '300×600', text: 'під ним; залишається на екрані, поки читач гортає сторінку' },
    ],
  },
  {
    image: homeMobile,
    alt: 'Головна сторінка «Мандату» на телефоні: банер 300×250 над блоком «Головні новини»',
    caption: 'Головна сторінка, телефон',
    mobile: true,
    slots: [{ size: '300×250', text: 'над стрічкою новин' }],
  },
  {
    image: articleDesktop,
    alt: 'Сторінка статті на комп’ютері: банер праворуч від тексту і кетфіш 970×90 унизу екрана',
    caption: 'Сторінка статті, комп’ютер',
    slots: [
      { size: '300×250 або 300×600', text: 'у першому екрані, праворуч від статті' },
      { size: 'Кетфіш 970×90', text: 'закріплений унизу екрана; читач може його закрити' },
    ],
  },
]

export const BANNER_ADS_SPECS_TITLE = 'Вимоги до макета'

export const BANNER_ADS_SPECS: { label: string; value: string; note?: string }[] = [
  { label: 'Формат файлу', value: 'JPEG, PNG або GIF' },
  { label: 'Вага', value: 'До 200 КБ' },
  { label: 'Розмір', value: 'Залежить від місця розміщення' },
  {
    label: 'Мобільний кетфіш',
    value: '320 × 100 px і 640 × 200 px',
    note: 'Потрібні обидві версії: друга — з тими самими пропорціями, лише вдвічі більша.',
  },
]

export const BANNER_ADS_CTA_TITLE = 'Готові обговорити розміщення?'
export const BANNER_ADS_ORDER_SUBJECT = 'Банерна реклама на «Мандаті»'
