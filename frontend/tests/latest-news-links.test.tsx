import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

import type { LatestNewsItem } from '../src/types'

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the other frontend tests do.
interface ModuleInternals {
  _resolveFilename: (
    request: string,
    parent?: unknown,
    isMain?: boolean,
    options?: unknown,
  ) => string
}

const moduleInternals = Module as unknown as ModuleInternals
const originalResolveFilename = moduleInternals._resolveFilename
const compiledSrcRoot = path.resolve(__dirname, '..', 'src')

moduleInternals._resolveFilename = function resolveAlias(request, ...rest) {
  const target = request.startsWith('@/') ? path.join(compiledSrcRoot, request.slice(2)) : request

  return originalResolveFilename.call(moduleInternals, target, ...rest)
}

after(() => {
  moduleInternals._resolveFilename = originalResolveFilename
})

function makeItem(overrides: Partial<LatestNewsItem> = {}): LatestNewsItem {
  return {
    id: 'test-slug',
    time: '14:35',
    source: 'Новини',
    important: false,
    title: 'Тестовий заголовок новини',
    ...overrides,
  }
}

// LatestNews imports '@/lib/utils'; the alias resolver is installed by this
// module above, so the section has to be loaded after it (dynamic import).
async function loadLatestNews(): Promise<ComponentType<{ items: LatestNewsItem[] }>> {
  const latestNewsModule = await import('../src/sections/LatestNews')

  return latestNewsModule.default as ComponentType<{ items: LatestNewsItem[] }>
}

async function renderLatest(items: LatestNewsItem[]): Promise<string> {
  const LatestNews = await loadLatestNews()

  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/']}>
      <LatestNews items={items} />
    </MemoryRouter>,
  )
}

function attribute(attributes: string, name: string): string | undefined {
  const match = attributes.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

  return match?.[1]
}

interface AnchorElement {
  attributes: string
  content: string
}

function anchors(html: string): AnchorElement[] {
  return Array.from(html.matchAll(/<a\b([^>]*)>([\s\S]*?)<\/a>/g), (match) => ({
    attributes: match[1],
    content: match[2],
  }))
}

function decodeHtml(value: string): string {
  return value
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#x27;/g, "'")
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, '&')
}

function visibleText(html: string): string {
  return decodeHtml(html.replace(/<[^>]*>/g, ' '))
    .replace(/&nbsp;/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function hrefOf(anchor: AnchorElement): string {
  return attribute(anchor.attributes, 'href') ?? ''
}

test('LatestNews «Всі новини» links to the public /news feed', async () => {
  const html = await renderLatest([makeItem({ id: 'latest-one' })])
  const allNewsLink = anchors(html).find((anchor) =>
    visibleText(anchor.content).startsWith('Всі новини'),
  )

  assert.ok(allNewsLink, `expected the «Всі новини» link in:\n${html}`)
  assert.equal(
    hrefOf(allNewsLink),
    '/news',
    `expected «Всі новини» to open /news, got ${JSON.stringify(hrefOf(allNewsLink))}`,
  )
})

test('LatestNews rows link to their article page', async () => {
  const items = [
    makeItem({ id: 'latest-one', title: 'Перша остання новина' }),
    makeItem({ id: 'latest/two слот', title: 'Друга остання новина', time: '09:05' }),
  ]
  const html = await renderLatest(items)
  const links = anchors(html).filter((anchor) => hrefOf(anchor).startsWith('/news/'))

  assert.equal(
    links.length,
    items.length,
    `expected one article link per row, got ${JSON.stringify(links.map(hrefOf))} in:\n${html}`,
  )

  for (const item of items) {
    const expectedHref = `/news/${encodeURIComponent(item.id)}`
    const link = links.find((candidate) => hrefOf(candidate) === expectedHref)

    assert.ok(link, `expected the «${item.title}» row to link to ${expectedHref} in:\n${html}`)
    assert.ok(
      visibleText(link.content).includes(item.title),
      `expected the ${expectedHref} link to contain «${item.title}»`,
    )
  }
})

test('LatestNews rows do not fall back to a placeholder href', async () => {
  const html = await renderLatest([makeItem({ id: 'latest-placeholder' })])

  for (const anchor of anchors(html)) {
    assert.notEqual(
      hrefOf(anchor),
      '#',
      `expected no placeholder anchor in the latest news strip, got ${JSON.stringify(
        visibleText(anchor.content),
      )}`,
    )
  }
})
