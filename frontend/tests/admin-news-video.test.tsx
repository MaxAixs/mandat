import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import type { ComponentType, ReactNode } from 'react'

import type { AdminNews, NewsFormValues, NewsVideo } from '../src/types'

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

type VideoUploadStatus = 'idle' | 'uploading' | 'done' | 'error'

interface VideoUploadState {
  status: VideoUploadStatus
  progress: number
  fileName: string
  error: string | null
}

interface NewsVideoFieldProps {
  videoKey: string
  videoUrl: string
  currentVideo: NewsVideo | null
  upload: VideoUploadState
  error?: string
  onSelectFile: (file: File) => void
  onCancelUpload: () => void
  onLinkChange: (url: string) => void
  onRemove: () => void
}

interface NewsEditorDialogProps {
  article?: AdminNews
  busy?: boolean
  error?: string | null
  onClose: () => void
  onSave: (values: NewsFormValues) => void
}

// src/components/admin/NewsVideoField.tsx and src/hooks/use-video-upload.ts do
// not exist yet. Their module paths stay plain strings on purpose: literal
// imports would fail compilation (TS2307), while this keeps the suite compiling
// so RED is a behavioral module-not-found failure instead of a type error.
async function loadNewsVideoField(): Promise<ComponentType<NewsVideoFieldProps>> {
  const modulePath: string = '../src/components/admin/NewsVideoField'
  const fieldModule = (await import(modulePath)) as { default?: unknown }

  assert.equal(
    typeof fieldModule.default,
    'function',
    'NewsVideoField must default-export a component from src/components/admin/NewsVideoField',
  )
  return fieldModule.default as ComponentType<NewsVideoFieldProps>
}

async function loadVideoUploadModule(): Promise<{ IDLE_VIDEO_UPLOAD: VideoUploadState }> {
  const modulePath: string = '../src/hooks/use-video-upload'
  const mod = (await import(modulePath)) as { IDLE_VIDEO_UPLOAD?: unknown }

  assert.ok(
    mod.IDLE_VIDEO_UPLOAD,
    'IDLE_VIDEO_UPLOAD must be exported from src/hooks/use-video-upload',
  )
  return mod as { IDLE_VIDEO_UPLOAD: VideoUploadState }
}

async function loadNewsEditorDialog(): Promise<ComponentType<NewsEditorDialogProps>> {
  const editorModule = await import('../src/components/admin/NewsEditorDialog')

  return editorModule.default as ComponentType<NewsEditorDialogProps>
}

const FILE_VIDEO_URL = 'https://cdn.example.com/videos/news-1-a1b2c3d4.mp4'
const EMBED_VIDEO_URL = 'https://www.youtube-nocookie.com/embed/AbC123dEfG_'
const YOUTUBE_WATCH_URL = 'https://www.youtube.com/watch?v=AbC123dEfG_'

const IDLE_UPLOAD: VideoUploadState = { status: 'idle', progress: 0, fileName: '', error: null }

