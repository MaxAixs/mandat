import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import type { ComponentType, ReactNode } from 'react'

import type { AdminNews, NewsFormValues } from '../src/types'

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

type ImageStatus = 'ready' | 'uploading' | 'error'

interface ImageItem {
  id: string
  value: string
  preview: string
  status: ImageStatus
  progress: number
  error: string | null
}

type ImageMoveDirection = 'up' | 'down'

interface NewsImagesFieldProps {
  items: ImageItem[]
  error?: string
  onAddFiles: (files: File[]) => void
  onAddLink: (url: string) => void
  onMove: (id: string, direction: ImageMoveDirection) => void
  onRemove: (id: string) => void
}

interface NewsEditorDialogProps {
  article?: AdminNews
  busy?: boolean
  error?: string | null
  onClose: () => void
  onSave: (values: NewsFormValues) => void
}

// src/components/admin/NewsImagesField.tsx and src/hooks/use-image-uploads.ts
// do not exist yet. Their module paths stay plain strings on purpose: literal
// imports would fail compilation (TS2307), while this keeps the suite compiling
// so RED is a behavioral module-not-found failure instead of a type error.
async function loadNewsImagesField(): Promise<ComponentType<NewsImagesFieldProps>> {
  const modulePath: string = '../src/components/admin/NewsImagesField'
  const fieldModule = (await import(modulePath)) as { default?: unknown }

  assert.equal(
    typeof fieldModule.default,
    'function',
    'NewsImagesField must default-export a component from src/components/admin/NewsImagesField',
  )
  return fieldModule.default as ComponentType<NewsImagesFieldProps>
}

async function loadNewsEditorDialog(): Promise<ComponentType<NewsEditorDialogProps>> {
  const editorModule = await import('../src/components/admin/NewsEditorDialog')

  return editorModule.default as ComponentType<NewsEditorDialogProps>
}

const IMAGE_A = 'https://cdn.example.com/images/a-11111111.jpg'
const IMAGE_B = 'https://cdn.example.com/images/a-22222222.png'

function imageItem(overrides: Partial<ImageItem> = {}): ImageItem {
  return {
    id: 'image-1',
    value: IMAGE_A,
    preview: IMAGE_A,
    status: 'ready',
    progress: 1,
    error: null,
    ...overrides,
  }
}

function fieldProps(overrides: Partial<NewsImagesFieldProps> = {}): NewsImagesFieldProps {
  return {
    items: [],
    onAddFiles: () => {},
    onAddLink: () => {},
    onMove: () => {},
    onRemove: () => {},
    ...overrides,
  }
}

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
    video: null,
    views: 10,
    published_at: '2026-10-25T11:35:00Z',
    created_at: '2026-10-24T09:00:00Z',
    updated_at: '2026-10-25T11:35:00Z',
    ...overrides,
  }
}

function makeItems(count: number): ImageItem[] {
  return Array.from({ length: count }, (_, index) => {
    const preview = `https://cdn.example.com/images/image-${index}.jpg`

    return imageItem({ id: `image-${index + 1}`, value: preview, preview })
  })
}

function attribute(attributes: string, name: string): string | undefined {
  const match = attributes.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

  return match?.[1]
}

function hasDisabled(attributes: string): boolean {
  return /(?:^|\s)disabled(?:=""|(?=\s|$))/.test(attributes)
}

interface ButtonNode {
  attributes: string
  text: string
}

function buttons(html: string): ButtonNode[] {
  return Array.from(
    html.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g),
    ([, attributes, content]) => ({
      attributes,
      text: content.replace(/<[^>]*>/g, '').trim(),
    }),
  )
}

function buttonByText(html: string, label: string): ButtonNode | undefined {
  return buttons(html).find((button) => button.text === label)
}

function buttonsByAriaLabel(html: string, label: string): ButtonNode[] {
  return buttons(html).filter((button) => attribute(button.attributes, 'aria-label') === label)
}

function inputs(html: string): string[] {
  return Array.from(html.matchAll(/<input\b([^>]*)>/g), ([, attributes]) => attributes)
}

function inputById(html: string, id: string): string | undefined {
  return inputs(html).find((attributes) => attribute(attributes, 'id') === id)
}

function images(html: string): string[] {
  return Array.from(html.matchAll(/<img\b([^>]*)>/g), ([, attributes]) => attributes)
}

function tagByRole(html: string, role: string): string | undefined {
  const match = html.match(new RegExp(`<[^>]*\\brole="${role}"[^>]*>`))

  return match?.[0]
}

