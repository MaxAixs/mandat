import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

import type { Contact, ContactType } from '../src/types'

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the other frontend suites do.
interface ModuleInternals {
  _resolveFilename: (
    request: string,
    parent?: unknown,
    isMain?: boolean,
    options?: unknown,
  ) => string
  _load: (request: string, parent?: unknown, isMain?: boolean) => unknown
}

const moduleInternals = Module as unknown as ModuleInternals
const originalResolveFilename = moduleInternals._resolveFilename
const originalLoad = moduleInternals._load
const compiledSrcRoot = path.resolve(__dirname, '..', 'src')

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

// ---------------------------------------------------------------------------
// The Contacts page loads its rows through useEffect, which the server
// renderer never runs. Patching `react` only for the page module keeps the
// real page tree rendering while making the loaded contacts observable:
// an injected array stands in for the answered API. Every other module keeps
// the real hooks.
// ---------------------------------------------------------------------------

let realUseState: ((initial: unknown) => unknown) | undefined
let pageContacts: Contact[] | undefined

function pageUseState(initial: unknown) {
  if (pageContacts !== undefined && (Array.isArray(initial) || initial === null)) {
    return [pageContacts, () => {}]
  }
  if (!realUseState) throw new Error('react useState was not captured for the Contacts page')

  return realUseState(initial)
}

moduleInternals._load = function loadWithPageContacts(request, parent, ...rest) {
  const loaded = originalLoad.call(moduleInternals, request, parent, ...rest) as Record<string, unknown>
  if (!realUseState && typeof loaded.useState === 'function') {
    realUseState = loaded.useState as (initial: unknown) => unknown
  }

  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''
  if (request === 'react' && parentFilename.endsWith(path.join('pages', 'Contacts.js'))) {
    return { ...loaded, useState: pageUseState }
  }

  return loaded
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  moduleInternals._load = originalLoad
})

afterEach(() => {
  pageContacts = undefined
})

// The subjects load lazily: compiled src modules keep the '@/...' alias, so
// they must be required after the resolver above is installed (static imports
// would run first and fail on the alias).
type ContactsConstants = typeof import('../src/constants/contacts')

interface ContactListProps {
  contacts: Contact[]
}

let CONTACT_TYPE_LABELS!: ContactsConstants['CONTACT_TYPE_LABELS']
let CONTACT_VALUE_PLACEHOLDERS!: ContactsConstants['CONTACT_VALUE_PLACEHOLDERS']
let CONTACTS_TITLE!: string
let ContactList!: ComponentType<ContactListProps>
let Contacts!: ComponentType

before(async () => {
  const contactsConstants: ContactsConstants = await import('../src/constants/contacts')
  CONTACT_TYPE_LABELS = contactsConstants.CONTACT_TYPE_LABELS
  CONTACT_VALUE_PLACEHOLDERS = contactsConstants.CONTACT_VALUE_PLACEHOLDERS
  CONTACTS_TITLE = contactsConstants.CONTACTS_TITLE
  ;({ default: ContactList } = await import('../src/components/ContactList'))
  ;({ default: Contacts } = await import('../src/pages/Contacts'))
})

function makeContact(overrides: Partial<Contact> = {}): Contact {
  return {
    id: 'contact-1',
    type: 'telegram',
    value: 'mandat',
    label: 'Підтримка',
    display: '@mandat',
    href: 'https://t.me/mandat',
    position: 1,
    ...overrides,
  }
}

