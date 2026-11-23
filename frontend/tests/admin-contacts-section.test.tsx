import { after, afterEach, before, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

import { AuthContext } from '../src/context/auth-context'
import type { AdminNews } from '../src/types'
import type { Contact } from '../src/types'

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
// The Admin page and the contacts panel load their rows through useEffect,
// which the server renderer never runs. Patching `react` per parent module
// keeps the real trees rendering while making the rows observable: injected
// arrays stand in for the answered APIs. Forcing the section state to
// 'contacts' renders the panel branch without clicking the nav (the runner
// has no DOM). A closed Radix select renders its items into a
// DocumentFragment through a portal, which the server renderer drops;
// exposing that fragment keeps the real option list observable. Only the
// select internals and the `react` imports of Admin/ContactsPanel are
// patched, so every other module keeps the real hooks.
// ---------------------------------------------------------------------------

let realUseState: ((initial: unknown) => unknown) | undefined
let articlesForRender: AdminNews[] = []
let contactsForRender: Contact[] | undefined
let forceContactsSection = false

function adminUseState(initial: unknown) {
  if (Array.isArray(initial)) {
    return [articlesForRender, () => {}]
  }
  if (initial === true) {
    return [false, () => {}]
  }
  if (forceContactsSection && initial === 'news') {
    return ['contacts', () => {}]
  }
  if (!realUseState) throw new Error('react useState was not captured for Admin')

  return realUseState(initial)
}

function panelUseState(initial: unknown) {
  if (contactsForRender !== undefined && Array.isArray(initial)) {
    return [contactsForRender, () => {}]
  }
  if (contactsForRender !== undefined && initial === true) {
    return [false, () => {}]
  }
  if (!realUseState) throw new Error('react useState was not captured for ContactsPanel')

  return realUseState(initial)
}

const selectDistSuffix = path.join('@radix-ui', 'react-select', 'dist', 'index.js')

moduleInternals._load = function loadWithSectionInitialState(request, parent, ...rest) {
  const loaded = originalLoad.call(moduleInternals, request, parent, ...rest) as Record<string, unknown>
  if (!realUseState && typeof loaded.useState === 'function') {
    realUseState = loaded.useState as (initial: unknown) => unknown
  }

  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''

  if (request === 'react-dom') {
    return { ...loaded, createPortal: (children: React.ReactNode) => children }
  }

  if (request === '@radix-ui/react-portal') {
    return { ...loaded, Portal: ({ children }: { children?: React.ReactNode }) => children }
  }

  if (request === 'react' && parentFilename.endsWith(selectDistSuffix)) {
    const selectUseState = loaded.useState as (initial?: unknown) => [unknown, (value: unknown) => void]

    return {
      ...loaded,
      // The closed-content branch keeps its DocumentFragment in a state
      // initialised without an argument; give it a value so the portal renders.
      useState: (initial?: unknown) =>
        initial === undefined ? [{}, () => undefined] : selectUseState(initial),
    }
  }

  if (request === 'react' && parentFilename.endsWith(path.join('pages', 'Admin.js'))) {
    return { ...loaded, useState: adminUseState }
  }

  if (request === 'react' && parentFilename.endsWith(path.join('admin', 'ContactsPanel.js'))) {
    return { ...loaded, useState: panelUseState }
  }

  return loaded
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  moduleInternals._load = originalLoad
})

afterEach(() => {
  articlesForRender = []
  contactsForRender = undefined
  forceContactsSection = false
})

// The subjects load lazily: compiled src modules keep the '@/...' alias, so
// they must be required after the resolver above is installed (static imports
// would run first and fail on the alias).
type ContactsConstants = typeof import('../src/constants/contacts')

let CONTACTS_ADMIN_SECTION!: string
let CONTACT_TYPE_LABELS!: ContactsConstants['CONTACT_TYPE_LABELS']
let Admin!: ComponentType
let ContactsPanel!: ComponentType

before(async () => {
  const contactsConstants: ContactsConstants = await import('../src/constants/contacts')
  CONTACTS_ADMIN_SECTION = contactsConstants.CONTACTS_ADMIN_SECTION
  CONTACT_TYPE_LABELS = contactsConstants.CONTACT_TYPE_LABELS
  ;({ default: Admin } = await import('../src/pages/Admin'))
  ;({ default: ContactsPanel } = await import('../src/components/admin/ContactsPanel'))
})

