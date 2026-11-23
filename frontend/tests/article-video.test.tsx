import { test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

import type { Article, ArticleDetail, NewsVideo } from '../src/types'

// Compiled src modules import through the '@/...' alias; Node needs it resolved
// at runtime, exactly like the other frontend suites do.
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

interface ArticleVideoProps {
  video: NewsVideo
  title: string
  poster?: string
}

// src/components/ArticleVideo.tsx does not exist yet. Its module path stays a
// plain string on purpose: a literal import would fail compilation (TS2307),
// while this keeps the suite compiling so RED is a behavioral module-not-found
// failure instead of a type error.
const articleVideoModulePath: string = '../src/components/ArticleVideo'

async function loadArticleVideo(): Promise<ComponentType<ArticleVideoProps>> {
  const videoModule = (await import(articleVideoModulePath)) as {
    default?: unknown
  }

  assert.equal(
    typeof videoModule.default,
    'function',
    'ArticleVideo must default-export a component from src/components/ArticleVideo',
  )
  return videoModule.default as ComponentType<ArticleVideoProps>
}

async function loadArticleContent(): Promise<ComponentType<{ article: ArticleDetail }>> {
  // Loaded dynamically so the alias hook above is installed before Node
  // resolves Article's '@/...' imports.
  const articleModule = (await import('../src/pages/Article')) as {
    ArticleContent?: unknown
  }

  assert.equal(
    typeof articleModule.ArticleContent,
    'function',
    'Article.tsx must export ArticleContent',
  )
  return articleModule.ArticleContent as ComponentType<{ article: ArticleDetail }>
}

const FILE_VIDEO_URL = 'https://cdn.example.com/videos/news-1-a1b2c3d4.mp4'
const EMBED_VIDEO_URL = 'https://www.youtube-nocookie.com/embed/AbC123dEfG_'
const COVER_URL = 'https://cdn.example.com/article-cover.jpg'
const ARTICLE_TITLE = 'Повний заголовок статті'

function makeArticle(overrides: Partial<Article> = {}): Article {
  return {
    id: 'test-slug',
    category: 'Новини',
    title: ARTICLE_TITLE,
    excerpt: 'Короткий опис новини для картки',
    views: '12,4\u00A0тис.',
    publishedAt: '25.10.2026, 14:35',
    comments: '32 коментарі',
    featured: false,
    imageUrl: COVER_URL,
    ...overrides,
  }
}

function makeArticleDetail(overrides: Partial<ArticleDetail> = {}): ArticleDetail {
  return {
    ...makeArticle(),
    text: 'Перший абзац статті.',
    ...overrides,
  }
}

function attribute(attributes: string, name: string, caseInsensitive = false): string | undefined {
  const match = attributes.match(
    new RegExp(`(?:^|\\s)${name}="([^"]*)"`, caseInsensitive ? 'i' : ''),
  )

  return match?.[1]
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
}

function videoTag(html: string): string | undefined {
  const match = html.match(/<video\b([^>]*)>/)

  return match?.[1]
}

function iframeTag(html: string): string | undefined {
  const match = html.match(/<iframe\b([^>]*)>/)

  return match?.[1]
}

let articleVideoPromise: Promise<ComponentType<ArticleVideoProps>> | undefined
let articleContentPromise: Promise<ComponentType<{ article: ArticleDetail }>> | undefined

async function renderArticleVideo(props: ArticleVideoProps): Promise<string> {
  articleVideoPromise ??= loadArticleVideo()
  const ArticleVideo = await articleVideoPromise

  return renderToStaticMarkup(<ArticleVideo {...props} />)
}

async function renderArticleContent(article: ArticleDetail): Promise<string> {
  articleContentPromise ??= loadArticleContent()
  const ArticleContent = await articleContentPromise

  return renderToStaticMarkup(
    <MemoryRouter>
      <ArticleContent article={article} />
    </MemoryRouter>,
  )
}

test('ArticleVideo renders a file video with controls, metadata preload and inline playback', async () => {
  const html = await renderArticleVideo({
    video: { kind: 'file', url: FILE_VIDEO_URL },
    title: ARTICLE_TITLE,
  })
  const video = videoTag(html)

  assert.ok(video, `expected a <video> element in:\n${html}`)
  assert.equal(decodeHtml(attribute(video, 'src') ?? ''), FILE_VIDEO_URL)
  assert.ok(video.includes('controls'), `expected the video to have controls in:\n${html}`)
  assert.equal(attribute(video, 'preload'), 'metadata')
  assert.ok(
    /\bplaysinline\b/i.test(video),
    `expected the video to play inline in:\n${html}`,
  )
})

const posterCases: Array<{
  name: string
  poster: string | undefined
  expected: string | undefined
}> = [
  {
    name: 'uses the given poster for a file video',
    poster: COVER_URL,
    expected: COVER_URL,
  },
  {
    name: 'omits the poster attribute of a file video without a cover',
    poster: undefined,
    expected: undefined,
  },
]

for (const { name, poster, expected } of posterCases) {
  test(`ArticleVideo ${name}`, async () => {
    const html = await renderArticleVideo({
      video: { kind: 'file', url: FILE_VIDEO_URL },
      title: ARTICLE_TITLE,
      poster,
    })
    const video = videoTag(html)

    assert.ok(video, `expected a <video> element in:\n${html}`)
    assert.equal(
      attribute(video, 'poster') === undefined
        ? undefined
        : decodeHtml(attribute(video, 'poster') ?? ''),
      expected,
    )
  })
}

test('ArticleVideo renders an embed video as a lazy iframe with the article title in a 16:9 wrapper', async () => {
  const html = await renderArticleVideo({
    video: { kind: 'embed', url: EMBED_VIDEO_URL },
    title: ARTICLE_TITLE,
  })
  const iframe = iframeTag(html)

  assert.ok(iframe, `expected an <iframe> element in:\n${html}`)
  assert.equal(decodeHtml(attribute(iframe, 'src') ?? ''), EMBED_VIDEO_URL)
  assert.ok(
    decodeHtml(attribute(iframe, 'title') ?? '').includes(ARTICLE_TITLE),
    `expected the iframe title to contain the article title in:\n${html}`,
  )
  assert.ok(
    /\ballowfullscreen\b/i.test(iframe),
    `expected the iframe to allow fullscreen in:\n${html}`,
  )
  assert.equal(attribute(iframe, 'loading'), 'lazy')
  assert.equal(attribute(iframe, 'referrerpolicy', true), 'strict-origin-when-cross-origin')
  assert.match(
    html,
    /class="[^"]*\baspect-video\b[^"]*"/,
    `expected a 16:9 wrapper in:\n${html}`,
  )
})

test('ArticleContent renders the file video in the middle of the text', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({
      text: 'P1\n\nP2\n\nP3\n\nP4',
      video: { kind: 'file', url: FILE_VIDEO_URL },
    }),
  )

  const p1Index = html.indexOf('P1')
  const p2Index = html.indexOf('P2')
  const videoIndex = html.indexOf('<video')
  const p3Index = html.indexOf('P3')
  const p4Index = html.indexOf('P4')

  assert.ok(p1Index >= 0, `expected the P1 text in:\n${html}`)
  assert.ok(p2Index >= 0, `expected the P2 text in:\n${html}`)
  assert.ok(videoIndex >= 0, `expected the <video> element in:\n${html}`)
  assert.ok(p3Index >= 0, `expected the P3 text in:\n${html}`)
  assert.ok(p4Index >= 0, `expected the P4 text in:\n${html}`)
  assert.ok(
    p1Index < p2Index && p2Index < videoIndex && videoIndex < p3Index && p3Index < p4Index,
    `expected the order P1, P2, video, P3, P4 in:\n${html}`,
  )
})

