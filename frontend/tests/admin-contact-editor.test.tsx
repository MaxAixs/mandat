import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import type { ComponentType, ReactNode } from 'react'

import type { Contact, ContactType } from '../src/types'
import type { ContactEditorDialogProps } from '../src/components/admin/ContactEditorDialog'

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

// Radix renders dialog content through a portal, and the server renderer drops
// portals. Replacing only the Portal with a passthrough keeps the real dialog
// behavior while making the editor form observable in static markup. The type
// select is a Radix select: its closed content renders into a DocumentFragment
// through a portal, so the same react-dom/react-portal/select patching tricks
// as the admin advertising section suite apply here.
function PortalPassthrough({ children }: { children?: ReactNode }) {
  return <>{children}</>
}

const selectDistSuffix = path.join('@radix-ui', 'react-select', 'dist', 'index.js')

moduleInternals._load = function loadWithInlineDialogPortal(request, parent, ...rest) {
  const loaded = originalLoad.call(moduleInternals, request, parent, ...rest) as Record<string, unknown>
  const parentFilename = (parent as { filename?: string } | undefined)?.filename ?? ''

  if (request === 'react-dom') {
    return { ...loaded, createPortal: (children: ReactNode) => children }
  }

  if (request === '@radix-ui/react-portal') {
    return { ...loaded, Portal: PortalPassthrough }
  }

  if (request === '@radix-ui/react-dialog') {
    return { ...loaded, Portal: PortalPassthrough }
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

  return loaded
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  moduleInternals._load = originalLoad
})

async function loadDialog(): Promise<ComponentType<ContactEditorDialogProps>> {
  const dialogModule = await import('../src/components/admin/ContactEditorDialog')

  return dialogModule.default as ComponentType<ContactEditorDialogProps>
}

let dialogPromise: Promise<ComponentType<ContactEditorDialogProps>> | undefined

