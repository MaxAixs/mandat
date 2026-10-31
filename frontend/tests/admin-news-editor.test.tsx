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

interface NewsEditorDialogProps {
  article?: AdminNews
  busy?: boolean
  error?: string | null
  onClose: () => void
  onSave: (values: NewsFormValues) => void
}

// The admin editor consumes the main-event flag from the API; keeping it on an
// intersection lets this suite fail on the missing switch before the field
// lands in the shared contract.
type AdminNewsWithMainEvent = AdminNews & { main_event: boolean }

async function loadNewsEditorDialog(): Promise<ComponentType<NewsEditorDialogProps>> {
  const editorModule = await import('../src/components/admin/NewsEditorDialog')

  return editorModule.default as ComponentType<NewsEditorDialogProps>
}

function makeArticle(overrides: Partial<AdminNewsWithMainEvent> = {}): AdminNewsWithMainEvent {
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
    image_url: '',
    views: 10,
    published_at: '2026-10-25T11:35:00Z',
    created_at: '2026-10-24T09:00:00Z',
    updated_at: '2026-10-25T11:35:00Z',
    main_event: false,
    ...overrides,
  }
}

function attribute(attributes: string, name: string): string | undefined {
  const match = attributes.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

  return match?.[1]
}

function labelTarget(html: string, labelText: string): string | undefined {
  const labels = Array.from(html.matchAll(/<label\b([^>]*)>([\s\S]*?)<\/label>/g))

  for (const [, attributes, content] of labels) {
    if (content.includes(labelText)) {
      return attribute(attributes, 'for')
    }
  }

  return undefined
}

function switchAttributes(html: string, id: string): string | undefined {
  const buttons = Array.from(html.matchAll(/<button\b([^>]*)>/g), (match) => match[1])

  return buttons.find(
    (attributes) =>
      attribute(attributes, 'id') === id && attribute(attributes, 'role') === 'switch',
  )
}

function switchForLabel(
  html: string,
  labelText: string,
): { id: string; attributes: string } | undefined {
  const id = labelTarget(html, labelText)
  if (!id) return undefined

  const attributes = switchAttributes(html, id)
  if (!attributes) return undefined

  return { id, attributes }
}

let editorPromise: Promise<ComponentType<NewsEditorDialogProps>> | undefined

async function renderEditor(article?: AdminNewsWithMainEvent): Promise<string> {
  editorPromise ??= loadNewsEditorDialog()
  const NewsEditorDialog = await editorPromise

  return renderToStaticMarkup(
    <NewsEditorDialog article={article} onClose={() => {}} onSave={() => {}} />,
  )
}

test('NewsEditorDialog shows an unchecked «Головна подія дня» switch for a new article', async () => {
  const html = await renderEditor()
  const mainEventSwitch = switchForLabel(html, 'Головна подія дня')

  assert.ok(
    mainEventSwitch,
    `expected a «Головна подія дня» switch in the editor markup:\n${html}`,
  )
  assert.equal(attribute(mainEventSwitch.attributes, 'aria-checked'), 'false')
  assert.equal(attribute(mainEventSwitch.attributes, 'data-state'), 'unchecked')
})

test('NewsEditorDialog mirrors article.main_event in the «Головна подія дня» switch', async () => {
  const cases = [
    {
      name: 'checks the switch for an article selected as the main event',
      article: makeArticle({ main_event: true }),
      expected: 'true',
    },
    {
      name: 'keeps the switch unchecked for an article that is not the main event',
      article: makeArticle({ main_event: false }),
      expected: 'false',
    },
  ]

  for (const { name, article, expected } of cases) {
    const html = await renderEditor(article)
    const mainEventSwitch = switchForLabel(html, 'Головна подія дня')

    assert.ok(mainEventSwitch, `${name}: expected the «Головна подія дня» switch in:\n${html}`)
    assert.equal(
      attribute(mainEventSwitch.attributes, 'aria-checked'),
      expected,
      `${name}: unexpected main-event switch state in:\n${html}`,
    )
  }
})

test('NewsEditorDialog keeps the existing «На головній» switch tied to featured', async () => {
  const cases = [
    {
      article: makeArticle({ featured: true, main_event: false }),
      expected: 'true',
    },
    {
      article: makeArticle({ featured: false, main_event: true }),
      expected: 'false',
    },
  ]

  for (const { article, expected } of cases) {
    const html = await renderEditor(article)
    const featuredSwitch = switchForLabel(html, 'На головній')

    assert.ok(featuredSwitch, `expected the existing «На головній» switch in:\n${html}`)
    assert.equal(
      attribute(featuredSwitch.attributes, 'aria-checked'),
      expected,
      `expected «На головній» to keep tracking article.featured in:\n${html}`,
    )
  }
})