test('ArticleContent renders the video after a single paragraph', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({
      text: 'Only paragraph.',
      video: { kind: 'file', url: FILE_VIDEO_URL },
    }),
  )
  const video = videoTag(html)

  assert.ok(video, `expected a <video> element in:\n${html}`)
  assert.equal(decodeHtml(attribute(video, 'src') ?? ''), FILE_VIDEO_URL)
  assert.ok(
    html.indexOf('<video') > html.indexOf('Only paragraph.'),
    `expected the video after the text in:\n${html}`,
  )
})

test('ArticleContent renders only the video for empty text', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({
      text: '',
      video: { kind: 'file', url: FILE_VIDEO_URL },
    }),
  )
  const video = videoTag(html)

  assert.ok(video, `expected a <video> element in:\n${html}`)
  assert.equal(decodeHtml(attribute(video, 'src') ?? ''), FILE_VIDEO_URL)
})

test('ArticleContent renders a literal [video] line as ordinary text without a video', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({ text: 'Intro\n[video]\nOutro' }),
  )

  assert.ok(!/<video\b/i.test(html), `expected no <video> element in:\n${html}`)
  assert.ok(!/<iframe\b/i.test(html), `expected no <iframe> element in:\n${html}`)
  assert.ok(html.includes('[video]'), `the literal [video] line must stay as text in:\n${html}`)

  const text = visibleText(html)
  assert.ok(text.includes('Intro'), `expected the Intro text in:\n${html}`)
  assert.ok(text.includes('Outro'), `expected the Outro text in:\n${html}`)
})

test('ArticleContent renders the embed video with the canonical src', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({
      text: 'P1\n\nP2\n\nP3\n\nP4',
      video: { kind: 'embed', url: EMBED_VIDEO_URL },
    }),
  )
  const iframe = iframeTag(html)

  assert.ok(iframe, `expected an <iframe> element in:\n${html}`)
  assert.equal(decodeHtml(attribute(iframe, 'src') ?? ''), EMBED_VIDEO_URL)
  assert.ok(!/<video\b/i.test(html), `expected no <video> element in:\n${html}`)
})

test('ArticleContent uses the article cover as the file video poster', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({
      text: 'P1\n\nP2\n\nP3\n\nP4',
      video: { kind: 'file', url: FILE_VIDEO_URL },
    }),
  )
  const video = videoTag(html)

  assert.ok(video, `expected a <video> element in:\n${html}`)
  assert.equal(decodeHtml(attribute(video, 'poster') ?? ''), COVER_URL)
})

test('ArticleContent keeps the markup free of players without a marker and a video', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({ text: 'First paragraph.\n\nSecond paragraph.' }),
  )

  assert.ok(!/<video\b/i.test(html), `expected no <video> element in:\n${html}`)
  assert.ok(!/<iframe\b/i.test(html), `expected no <iframe> element in:\n${html}`)

  const text = visibleText(html)
  assert.ok(text.includes('First paragraph.'), `expected the article text in:\n${html}`)
  assert.ok(text.includes('Second paragraph.'), `expected the article text in:\n${html}`)
})
