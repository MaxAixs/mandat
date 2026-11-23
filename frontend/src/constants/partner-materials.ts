/** Content of the /advertising/partner-materials page: PROMOTED materials about a partner's brand. */

import type { AdFormatItem } from '@/types'

export const PARTNER_MATERIALS_PATH = '/advertising/partner-materials'

/**
 * Title of the ad format card (managed in the admin panel) that opens this page.
 * If the card is renamed in the admin panel, update it here too.
 */
export const PARTNER_MATERIALS_FORMAT_TITLE = 'Партнерський матеріал'

export const PARTNER_MATERIALS_TITLE = 'Партнерський матеріал'
export const PARTNER_MATERIALS_LEAD =
  'Публікація про ваш бренд, продукт або подію на «Мандаті» з позначкою PROMOTED. Оберіть подачу: у звичному форматі сайту або з власним дизайном.'

export const PARTNER_MATERIALS_PRINCIPLE =
  'Кожен партнерський матеріал позначений PROMOTED: читач бачить, хто розповідає історію, і саме тому їй довіряє.'

export const PARTNER_MATERIALS_FORMATS_TITLE = 'Формати PROMOTED'

export const PARTNER_MATERIALS_FORMATS: AdFormatItem[] = [
  {
    title: 'PROMOTED',
    subtitle: 'новина / стаття',
    text: 'Рекламний текст про ваш бренд, продукт або подію, який ви надаєте для публікації. Матеріал верстається в адмінці платформи у форматах новини або статті.',
  },
  {
    title: 'PROMOTED',
    subtitle: 'унікальний дизайн',
    text: 'Той самий формат рекламного матеріалу, але з унікальним дизайном. Верстка та візуальні елементи адаптуються під стиль вашого бренду й індивідуальні побажання.',
  },
]

export const PARTNER_MATERIALS_CTA_TITLE = 'Розкажемо про ваш бренд?'