function makeArticle(overrides: Partial<AdminNews> = {}): AdminNews {
  return {
    id: 'news-1',
    slug: 'news-1',
    category: 'Новини',
    title: 'Тестова новина',
    description: 'Короткий опис новини',
    text: 'Повний текст новини',
    language: 'uk',
    status: 'published',
    featured: false,
    main_event: false,
    image_url: '',
    views: 10,
    published_at: '2026-10-25T11:35:00Z',
    created_at: '2026-10-24T09:00:00Z',
    updated_at: '2026-10-25T11:35:00Z',
    ...overrides,
  }
}

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

/** Display name of a row: the label when set, otherwise the type name. */
function contactName(contact: Contact): string {
  return contact.label || CONTACT_TYPE_LABELS[contact.type]
}

const authValue = {
  user: { id: 'staff-1', login: 'editor', role: 'admin' as const },
  loading: false,
  login: async () => {},
  logout: async () => {},
  clearUser: () => {},
}

function sectionNavs(html: string): string[] {
  const matches = html.matchAll(/<nav[^>]*aria-label="Розділи адмінпанелі"[^>]*>([\s\S]*?)<\/nav>/g)

  return Array.from(matches, (match) => match[1])
}

interface NavButton {
  openingTag: string
  label: string
}

function navButtons(navHtml: string): NavButton[] {
  const matches = navHtml.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g)

  return Array.from(matches, (match) => ({
    openingTag: match[1],
    label: visibleText(match[2]),
  }))
}

function visibleText(html: string): string {
  return html
    .replace(/<[^>]*>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/&#x27;/g, "'")
    .replace(/&quot;/g, '"')
    .replace(/&amp;/g, '&')
    .replace(/\s+/g, ' ')
    .trim()
    .toLocaleLowerCase('uk')
}

interface LabeledButton {
  label: string
  tag: string
}

function labeledButtons(html: string): LabeledButton[] {
  return Array.from(html.matchAll(/<button\b([^>]*)>/g), (match) => {
    const label = match[1].match(/aria-label="([^"]*)"/)?.[1] ?? ''

    return { label, tag: match[0] }
  }).filter((button) => button.label !== '')
}

function isDisabled(tag: string): boolean {
  return /\sdisabled(?:\s|=|>)/.test(tag)
}

async function renderAdmin(): Promise<string> {
  articlesForRender = [makeArticle()]

  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/admin']}>
      <AuthContext.Provider value={authValue}>
        <Admin />
      </AuthContext.Provider>
    </MemoryRouter>,
  )
}

function renderPanel(): string {
  return renderToStaticMarkup(<ContactsPanel />)
}

test('Admin page sections nav gains a third «Контакти» entry', async () => {
  const html = await renderAdmin()
  const navs = sectionNavs(html)

  assert.ok(
    navs.length >= 1,
    `Admin page: expected a <nav aria-label="Розділи адмінпанелі"> in:\n${html}`,
  )

  for (const nav of navs) {
    const buttons = navButtons(nav)
    const newsButton = buttons.find((button) => button.label.includes('керування новинами'))
    const adsButton = buttons.find((button) => button.label.includes('формати реклами'))
    const contactsButton = buttons.find((button) =>
      button.label.includes(CONTACTS_ADMIN_SECTION.toLocaleLowerCase('uk')),
    )

    assert.ok(newsButton, `Admin page: expected a «Керування новинами» button in the sections nav in:\n${nav}`)
    assert.ok(adsButton, `Admin page: expected a «Формати реклами» button in the sections nav in:\n${nav}`)
    assert.ok(
      contactsButton,
      `Admin page: expected a «${CONTACTS_ADMIN_SECTION}» button in the sections nav in:\n${nav}`,
    )
    assert.ok(
      contactsButton.openingTag.includes('type="button"'),
      `Admin page: expected the contacts section item to be <button type="button"> in:\n${nav}`,
    )
    assert.ok(
      newsButton.openingTag.includes('aria-current="page"'),
      `Admin page: expected the news button to carry aria-current="page" on the initial render in:\n${nav}`,
    )
    assert.ok(
      !contactsButton.openingTag.includes('aria-current'),
      `Admin page: expected the contacts button to carry no aria-current on the initial render in:\n${nav}`,
    )
  }
})

