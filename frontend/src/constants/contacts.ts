/** Content of the /contacts page, where every «Замовити рекламу» button leads. */

import type { ContactType } from '@/types'

export const CONTACTS_PATH = '/contacts'
export const CONTACTS_TITLE = 'Наші контакти'

/** Type names shown in the admin panel and on the page when a contact has no label. */
export const CONTACT_TYPE_LABELS: Record<ContactType, string> = {
  email: 'Email',
  telegram: 'Telegram',
  instagram: 'Instagram',
  threads: 'Threads',
  facebook: 'Facebook',
  phone: 'Телефон',
  website: 'Сайт',
}

/** Example values for the admin editor's value field. */
export const CONTACT_VALUE_PLACEHOLDERS: Record<ContactType, string> = {
  email: 'name@example.com',
  telegram: '@mandat або https://t.me/mandat',
  instagram: '@mandat або https://www.instagram.com/mandat',
  threads: '@mandat або https://www.threads.net/@mandat',
  facebook: 'mandat.ua або https://www.facebook.com/mandat.ua',
  phone: '+380 44 123 45 67',
  website: 'https://mandat.ua',
}

/** Admin breadcrumb and section label of the contacts management. */
export const CONTACTS_ADMIN_SECTION = 'Контакти'