let fieldPromise: Promise<ComponentType<NewsImagesFieldProps>> | undefined
let editorPromise: Promise<ComponentType<NewsEditorDialogProps>> | undefined

async function renderField(overrides: Partial<NewsImagesFieldProps> = {}): Promise<string> {
  fieldPromise ??= loadNewsImagesField()
  const NewsImagesField = await fieldPromise

  return renderToStaticMarkup(<NewsImagesField {...fieldProps(overrides)} />)
}

async function renderEditor(article?: AdminNews): Promise<string> {
  editorPromise ??= loadNewsEditorDialog()
  const NewsEditorDialog = await editorPromise

  return renderToStaticMarkup(
    <NewsEditorDialog article={article} onClose={() => {}} onSave={() => {}} />,
  )
}

test('NewsImagesField renders the empty counter', async () => {
  const html = await renderField()

  assert.ok(html.includes('0 / 10'), `expected the «0 / 10» counter in:\n${html}`)
})

test('NewsImagesField renders the counter for two items', async () => {
  const html = await renderField({
    items: [imageItem({ id: 'image-1' }), imageItem({ id: 'image-2', value: IMAGE_B, preview: IMAGE_B })],
  })

  assert.ok(html.includes('2 / 10'), `expected the «2 / 10» counter in:\n${html}`)
})

test('NewsImagesField offers a multi-file picker with the image types', async () => {
  const html = await renderField()
  const picker = inputById(html, 'news-images-file')

  assert.ok(picker, `expected an input#news-images-file picker in:\n${html}`)
  assert.equal(attribute(picker, 'type'), 'file')
  assert.ok(
    picker.includes('multiple'),
    `expected the picker to accept multiple files in:\n${html}`,
  )

  const accept = attribute(picker, 'accept') ?? ''

  assert.ok(accept.includes('image/jpeg'), `expected accept to allow jpeg in:\n${html}`)
  assert.ok(accept.includes('image/png'), `expected accept to allow png in:\n${html}`)
  assert.ok(accept.includes('image/webp'), `expected accept to allow webp in:\n${html}`)
  assert.ok(accept.includes('image/gif'), `expected accept to allow gif in:\n${html}`)
  assert.ok(
    html.includes('JPG, PNG, WebP або GIF, до 10 МБ'),
    `expected the format hint in:\n${html}`,
  )
})

test('NewsImagesField renders the link input with an add button', async () => {
  const html = await renderField()
  const link = inputById(html, 'news-image-url')

  assert.ok(link, `expected an input#news-image-url in:\n${html}`)
  assert.ok(
    (attribute(link, 'placeholder') ?? '').startsWith('https://'),
    `expected the placeholder to start with «https://» in:\n${html}`,
  )
  assert.ok(
    buttonByText(html, 'Додати'),
    `expected a «Додати» button in:\n${html}`,
  )
})

test('NewsImagesField renders thumbnails in order with indexed alt texts', async () => {
  const html = await renderField({
    items: [
      imageItem({ id: 'image-1', value: IMAGE_A, preview: IMAGE_A }),
      imageItem({ id: 'image-2', value: IMAGE_B, preview: IMAGE_B }),
    ],
  })
  const thumbnails = images(html)

  assert.equal(thumbnails.length, 2, `expected two thumbnails in:\n${html}`)
  assert.equal(attribute(thumbnails[0], 'src'), IMAGE_A)
  assert.equal(attribute(thumbnails[0], 'alt'), 'Фото 1')
  assert.equal(attribute(thumbnails[1], 'src'), IMAGE_B)
  assert.equal(attribute(thumbnails[1], 'alt'), 'Фото 2')
})

test('NewsImagesField renders reorder and remove buttons for each item', async () => {
  const html = await renderField({
    items: [
      imageItem({ id: 'image-1', value: IMAGE_A, preview: IMAGE_A }),
      imageItem({ id: 'image-2', value: IMAGE_B, preview: IMAGE_B }),
    ],
  })

  assert.equal(
    buttonsByAriaLabel(html, 'Перемістити вгору').length,
    2,
    `expected an «up» button per item in:\n${html}`,
  )
  assert.equal(
    buttonsByAriaLabel(html, 'Перемістити вниз').length,
    2,
    `expected a «down» button per item in:\n${html}`,
  )
  assert.equal(
    buttonsByAriaLabel(html, 'Видалити фото').length,
    2,
    `expected a remove button per item in:\n${html}`,
  )
})

