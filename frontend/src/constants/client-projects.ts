/** Content of the /advertising/client-projects page: sections, podcasts and video series made for a client. */

import type { AdFormatItem } from '@/types'

export const CLIENT_PROJECTS_PATH = '/advertising/client-projects'

/**
 * Title of the ad format card (managed in the admin panel) that opens this page.
 * If the card is renamed in the admin panel, update it here too.
 */
export const CLIENT_PROJECTS_FORMAT_TITLE = 'Проєкти під клієнта'

export const CLIENT_PROJECTS_TITLE = 'Проєкти під клієнта'
export const CLIENT_PROJECTS_LEAD = 'Створення розділу, подкасту, серії роликів під клієнта.'

export const CLIENT_PROJECTS_PRINCIPLE =
  'Створення окремого лендінгу та матеріалів різного формату: це дозволить розкрити вашу компанію чи послугу з різних сторін, охопити більше цікавих тем і ширшу аудиторію.'

export const CLIENT_PROJECTS_FORMATS_TITLE = 'Що можемо створити'

export const CLIENT_PROJECTS_FORMATS: AdFormatItem[] = [
  {
    title: 'Розділ на сайті',
    text: 'Окрема рубрика з серією матеріалів на теми, важливі для вашої компанії.',
  },
  {
    title: 'Подкаст',
    text: 'Цикл випусків, у яких ваша компанія чи послуга стає частиною розмови з гостями.',
  },
  {
    title: 'Серія роликів',
    text: 'Відео для сайту та соцмереж, об’єднані однією темою та стилем.',
  },
  {
    title: 'Лендінг',
    text: 'Окрема сторінка проєкту, що збирає всі матеріали разом і веде читача від однієї теми до іншої.',
  },
]

export const CLIENT_PROJECTS_CTA_TITLE = 'Придумаємо проєкт разом?'
