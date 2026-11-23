import { test } from 'node:test'
import assert from 'node:assert/strict'
import Module from 'node:module'
import path from 'node:path'

import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router'
import type { ComponentType } from 'react'

import type { Article, ArticleDetail } from '../src/types'

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

interface ArticleImageProps {
  src: string
  alt: string
}

// src/components/ArticleImage.tsx does not exist yet. Its module path stays a
// plain string on purpose: a literal import would fail compilation (TS2307),
// while this keeps the suite compiling so RED is a behavioral module-not-found
// failure instead of a type error.
const articleImageModulePath: string = '../src/components/ArticleImage'

async function loadArticleImage(): Promise<ComponentType<ArticleImageProps>> {
  const imageModule = (await import(articleImageModulePath)) as {
    default?: unknown
  }

  assert.equal(
    typeof imageModule.default,
    'function',
    'ArticleImage must default-export a component from src/components/ArticleImage',
  )
  return imageModule.default as ComponentType<ArticleImageProps>
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
const COVER_URL = 'https://cdn.example.com/article-cover.jpg'
const ARTICLE_TITLE = 'Повний заголовок статті'
const IMAGE_A = 'https://cdn.example.com/images/image-a.jpg'
const IMAGE_B = 'https://cdn.example.com/images/image-b.jpg'
const IMAGE_C = 'https://cdn.example.com/images/image-c.jpg'

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

function attribute(attributes: string, name: string): string | undefined {
  const match = attributes.match(new RegExp(`(?:^|\\s)${name}="([^"]*)"`))

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

function imgTags(html: string): string[] {
  return Array.from(html.matchAll(/<img\b([^>]*)>/g), (match) => match[1])
}

function buttons(html: string): { attributes: string; content: string }[] {
  return Array.from(html.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g), (match) => ({
    attributes: match[1],
    content: match[2],
  }))
}

function imageOpenerButtons(html: string): { attributes: string; content: string }[] {
  return buttons(html).filter((button) =>
    decodeHtml(attribute(button.attributes, 'aria-label') ?? '').includes('Відкрити фото'),
  )
}

function imageImgTags(html: string, images: string[]): string[] {
  return imgTags(html).filter((attributes) =>
    images.includes(decodeHtml(attribute(attributes, 'src') ?? '')),
  )
}

function assertOrder(html: string, needles: string[]): void {
  let fromIndex = 0
  for (const needle of needles) {
    const index = html.indexOf(needle, fromIndex)

    assert.ok(
      index >= 0,
      `expected ${JSON.stringify(needle)} after index ${fromIndex} in:\n${html}`,
    )
    fromIndex = index + needle.length
  }
}

let articleImagePromise: Promise<ComponentType<ArticleImageProps>> | undefined
let articleContentPromise: Promise<ComponentType<{ article: ArticleDetail }>> | undefined

async function renderArticleImage(props: ArticleImageProps): Promise<string> {
  articleImagePromise ??= loadArticleImage()
  const ArticleImage = await articleImagePromise

  return renderToStaticMarkup(<ArticleImage {...props} />)
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

test('ArticleImage renders the image inside a button labelled "Відкрити фото"', async () => {
  const html = await renderArticleImage({ src: IMAGE_A, alt: ARTICLE_TITLE })
  const openers = imageOpenerButtons(html)

  assert.equal(
    openers.length,
    1,
    `expected exactly one "Відкрити фото" button in:\n${html}`,
  )
  assert.equal(
    attribute(openers[0].attributes, 'type'),
    'button',
    `expected the opener to be type="button" in:\n${html}`,
  )

  const imgs = imgTags(openers[0].content)

  assert.equal(
    imgs.length,
    1,
    `expected the image <img> inside the opener button in:\n${html}`,
  )
  assert.equal(decodeHtml(attribute(imgs[0], 'src') ?? ''), IMAGE_A)
  assert.equal(decodeHtml(attribute(imgs[0], 'alt') ?? ''), ARTICLE_TITLE)
  assert.equal(attribute(imgs[0], 'loading'), 'lazy')
})

test('ArticleImage keeps the viewer closed by default', async () => {
  const html = await renderArticleImage({ src: IMAGE_A, alt: ARTICLE_TITLE })

  assert.ok(
    !/role="dialog"/i.test(html),
    `expected no viewer dialog in the static markup, got:\n${html}`,
  )
})

test('ArticleContent spreads two images through four paragraphs', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({
      text: 'P1\n\nP2\n\nP3\n\nP4',
      images: [IMAGE_A, IMAGE_B],
    }),
  )

  assertOrder(html, ['P1', IMAGE_A, 'P2', 'P3', IMAGE_B, 'P4'])
})