async function renderDialog(overrides: Partial<ContactEditorDialogProps> = {}): Promise<string> {
  dialogPromise ??= loadDialog()
  const ContactEditorDialog = await dialogPromise

  return renderToStaticMarkup(
    <ContactEditorDialog open onOpenChange={() => {}} onSubmit={() => {}} {...overrides} />,
  )
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

const CONTACT_TYPE_LABELS: Record<ContactType, string> = {
  email: 'Email',
  telegram: 'Telegram',
  instagram: 'Instagram',
  threads: 'Threads',
  facebook: 'Facebook',
  phone: 'Телефон',
  website: 'Сайт',
}

const CONTACT_VALUE_PLACEHOLDERS: Record<ContactType, string> = {
  email: 'name@example.com',
  telegram: '@mandat або https://t.me/mandat',
  instagram: '@mandat або https://www.instagram.com/mandat',
  threads: '@mandat або https://www.threads.net/@mandat',
  facebook: 'mandat.ua або https://www.facebook.com/mandat.ua',
  phone: '+380 44 123 45 67',
  website: 'https://mandat.ua',
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

function attribute(tag: string, name: string): string | undefined {
  const match = tag.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`, 'i'))

  return match?.[1]
}

function inputTags(html: string): string[] {
  return Array.from(html.matchAll(/<input\b[^>]*>/g), (match) => match[0])
}

function inputById(html: string, id: string): string | undefined {
  return inputTags(html).find((tag) => attribute(tag, 'id') === id)
}

/**
 * Labels of the type select options: native <option> elements and Radix
 * SelectItem entries (role="option") alike, so the assertion holds for the
 * shadcn select the admin editors use.
 */
function selectOptionLabels(html: string): string[] {
  const native = Array.from(html.matchAll(/<option\b[^>]*>([\s\S]*?)<\/option>/g), (match) =>
    visibleText(match[1]),
  )
  const radix = Array.from(html.matchAll(/<div[^>]*\brole="option"[^>]*>([\s\S]*?)<\/div>/g), (match) =>
    visibleText(match[1]),
  )

  return [...native, ...radix]
}

/** Visible text of the closed type select (the Radix combobox trigger). */
function selectTriggerText(html: string): string | undefined {
  const triggers = Array.from(html.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g), (entry) => ({
    attributes: entry[1],
    content: entry[2],
  })).filter((button) => attribute(button.attributes, 'role') === 'combobox')

  return triggers.length > 0 ? visibleText(triggers[0]?.content ?? '') : undefined
}

function formTags(html: string): string[] {
  return Array.from(html.matchAll(/<form\b[^>]*>/g), (match) => match[0])
}

interface ButtonElement {
  attributes: string
  content: string
}

function buttons(html: string): ButtonElement[] {
  return Array.from(html.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g), (match) => ({
    attributes: match[1],
    content: match[2],
  }))
}

function submitButton(html: string): ButtonElement {
  const submit = buttons(html).find((button) => attribute(button.attributes, 'type') === 'submit')
  assert.ok(submit, `expected a submit button in the editor markup:\n${html}`)

  return submit
}

test('ContactEditorDialog shows an empty form with the email defaults for a new contact', async () => {
  const html = await renderDialog()

  assert.ok(html.includes('Новий контакт'), `expected the «Новий контакт» title in:\n${html}`)

  const valueInput = inputById(html, 'contact-value')
  assert.ok(valueInput, `expected a value input with id="contact-value" in:\n${html}`)
  assert.equal(attribute(valueInput, 'value') ?? '', '', `expected an empty value for a new contact in:\n${html}`)
  assert.equal(
    attribute(valueInput, 'placeholder'),
    CONTACT_VALUE_PLACEHOLDERS.email,
    `expected the email placeholder for a new contact in:\n${html}`,
  )

  const labelInput = inputById(html, 'contact-label')
  assert.ok(labelInput, `expected a label input with id="contact-label" in:\n${html}`)
  assert.equal(attribute(labelInput, 'value') ?? '', '', `expected an empty label for a new contact in:\n${html}`)
  assert.equal(attribute(labelInput, 'maxlength') ?? attribute(labelInput, 'maxLength'), '40', `expected maxLength=40 on the label input in:\n${html}`)

  assert.equal(
    selectTriggerText(html),
    CONTACT_TYPE_LABELS.email.toLocaleLowerCase('uk'),
    `expected the type select to default to «${CONTACT_TYPE_LABELS.email}» in:\n${html}`,
  )
})

test('ContactEditorDialog offers every contact type in the type select', async () => {
  const html = await renderDialog()
  const options = selectOptionLabels(html)
  const expected = Object.values(CONTACT_TYPE_LABELS).map((label) => label.toLocaleLowerCase('uk'))

  assert.ok(options.length >= 7, `expected the seven type options in the editor markup:\n${html}`)

  for (const label of expected) {
    assert.ok(
      options.includes(label),
      `expected the type option «${label}» in [${options.join(', ')}] in:\n${html}`,
    )
  }
})

test('ContactEditorDialog prefills the form and the type for the contact being edited', async () => {
  const cases: Array<{ name: string; contact: Contact }> = [
    {
      name: 'telegram',
      contact: makeContact({ type: 'telegram', value: 'mandat', label: 'Підтримка' }),
    },
    {
      name: 'phone',
      contact: makeContact({
        id: 'contact-2',
        type: 'phone',
        value: '+380 44 123 45 67',
        label: '',
        display: '+380 44 123 45 67',
        href: 'tel:+380441234567',
        position: 2,
      }),
    },
  ]

  for (const { name, contact } of cases) {
    const html = await renderDialog({ contact })

    assert.ok(html.includes('Редагувати контакт'), `${name}: expected the «Редагувати контакт» title in:\n${html}`)

    const valueInput = inputById(html, 'contact-value')
    assert.ok(valueInput, `${name}: expected the value input in:\n${html}`)
    assert.equal(
      attribute(valueInput, 'value'),
      contact.value,
      `${name}: expected the value to be prefilled in:\n${html}`,
    )
    assert.equal(
      attribute(valueInput, 'placeholder'),
      CONTACT_VALUE_PLACEHOLDERS[contact.type],
      `${name}: expected the ${contact.type} placeholder in:\n${html}`,
    )

    const labelInput = inputById(html, 'contact-label')
    assert.ok(labelInput, `${name}: expected the label input in:\n${html}`)
    assert.equal(
      attribute(labelInput, 'value') ?? '',
      contact.label,
      `${name}: expected the label to be prefilled in:\n${html}`,
    )

    assert.equal(
      selectTriggerText(html),
      CONTACT_TYPE_LABELS[contact.type].toLocaleLowerCase('uk'),
      `${name}: expected the type select to show «${CONTACT_TYPE_LABELS[contact.type]}» in:\n${html}`,
    )
  }
})

test('ContactEditorDialog limits the label length', async () => {
  const html = await renderDialog()
  const labelInput = inputById(html, 'contact-label')

  assert.ok(labelInput, `expected the label input in:\n${html}`)
  assert.ok(
    /maxlength="40"/i.test(labelInput),
    `expected the label input to carry maxLength=40 in:\n${html}`,
  )
})

test('ContactEditorDialog wraps the fields in a form with a submit button', async () => {
  const html = await renderDialog()

  // Submitting the form calls onSubmit with the trimmed {type, value, label}
  // and shows «Вкажіть значення контакту.» without calling onSubmit when the
  // value is empty. Static markup cannot click submit (the repo has no DOM
  // runner: interactive behavior is verified manually), so this suite pins
  // the observable precondition of both flows: the fields live in a <form>
  // with a submit button.
  assert.ok(formTags(html).length >= 1, `expected the fields wrapped in a <form> for submitting the values in:\n${html}`)
  submitButton(html)
})

test('ContactEditorDialog renders the error message', async () => {
  const html = await renderDialog({ error: 'Не вдалося зберегти контакт.' })

  assert.ok(html.includes('Не вдалося зберегти контакт.'), `expected the error text in:\n${html}`)
})

test('ContactEditorDialog disables the submit button while saving', async () => {
  const html = await renderDialog({ saving: true })
  const submit = submitButton(html)

  assert.ok(
    attribute(submit.attributes, 'disabled') !== undefined,
    `expected a disabled submit button while saving in:\n${html}`,
  )
})
