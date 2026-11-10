import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

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

// src/pages/Feed.tsx is specified by the feed plan but does not exist yet.
// The path stays in a variable so the suite still compiles: the failure has to
// be the missing page, not a type error in this file.
async function loadFeed(): Promise<ComponentType> {
  const modulePath: string = '../src/pages/Feed'
  let pageModule: Record<string, unknown>

  try {
    pageModule = (await import(modulePath)) as Record<string, unknown>
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    assert.fail(
      'src/pages/Feed.tsx must exist and default-export the feed page, ' +
        `but importing it failed: ${reason}`,
    )

    return assert.fail('unreachable')
  }

  assert.equal(
    typeof pageModule.default,
    'function',
    'src/pages/Feed.tsx must default-export a component',
  )

  return pageModule.default as ComponentType
}

function renderFeed(Feed: ComponentType): string {
  return renderToStaticMarkup(
    <MemoryRouter initialEntries={['/news']}>
      <Feed />
    </MemoryRouter>,
  )
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

function filterLabels(html: string): string[] {
  const openingTag = /<div\b[^>]*\baria-label="Фільтр новин"[^>]*>/i.exec(html)
  assert.ok(openingTag, `expected the feed filter group in:\n${html}`)

  const start = openingTag.index + openingTag[0].length
  const end = html.indexOf('</div>', start)
  assert.ok(end > start, `expected the feed filter group to be closed in:\n${html}`)

  return buttons(html.slice(start, end)).map((button) => visibleText(button.content))
}

test('Feed page default-exports a component that renders the site feed', async () => {
  const Feed = await loadFeed()
  const html = renderFeed(Feed)

  assert.ok(html.length > 0, 'the feed page must render markup')
})

test('Feed page renders a filter button for every news rubric except «Про нас»', async () => {
  const Feed = await loadFeed()
  const html = renderFeed(Feed)

  assert.deepEqual(
    filterLabels(html),
    [
      'Всі новини',
      'Новини',
      'Міжнародні новини',
      'Спорт',
      'Інтерв\'ю',
      'Політика',
      'Регіональні новини',
    ],
    `expected «Всі новини» plus every news rubric except «Про нас» as feed filters:\n${html}`,
  )
})