test('NewsImagesField disables the up button of the first item and the down button of the last item', async () => {
  const html = await renderField({
    items: [
      imageItem({ id: 'image-1', value: IMAGE_A, preview: IMAGE_A }),
      imageItem({ id: 'image-2', value: IMAGE_B, preview: IMAGE_B }),
    ],
  })
  const upButtons = buttonsByAriaLabel(html, 'Перемістити вгору')
  const downButtons = buttonsByAriaLabel(html, 'Перемістити вниз')

  assert.ok(
    hasDisabled(upButtons[0].attributes),
    `expected the first «up» button to be disabled in:\n${html}`,
  )
  assert.ok(
    !hasDisabled(upButtons[1].attributes),
    `expected the last «up» button to stay enabled in:\n${html}`,
  )
  assert.ok(
    !hasDisabled(downButtons[0].attributes),
    `expected the first «down» button to stay enabled in:\n${html}`,
  )
  assert.ok(
    hasDisabled(downButtons[1].attributes),
    `expected the last «down» button to be disabled in:\n${html}`,
  )
})

test('NewsImagesField disables both move buttons for a single item', async () => {
  const html = await renderField({ items: [imageItem({ id: 'image-1' })] })
  const upButtons = buttonsByAriaLabel(html, 'Перемістити вгору')
  const downButtons = buttonsByAriaLabel(html, 'Перемістити вниз')

  assert.equal(upButtons.length, 1, `expected one «up» button in:\n${html}`)
  assert.equal(downButtons.length, 1, `expected one «down» button in:\n${html}`)
  assert.ok(
    hasDisabled(upButtons[0].attributes),
    `expected the single «up» button to be disabled in:\n${html}`,
  )
  assert.ok(
    hasDisabled(downButtons[0].attributes),
    `expected the single «down» button to be disabled in:\n${html}`,
  )
})

test('NewsImagesField shows upload progress of an uploading item', async () => {
  const html = await renderField({
    items: [imageItem({ id: 'image-1', status: 'uploading', progress: 0.4, error: null })],
  })
  const progressbar = tagByRole(html, 'progressbar')

  assert.ok(progressbar, `expected an element with role="progressbar" in:\n${html}`)
  assert.equal(attribute(progressbar, 'aria-valuenow'), '40')
  assert.ok(html.includes('40%'), `expected the 40% progress text in:\n${html}`)
})

test('NewsImagesField surfaces an item error as an alert', async () => {
  const html = await renderField({
    items: [imageItem({ id: 'image-1', status: 'error', error: 'Не вдалося завантажити фото.' })],
  })
  const alert = tagByRole(html, 'alert')

  assert.ok(alert, `expected an element with role="alert" in:\n${html}`)
  assert.ok(
    html.includes('Не вдалося завантажити фото.'),
    `expected the item error text in:\n${html}`,
  )
})

test('NewsImagesField hides the picker and the link input at the limit', async () => {
  const html = await renderField({ items: makeItems(10) })

  assert.ok(html.includes('10 / 10'), `expected the «10 / 10» counter in:\n${html}`)
  assert.ok(
    html.includes('Досягнуто ліміт 10 фото.'),
    `expected the limit text in:\n${html}`,
  )
  assert.ok(
    inputById(html, 'news-images-file') === undefined,
    `expected no file picker at the limit in:\n${html}`,
  )
  assert.ok(
    inputById(html, 'news-image-url') === undefined,
    `expected no link input at the limit in:\n${html}`,
  )
})

test('NewsImagesField renders the error prop text', async () => {
  const html = await renderField({ error: 'Додайте не більше 10 фото.' })

  assert.ok(
    html.includes('Додайте не більше 10 фото.'),
    `expected the error prop text in:\n${html}`,
  )
})

test('NewsEditorDialog renders a «ФОТО» section backed by the images field', async () => {
  const html = await renderEditor()

  assert.ok(html.includes('ФОТО'), `expected a «ФОТО» section in the editor:\n${html}`)
  assert.ok(
    inputById(html, 'news-images-file'),
    `expected the images field picker in a new article editor:\n${html}`,
  )
})

test('NewsEditorDialog previews the saved images in order on edit', async () => {
  const html = await renderEditor(makeArticle({ images: [IMAGE_A, IMAGE_B] }))
  const thumbnails = images(html).filter(
    (attributes) => attribute(attributes, 'src') === IMAGE_A || attribute(attributes, 'src') === IMAGE_B,
  )

  assert.equal(thumbnails.length, 2, `expected two saved image thumbnails in:\n${html}`)
  assert.equal(attribute(thumbnails[0], 'src'), IMAGE_A)
  assert.equal(attribute(thumbnails[1], 'src'), IMAGE_B)
  assert.ok(html.includes('2 / 10'), `expected the «2 / 10» counter in:\n${html}`)
})