test('ArticleContent interleaves the file video with two images', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({
      text: 'P1\n\nP2\n\nP3\n\nP4',
      video: { kind: 'file', url: FILE_VIDEO_URL },
      images: [IMAGE_A, IMAGE_B],
    }),
  )

  assertOrder(html, ['P1', IMAGE_A, 'P2', '<video', 'P3', IMAGE_B, 'P4'])
})

test('ArticleContent renders each image with the article title, lazy loading and an opener button', async () => {
  const images = [IMAGE_A, IMAGE_B]
  const html = await renderArticleContent(
    makeArticleDetail({
      text: 'P1\n\nP2\n\nP3\n\nP4',
      images,
    }),
  )
  const imgs = imageImgTags(html, images)

  assert.equal(
    imgs.length,
    images.length,
    `expected one <img> per image in:\n${html}`,
  )
  for (const img of imgs) {
    assert.equal(
      decodeHtml(attribute(img, 'alt') ?? ''),
      ARTICLE_TITLE,
      `expected the image alt to equal the article title in:\n${html}`,
    )
    assert.equal(attribute(img, 'loading'), 'lazy')
  }

  const openers = imageOpenerButtons(html)

  assert.equal(
    openers.length,
    images.length,
    `expected one "Відкрити фото" button per image in:\n${html}`,
  )
  for (const src of images) {
    assert.ok(
      openers.some((button) => decodeHtml(button.content).includes(src)),
      `expected the image ${src} inside a "Відкрити фото" button in:\n${html}`,
    )
  }
})

test('ArticleContent renders three images after a single paragraph in order', async () => {
  const images = [IMAGE_A, IMAGE_B, IMAGE_C]
  const html = await renderArticleContent(
    makeArticleDetail({
      text: 'Only paragraph.',
      images,
    }),
  )

  assertOrder(html, ['Only paragraph.', IMAGE_A, IMAGE_B, IMAGE_C])
})

test('ArticleContent renders only the images for empty text', async () => {
  const images = [IMAGE_A, IMAGE_B]
  const html = await renderArticleContent(
    makeArticleDetail({
      text: '',
      images,
    }),
  )
  const imgs = imageImgTags(html, images)

  assert.equal(imgs.length, images.length, `expected both images in:\n${html}`)
  assertOrder(html, [IMAGE_A, IMAGE_B])
  assert.ok(!/<video\b/i.test(html), `expected no <video> element in:\n${html}`)
  assert.ok(!/<iframe\b/i.test(html), `expected no <iframe> element in:\n${html}`)
})

test('ArticleContent renders no extra media without images and a video', async () => {
  const html = await renderArticleContent(
    makeArticleDetail({ text: 'First paragraph.\n\nSecond paragraph.' }),
  )

  assert.ok(!/<video\b/i.test(html), `expected no <video> element in:\n${html}`)
  assert.ok(!/<iframe\b/i.test(html), `expected no <iframe> element in:\n${html}`)

  const imgs = imgTags(html)

  assert.equal(imgs.length, 1, `expected only the cover image in:\n${html}`)
  assert.equal(decodeHtml(attribute(imgs[0], 'src') ?? ''), COVER_URL)
  assert.ok(html.includes('First paragraph.'), `expected the article text in:\n${html}`)
  assert.ok(html.includes('Second paragraph.'), `expected the article text in:\n${html}`)
})