function attribute(tag: string, name: string): string | undefined {
  const match = tag.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`, 'i'))

  return match?.[1]
}

function anchorTags(html: string): string[] {
  return Array.from(html.matchAll(/<a\b[^>]*>/g), (match) => match[0])
}

function anchorLinks(html: string): Array<{ href: string; tag: string }> {
  return anchorTags(html).map((tag) => ({ href: attribute(tag, 'href') ?? '', tag }))
}

function renderList(contacts: Contact[]): string {
  return renderToStaticMarkup(<ContactList contacts={contacts} />)
}

function renderPage(): string {
  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/contacts']}>
      <Contacts />
    </MemoryRouter>,
  )
}

test('ContactList renders one link per contact with the server href', () => {
  const contacts = [
    makeContact(),
    makeContact({
      id: 'contact-2',
      type: 'email',
      value: 'mandat@gmail.com',
      label: '',
      display: 'mandat@gmail.com',
      href: 'mailto:mandat@gmail.com',
      position: 2,
    }),
    makeContact({
      id: 'contact-3',
      type: 'phone',
      value: '+380 44 123 45 67',
      label: '',
      display: '+380 44 123 45 67',
      href: 'tel:+380441234567',
      position: 3,
    }),
  ]
  const html = renderList(contacts)
  const links = anchorLinks(html)

  assert.equal(links.length, contacts.length, `expected one link per contact in:\n${html}`)
  assert.deepEqual(
    links.map((link) => link.href),
    contacts.map((contact) => contact.href),
    `expected the server hrefs on the links in:\n${html}`,
  )
})

test('ContactList marks only http(s) links as external', () => {
  const contacts = [
    makeContact({ href: 'https://t.me/mandat' }),
    makeContact({ id: 'contact-2', type: 'email', href: 'mailto:mandat@gmail.com' }),
    makeContact({ id: 'contact-3', type: 'phone', href: 'tel:+380441234567' }),
  ]
  const html = renderList(contacts)
  const links = anchorLinks(html)

  assert.equal(links.length, 3, `expected the three contact links in:\n${html}`)

  for (const link of links) {
    const external = link.href.startsWith('http')

    if (external) {
      assert.equal(attribute(link.tag, 'target'), '_blank', `expected target="_blank" for ${link.href} in:\n${html}`)
      assert.equal(
        attribute(link.tag, 'rel'),
        'noopener noreferrer',
        `expected rel="noopener noreferrer" for ${link.href} in:\n${html}`,
      )
    } else {
      assert.equal(attribute(link.tag, 'target'), undefined, `expected no target for ${link.href} in:\n${html}`)
      assert.equal(attribute(link.tag, 'rel'), undefined, `expected no rel for ${link.href} in:\n${html}`)
    }
  }
})

test('ContactList shows the label or the type name with the display text', () => {
  const cases: Array<{ contact: Contact; expectedName: ContactType | string; unexpectedName?: string }> = [
    {
      contact: makeContact({ type: 'telegram', label: 'Підтримка' }),
      expectedName: 'Підтримка',
      unexpectedName: CONTACT_TYPE_LABELS.telegram,
    },
    {
      contact: makeContact({ id: 'contact-2', type: 'email', label: '', display: 'mandat@gmail.com' }),
      expectedName: CONTACT_TYPE_LABELS.email,
    },
  ]

  for (const { contact, expectedName, unexpectedName } of cases) {
    const html = renderList([contact])

    assert.ok(
      html.includes(expectedName),
      `expected «${expectedName}» for the ${contact.type} contact in:\n${html}`,
    )
    assert.ok(html.includes(contact.display), `expected the display «${contact.display}» in:\n${html}`)
    if (unexpectedName !== undefined && unexpectedName !== expectedName) {
      assert.ok(
        !html.includes(unexpectedName),
        `expected no type name «${unexpectedName}» next to the label in:\n${html}`,
      )
    }
  }
})

test('ContactList renders no links for an empty list', () => {
  const html = renderList([])

  assert.deepEqual(anchorTags(html), [], `expected no links for an empty contact list in:\n${html}`)
})

test('contacts constants expose the type labels and value placeholders', () => {
  assert.deepEqual(
    CONTACT_TYPE_LABELS,
    {
      email: 'Email',
      telegram: 'Telegram',
      instagram: 'Instagram',
      threads: 'Threads',
      facebook: 'Facebook',
      phone: 'Телефон',
      website: 'Сайт',
    },
    'expected the seven contact type labels',
  )
  assert.deepEqual(
    CONTACT_VALUE_PLACEHOLDERS,
    {
      email: 'name@example.com',
      telegram: '@mandat або https://t.me/mandat',
      instagram: '@mandat або https://www.instagram.com/mandat',
      threads: '@mandat або https://www.threads.net/@mandat',
      facebook: 'mandat.ua або https://www.facebook.com/mandat.ua',
      phone: '+380 44 123 45 67',
      website: 'https://mandat.ua',
    },
    'expected the per-type value placeholders',
  )
  assert.equal(CONTACTS_TITLE, 'Наші контакти', 'expected the contacts page title')
})

test('contacts constants no longer export the hardcoded CONTACTS list', async () => {
  const contactsModule = (await import('../src/constants/contacts')) as unknown as Record<string, unknown>

  assert.equal(contactsModule['CONTACTS'], undefined, 'expected the CONTACTS constant to be removed; the page reads /api/contacts')
})

test('Contacts page shows the «Наші контакти» heading', () => {
  const html = renderPage()

  assert.ok(html.includes('Наші контакти'), `expected the «Наші контакти» heading in:\n${html}`)
})

test('Contacts page passes the loaded contacts to ContactList', () => {
  pageContacts = [
    makeContact({ id: 'rc-1', value: 'rc-test-handle', label: '', display: '@rc-test-handle', href: 'https://t.me/rc-test-handle' }),
    makeContact({
      id: 'rc-2',
      type: 'email',
      value: 'rc-test@example.com',
      label: 'Редакція',
      display: 'rc-test@example.com',
      href: 'mailto:rc-test@example.com',
      position: 2,
    }),
  ]
  const html = renderPage()
  const links = anchorLinks(html)
  const hrefs = links.map((link) => link.href)

  assert.ok(hrefs.includes('https://t.me/rc-test-handle'), `expected the telegram href in:\n${html}`)
  assert.ok(hrefs.includes('mailto:rc-test@example.com'), `expected the email href in:\n${html}`)
  assert.ok(html.includes('@rc-test-handle'), `expected the telegram display in:\n${html}`)
  assert.ok(html.includes('Редакція'), `expected the contact label in:\n${html}`)
})

test('Contacts page shows no contact links before the API answers', () => {
  const html = renderPage()

  assert.ok(
    !html.includes('https://t.me/rc-test-handle') && !html.includes('@rc-test-handle'),
    `expected no contact links before the API answers in:\n${html}`,
  )
})