function fieldProps(overrides: Partial<NewsVideoFieldProps> = {}): NewsVideoFieldProps {
  return {
    videoKey: '',
    videoUrl: '',
    currentVideo: null,
    upload: IDLE_UPLOAD,
    onSelectFile: () => {},
    onCancelUpload: () => {},
    onLinkChange: () => {},
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

function attribute(attributes: string, name: string): string | undefined {
  const match = attributes.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

  return match?.[1]
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

function pressedState(html: string, label: string): string | undefined {
  const button = buttonByText(html, label)
  if (!button) return undefined

  return attribute(button.attributes, 'aria-pressed')
}

function inputs(html: string): string[] {
  return Array.from(html.matchAll(/<input\b([^>]*)>/g), ([, attributes]) => attributes)
}

function inputById(html: string, id: string): string | undefined {
  return inputs(html).find((attributes) => attribute(attributes, 'id') === id)
}

function tagByRole(html: string, role: string): string | undefined {
  const match = html.match(new RegExp(`<[^>]*\\brole="${role}"[^>]*>`))

  return match?.[0]
}

function videoTag(html: string): string | undefined {
  const match = html.match(/<video\b([^>]*)>/)

  return match?.[1]
}

let fieldPromise: Promise<ComponentType<NewsVideoFieldProps>> | undefined
let editorPromise: Promise<ComponentType<NewsEditorDialogProps>> | undefined

async function renderField(overrides: Partial<NewsVideoFieldProps> = {}): Promise<string> {
  fieldPromise ??= loadNewsVideoField()
  const NewsVideoField = await fieldPromise

  return renderToStaticMarkup(<NewsVideoField {...fieldProps(overrides)} />)
}

async function renderEditor(article?: AdminNews): Promise<string> {
  editorPromise ??= loadNewsEditorDialog()
  const NewsEditorDialog = await editorPromise

  return renderToStaticMarkup(
    <NewsEditorDialog article={article} onClose={() => {}} onSave={() => {}} />,
  )
}

test('IDLE_VIDEO_UPLOAD is the idle video upload state', async () => {
  const { IDLE_VIDEO_UPLOAD } = await loadVideoUploadModule()

  assert.deepEqual(IDLE_VIDEO_UPLOAD, {
    status: 'idle',
    progress: 0,
    fileName: '',
    error: null,
  })
})

test('NewsVideoField renders the «Файл» and «Посилання» switch with aria-pressed', async () => {
  const html = await renderField()

  assert.equal(pressedState(html, 'Файл'), 'true', `expected a pressed «Файл» button in:\n${html}`)
  assert.equal(
    pressedState(html, 'Посилання'),
    'false',
    `expected an unpressed «Посилання» button in:\n${html}`,
  )
})

const modeCases: Array<{
  name: string
  props: Partial<NewsVideoFieldProps>
  expected: 'Файл' | 'Посилання'
}> = [
  {
    name: 'starts in file mode for a new article',
    props: {},
    expected: 'Файл',
  },
  {
    name: 'starts in link mode for an embed video',
    props: { currentVideo: { kind: 'embed', url: EMBED_VIDEO_URL } },
    expected: 'Посилання',
  },
  {
    name: 'starts in link mode for a pasted link that differs from the file url',
    props: { currentVideo: { kind: 'file', url: FILE_VIDEO_URL }, videoUrl: EMBED_VIDEO_URL },
    expected: 'Посилання',
  },
  {
    name: 'stays in file mode when the url is the current file url',
    props: { currentVideo: { kind: 'file', url: FILE_VIDEO_URL }, videoUrl: FILE_VIDEO_URL },
    expected: 'Файл',
  },
  {
    name: 'stays in file mode for a file video without a pending link',
    props: { currentVideo: { kind: 'file', url: FILE_VIDEO_URL }, videoUrl: '' },
    expected: 'Файл',
  },
]

for (const { name, props, expected } of modeCases) {
  test(`NewsVideoField ${name}`, async () => {
    const html = await renderField(props)
    const other = expected === 'Файл' ? 'Посилання' : 'Файл'

    assert.equal(
      pressedState(html, expected),
      'true',
      `${name}: expected «${expected}» to be pressed in:\n${html}`,
    )
    assert.equal(
      pressedState(html, other),
      'false',
      `${name}: expected «${other}» to be unpressed in:\n${html}`,
    )
  })
}

test('NewsVideoField shows upload progress with a cancel button and no file input', async () => {
  const html = await renderField({
    upload: { status: 'uploading', progress: 0.4, fileName: 'clip.mp4', error: null },
  })

  assert.ok(html.includes('40%'), `expected the 40% progress text in:\n${html}`)

  const progressbar = tagByRole(html, 'progressbar')

  assert.ok(progressbar, `expected an element with role="progressbar" in:\n${html}`)
  assert.equal(attribute(progressbar, 'aria-valuenow'), '40')
  assert.ok(
    buttonByText(html, 'Скасувати завантаження'),
    `expected a «Скасувати завантаження» button in:\n${html}`,
  )
  assert.ok(
    !html.includes('type="file"'),
    `expected no file input while uploading in:\n${html}`,
  )
})

test('NewsVideoField shows a finished upload with a remove button', async () => {
  const html = await renderField({
    videoKey: 'pending/videos/0123456789abcdef0123456789abcdef.mp4',
    upload: { status: 'done', progress: 1, fileName: 'clip.mp4', error: null },
  })

  assert.ok(
    html.includes('Відео завантажено: clip.mp4'),
    `expected the uploaded file name in:\n${html}`,
  )
  assert.ok(
    buttonByText(html, 'Видалити відео'),
    `expected a «Видалити відео» button in:\n${html}`,
  )
})

test('NewsVideoField keeps the current file video with a preview', async () => {
  const html = await renderField({
    currentVideo: { kind: 'file', url: FILE_VIDEO_URL },
    videoUrl: FILE_VIDEO_URL,
  })
  const video = videoTag(html)

  assert.ok(video, `expected a <video> preview of the current file in:\n${html}`)
  assert.equal(attribute(video, 'src'), FILE_VIDEO_URL)
  assert.ok(video.includes('controls'), `expected the preview to have controls in:\n${html}`)
  assert.ok(
    buttonByText(html, 'Видалити відео'),
    `expected a «Видалити відео» button in:\n${html}`,
  )
})

test('NewsVideoField offers a file picker with the format hint otherwise', async () => {
  const html = await renderField()
  const picker = inputById(html, 'news-video-file')

  assert.ok(picker, `expected an input#news-video-file picker in:\n${html}`)
  assert.equal(attribute(picker, 'type'), 'file')

  const accept = attribute(picker, 'accept') ?? ''

  assert.ok(accept.includes('video/mp4'), `expected accept to allow mp4 in:\n${html}`)
  assert.ok(accept.includes('video/webm'), `expected accept to allow webm in:\n${html}`)
  assert.ok(
    html.includes('MP4 або WebM, до 500 МБ'),
    `expected the format hint in:\n${html}`,
  )
})

test('NewsVideoField surfaces an upload error as an alert', async () => {
  const html = await renderField({
    upload: { status: 'error', progress: 0, fileName: '', error: 'Не вдалося завантажити відео.' },
  })
  const alert = tagByRole(html, 'alert')

  assert.ok(alert, `expected an element with role="alert" in:\n${html}`)
  assert.ok(
    html.includes('Не вдалося завантажити відео.'),
    `expected the upload error text in:\n${html}`,
  )
})

test('NewsVideoField renders the link input with its hints in link mode', async () => {
  const html = await renderField({ videoUrl: YOUTUBE_WATCH_URL })
  const link = inputById(html, 'news-video-url')

  assert.ok(link, `expected an input#news-video-url in link mode in:\n${html}`)
  assert.equal(attribute(link, 'value'), YOUTUBE_WATCH_URL)
  assert.ok(
    (attribute(link, 'placeholder') ?? '').includes('youtube.com'),
    `expected the placeholder to mention youtube.com in:\n${html}`,
  )
  assert.ok(
    html.includes('YouTube або Vimeo'),
    `expected the link hint to mention YouTube або Vimeo in:\n${html}`,
  )
  assert.ok(
    buttonByText(html, 'Видалити відео'),
    `expected a «Видалити відео» button for a non-empty link in:\n${html}`,
  )
})

test('NewsVideoField starts the link input empty without a link', async () => {
  const html = await renderField({
    currentVideo: { kind: 'embed', url: EMBED_VIDEO_URL },
    videoUrl: '',
  })
  const link = inputById(html, 'news-video-url')

  assert.ok(link, `expected an input#news-video-url in link mode in:\n${html}`)
  assert.equal(attribute(link, 'value'), '')
  assert.ok(
    !buttonByText(html, 'Видалити відео'),
    `expected no «Видалити відео» button for an empty link in:\n${html}`,
  )
})

test('NewsVideoField never exposes the current file url as a link value', async () => {
  const html = await renderField({
    currentVideo: { kind: 'file', url: FILE_VIDEO_URL },
    videoUrl: FILE_VIDEO_URL,
  })
  const link = inputById(html, 'news-video-url')

  assert.ok(
    link === undefined || attribute(link, 'value') !== FILE_VIDEO_URL,
    `expected the file url to stay out of the link input in:\n${html}`,
  )
})

test('NewsVideoField renders the error prop text', async () => {
  const html = await renderField({ error: 'Відео не може бути більшим за 500 МБ.' })

  assert.ok(
    html.includes('Відео не може бути більшим за 500 МБ.'),
    `expected the error prop text in:\n${html}`,
  )
})

test('NewsEditorDialog renders a «ВІДЕО» section backed by the video field', async () => {
  const html = await renderEditor()

  assert.ok(html.includes('ВІДЕО'), `expected a «ВІДЕО» section in the editor:\n${html}`)
  assert.ok(
    inputById(html, 'news-video-file'),
    `expected the video field picker in a new article editor:\n${html}`,
  )
})

test('NewsEditorDialog does not offer a video insert button', async () => {
  const html = await renderEditor()

  assert.ok(
    !html.includes('Вставити відео'),
    `expected no «Вставити відео» button in the editor:\n${html}`,
  )
  assert.ok(
    html.includes('news-text'),
    `expected the text field to stay in the editor:\n${html}`,
  )
})

test('NewsEditorDialog previews the attached file video on edit', async () => {
  const html = await renderEditor(makeArticle({ video: { kind: 'file', url: FILE_VIDEO_URL } }))
  const video = videoTag(html)

  assert.ok(video, `expected a <video> preview for the file video in:\n${html}`)
  assert.equal(attribute(video, 'src'), FILE_VIDEO_URL)
})

test('NewsEditorDialog fills the link input with the embed url on edit', async () => {
  const html = await renderEditor(makeArticle({ video: { kind: 'embed', url: EMBED_VIDEO_URL } }))
  const link = inputById(html, 'news-video-url')

  assert.ok(link, `expected an input#news-video-url for the embed video in:\n${html}`)
  assert.equal(attribute(link, 'value'), EMBED_VIDEO_URL)
})

test('NewsEditorDialog offers the file picker for a new article', async () => {
  const html = await renderEditor()

  assert.ok(
    inputById(html, 'news-video-file'),
    `expected an input#news-video-file for a new article in:\n${html}`,
  )
})
