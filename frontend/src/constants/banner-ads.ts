/** Content of the /advertising/banners page: what a banner buys, targeting and file requirements. */

import articleDesktop from '@/assets/banners/article-desktop.jpg'
import articleMobile from '@/assets/banners/article-mobile.jpg'
import categoryDesktop from '@/assets/banners/category-desktop.jpg'
import feedDesktop from '@/assets/banners/feed-desktop.jpg'
import feedMobile from '@/assets/banners/feed-mobile.jpg'
import homeDesktop from '@/assets/banners/home-desktop.jpg'
import homeDesktopBottom from '@/assets/banners/home-desktop-bottom.jpg'
import homeMobile from '@/assets/banners/home-mobile.jpg'

export const BANNER_ADS_PATH = '/advertising/banners'

/**
 * Title of the ad format card (managed in the admin panel) that opens this page.
 * If the card is renamed in the admin panel, update it here too.
 */
export const BANNER_ADS_FORMAT_TITLE = 'Банерна реклама'

export const BANNER_ADS_TITLE = 'Банерна реклама'
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
}

export const BANNER_ADS_SETTINGS: BannerAdsSetting[] = [
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

export interface BannerAdsShot {
  image: string
  /** Short description of the screenshot for screen readers. */
  alt: string
  caption: string
  /** Mobile screenshots are rendered narrower, next to their caption. */
  mobile?: boolean
  slots: { size: string; text: string }[]
}

/** A page type of the site with example screenshots of its banner slots. */
export interface BannerAdsPlacement {
  title: string
  text: string
  shots: BannerAdsShot[]
}

export const BANNER_ADS_PLACEMENTS: BannerAdsPlacement[] = [
  {
    title: 'Головна сторінка',
    text: 'Перше, що бачить читач, коли відкриває сайт. Банер стоїть поруч із головними новинами дня.',
    shots: [
      {
        image: homeDesktop,
        alt: 'Головна сторінка «Мандату» на комп’ютері: банер 970×250 під стрічкою курсів, над блоком «Головні новини»',
        caption: 'Комп’ютер',
        slots: [{ size: '970×250', text: 'під стрічкою курсів валют і цін на пальне, одразу над головними новинами' }],
      },
      {
        image: homeDesktopBottom,
        alt: 'Головна сторінка «Мандату» на комп’ютері: банер 970×90 між головними та останніми новинами',
        caption: 'Комп’ютер, нижче на сторінці',
        slots: [{ size: '970×90', text: 'між головними та останніми новинами' }],
      },
      {
        image: homeMobile,
        alt: 'Головна сторінка «Мандату» на телефоні: банер 300×250 під стрічкою курсів',
        caption: 'Телефон',
        mobile: true,
        slots: [{ size: '300×250', text: 'під стрічкою курсів, перед головними новинами' }],
      },
    ],
  },
  {
    title: 'Сторінка статті',
    text: 'Банер супроводжує читача весь час, поки той читає матеріал.',
    shots: [
      {
        image: articleDesktop,
        alt: 'Сторінка статті на комп’ютері: банер 300×600 праворуч, 728×90 у тексті і кетфіш 970×90 унизу екрана',
        caption: 'Комп’ютер',
        slots: [
          { size: '300×600 або 300×250', text: 'праворуч від статті; залишається на екрані, поки читач гортає сторінку' },
          { size: '728×90', text: 'у тексті, після першого абзацу' },
          { size: 'Кетфіш 970×90', text: 'закріплений унизу екрана; читач може його закрити' },
        ],
      },
      {
        image: articleMobile,
        alt: 'Сторінка статті на телефоні: банер 300×250 у тексті і кетфіш 320×100 унизу екрана',
        caption: 'Телефон',
        mobile: true,
        slots: [
          { size: '300×250', text: 'у тексті, після першого абзацу' },
          { size: 'Кетфіш 320×100', text: 'закріплений унизу екрана; читач може його закрити' },
        ],
      },
    ],
  },
  {
    title: 'Сторінки рубрик',
    text: 'Економіка, Політика, Спорт та інші розділи. Підходить, якщо ваша реклама пов’язана з темою рубрики.',
    shots: [
      {
        image: categoryDesktop,
        alt: 'Рубрика «Економіка» на комп’ютері: банер 300×250 у сітці матеріалів і 970×90 під ними',
        caption: 'Комп’ютер',
        slots: [
          { size: '300×250', text: 'у сітці матеріалів, на місці однієї з карток' },
          { size: '970×90', text: 'під матеріалами рубрики' },
        ],
      },
    ],
  },
  {
    title: 'Стрічка «Всі новини»',
    text: 'Повний список свіжих новин із фільтром за розділами.',
    shots: [
      {
        image: feedDesktop,
        alt: 'Сторінка «Всі новини» на комп’ютері: банер 970×90 у списку новин',
        caption: 'Комп’ютер',
        slots: [{ size: '970×90', text: 'у списку, після п’ятої новини' }],
      },
      {
        image: feedMobile,
        alt: 'Сторінка «Всі новини» на телефоні: банер 300×250 у списку новин',
        caption: 'Телефон',
        mobile: true,
        slots: [{ size: '300×250', text: 'у списку, після другої новини' }],
      },
    ],
  },
]

export const BANNER_ADS_SPECS_TITLE = 'Вимоги до макета'

export const BANNER_ADS_SPECS: { label: string; value: string; note?: string }[] = [
  { label: 'Формат файлу', value: 'JPEG, PNG або GIF' },
  { label: 'Вага', value: 'До 200 КБ' },
  { label: 'Розмір', value: '300×250, 300×600, 728×90, 970×90 або 970×250', note: 'Залежить від місця розміщення.' },
  {
    label: 'Мобільний кетфіш',
    value: '320 × 100 px і 640 × 200 px',
    note: 'Потрібні обидві версії: друга — з тими самими пропорціями, лише вдвічі більша.',
  },
]

export const BANNER_ADS_CTA_TITLE = 'Готові обговорити розміщення?'