test("Admin contacts section renders ContactsPanel instead of the news table", async () => {
  forceContactsSection = true
  contactsForRender = []
  const html = await renderAdmin()

  assert.ok(
    html.includes('Керування контактами'),
    `Admin page: expected the «Керування контактами» panel heading for the contacts section in:\n${html}`,
  )
  assert.ok(
    html.includes('Додати контакт'),
    `Admin page: expected the «Додати контакт» button for the contacts section in:\n${html}`,
  )
  assert.ok(
    !html.includes('Додати новину'),
    `Admin page: expected no «Додати новину» button for the contacts section in:\n${html}`,
  )
})

test('ContactsPanel shows the management heading and add button on the first render', () => {
  const html = renderPanel()

  assert.ok(html.includes('Керування контактами'), `expected the «Керування контактами» title in:\n${html}`)
  assert.ok(html.includes('Додати контакт'), `expected the «Додати контакт» button in:\n${html}`)
})

test('ContactsPanel shows one row per contact with its display text', () => {
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
  ]
  contactsForRender = contacts
  const html = renderPanel()

  for (const contact of contacts) {
    const name = contactName(contact)

    assert.ok(html.includes(name), `expected the row name «${name}» in:\n${html}`)
    assert.ok(html.includes(contact.display), `expected the display «${contact.display}» in:\n${html}`)
  }
})

test('ContactsPanel move buttons carry directional labels with first-up and last-down disabled', () => {
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
  ]
  contactsForRender = contacts
  const html = renderPanel()
  const buttons = labeledButtons(html)
  const byLabel = new Map(buttons.map((button) => [button.label, button.tag]))

  for (const contact of contacts) {
    const name = contactName(contact)

    for (const direction of ['вище', 'нижче'] as const) {
      assert.ok(
        byLabel.has(`Перемістити «${name}» ${direction}`),
        `expected a «Перемістити «${name}» ${direction}» button in:\n${html}`,
      )
    }
  }

  const firstName = contactName(contacts[0]!)
  const lastName = contactName(contacts[contacts.length - 1]!)

  assert.ok(
    isDisabled(byLabel.get(`Перемістити «${firstName}» вище`) ?? ''),
    `expected the first row «вище» button to be disabled in:\n${html}`,
  )
  assert.ok(
    !isDisabled(byLabel.get(`Перемістити «${firstName}» нижче`) ?? ''),
    `expected the first row «нижче» button to be enabled in:\n${html}`,
  )
  assert.ok(
    !isDisabled(byLabel.get(`Перемістити «${lastName}» вище`) ?? ''),
    `expected the last row «вище» button to be enabled in:\n${html}`,
  )
  assert.ok(
    isDisabled(byLabel.get(`Перемістити «${lastName}» нижче`) ?? ''),
    `expected the last row «нижче» button to be disabled in:\n${html}`,
  )
})

test('ContactsPanel edit and delete buttons name their contact', () => {
  const contacts = [
    makeContact(),
    makeContact({ id: 'contact-2', type: 'email', label: '', display: 'mandat@gmail.com' }),
  ]
  contactsForRender = contacts
  const html = renderPanel()
  const buttons = labeledButtons(html)
  const labels = buttons.map((button) => button.label)

  for (const contact of contacts) {
    const name = contactName(contact)

    assert.ok(
      labels.includes(`Редагувати «${name}»`),
      `expected a «Редагувати «${name}»» button in [${labels.join(', ')}] in:\n${html}`,
    )
    assert.ok(
      labels.includes(`Видалити «${name}»`),
      `expected a «Видалити «${name}»» button in [${labels.join(', ')}] in:\n${html}`,
    )
  }
})

test('ContactsPanel shows the empty state without contacts', () => {
  contactsForRender = []
  const html = renderPanel()

  assert.ok(html.includes('Контактів ще немає'), `expected the «Контактів ще немає» empty state in:\n${html}`)
})

test('contacts admin section constant names the section «Контакти»', () => {
  assert.equal(CONTACTS_ADMIN_SECTION, 'Контакти', 'expected the admin section label')
})
