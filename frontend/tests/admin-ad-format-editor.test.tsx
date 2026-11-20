import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import type { ComponentType, ReactNode } from 'react'

import type { AdFormat } from '../src/types'
import type { AdFormatEditorDialogProps } from '../src/components/admin/AdFormatEditorDialog'

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
// behavior while making the editor form observable in static markup.
function PortalPassthrough({ children }: { children?: ReactNode }) {
  return <>{children}</>
}

moduleInternals._load = function loadWithInlineDialogPortal(request, ...rest) {
  const loaded = originalLoad.call(moduleInternals, request, ...rest)

  if (request === '@radix-ui/react-dialog') {
    return { ...(loaded as Record<string, unknown>), Portal: PortalPassthrough }
  }

  return loaded
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
  moduleInternals._load = originalLoad
})

async function loadDialog(): Promise<ComponentType<AdFormatEditorDialogProps>> {
  const dialogModule = await import('../src/components/admin/AdFormatEditorDialog')

  return dialogModule.default as ComponentType<AdFormatEditorDialogProps>
}

let dialogPromise: Promise<ComponentType<AdFormatEditorDialogProps>> | undefined

async function renderDialog(overrides: Partial<AdFormatEditorDialogProps> = {}): Promise<string> {
  dialogPromise ??= loadDialog()
  const AdFormatEditorDialog = await dialogPromise

  return renderToStaticMarkup(
    <AdFormatEditorDialog open onOpenChange={() => {}} onSubmit={() => {}} {...overrides} />,
  )
}

function makeFormat(overrides: Partial<AdFormat> = {}): AdFormat {
  return {
    id: 'format-1',
    title: 'Спецпроєкти',
    description: 'Окрема сторінка з власним дизайном',
    image_url: 'https://cdn.example.com/format.jpg',
    position: 1,
    ...overrides,
  }
}

function attribute(tag: string, name: string): string | undefined {
  const match = tag.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`, 'i'))

  return match?.[1]
}

function inputTags(html: string): string[] {
  return Array.from(html.matchAll(/<input\b[^>]*>/g), (match) => match[0])
}

interface TextareaElement {
  attributes: string
  content: string
}

function textareas(html: string): TextareaElement[] {
  return Array.from(html.matchAll(/<textarea\b([^>]*)>([\s\S]*?)<\/textarea>/g), (match) => ({
    attributes: match[1],
    content: match[2],
  }))
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

function imageTags(html: string): string[] {
  return Array.from(html.matchAll(/<img\b[^>]*>/g), (match) => match[0])
}

function submitButton(html: string): ButtonElement {
  const submit = buttons(html).find((button) => attribute(button.attributes, 'type') === 'submit')
  assert.ok(submit, `expected a submit button in the editor markup:\n${html}`)

  return submit
}

test('AdFormatEditorDialog shows an empty form with an image upload for a new card', async () => {
  const html = await renderDialog()

  assert.ok(html.includes('Новий формат'), `expected the «Новий формат» title in:\n${html}`)

  const titleInput = inputTags(html).find((tag) => attribute(tag, 'maxlength') === '80')
  assert.ok(titleInput, `expected a title input with maxLength=80 in:\n${html}`)
  assert.equal(attribute(titleInput, 'value') ?? '', '', `expected an empty title for a new card in:\n${html}`)

  const description = textareas(html).find((area) => attribute(area.attributes, 'maxlength') === '240')
  assert.ok(description, `expected a description textarea with maxLength=240 in:\n${html}`)
  assert.equal(description.content, '', `expected an empty description for a new card in:\n${html}`)

  const fileInput = inputTags(html).find((tag) => attribute(tag, 'type') === 'file')
  assert.ok(fileInput, `expected a file input in:\n${html}`)
  assert.equal(attribute(fileInput, 'accept'), 'image/*', `expected the file input to accept images in:\n${html}`)

  assert.ok(!html.includes('Прибрати зображення'), `a new card must not offer image removal in:\n${html}`)
})

test('AdFormatEditorDialog offers a Зберегти submit button for a new card', async () => {
  const html = await renderDialog()
  const submit = submitButton(html)

  assert.ok(submit.content.includes('Зберегти'), `expected a «Зберегти» submit button in:\n${html}`)
})

test('AdFormatEditorDialog prefills the form and offers image removal for a card with an image', async () => {
  const format = makeFormat()
  const html = await renderDialog({ format })

  const titleInput = inputTags(html).find((tag) => attribute(tag, 'maxlength') === '80')
  assert.ok(titleInput, `expected the title input in:\n${html}`)
  assert.equal(attribute(titleInput, 'value'), format.title, `expected the title to be prefilled in:\n${html}`)

  const description = textareas(html).find((area) => attribute(area.attributes, 'maxlength') === '240')
  assert.ok(description, `expected the description textarea in:\n${html}`)
  assert.equal(description.content, format.description, `expected the description to be prefilled in:\n${html}`)

  const preview = imageTags(html).find((tag) => attribute(tag, 'src') === format.image_url)
  assert.ok(preview, `expected an image preview with the card src in:\n${html}`)
  assert.ok(html.includes('Прибрати зображення'), `expected a «Прибрати зображення» button in:\n${html}`)
})

test('AdFormatEditorDialog hides the preview and removal for a card without an image', async () => {
  const html = await renderDialog({ format: makeFormat({ image_url: '' }) })

  assert.deepEqual(imageTags(html), [], `expected no image preview without an image_url in:\n${html}`)
  assert.ok(!html.includes('Прибрати зображення'), `expected no «Прибрати зображення» button without an image in:\n${html}`)
})

test('AdFormatEditorDialog limits the title and description length', async () => {
  const cases = [
    { field: 'title', pattern: /<input\b[^>]*maxlength="80"[^>]*>/i },
    { field: 'description', pattern: /<textarea\b[^>]*maxlength="240"[^>]*>/i },
  ]

  const html = await renderDialog()

  for (const { field, pattern } of cases) {
    assert.ok(pattern.test(html), `expected the ${field} field to carry its maxLength in:\n${html}`)
  }
})

test('AdFormatEditorDialog renders the error message', async () => {
  const html = await renderDialog({ error: 'Не вдалося зберегти формат.' })

  assert.ok(html.includes('Не вдалося зберегти формат.'), `expected the error text in:\n${html}`)
})

test('AdFormatEditorDialog disables the submit button while saving', async () => {
  const html = await renderDialog({ saving: true })
  const submit = submitButton(html)

  assert.ok(
    attribute(submit.attributes, 'disabled') !== undefined,
    `expected a disabled submit button while saving in:\n${html}`,
  )
})
