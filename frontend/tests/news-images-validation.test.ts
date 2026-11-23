import { test } from 'node:test'
import assert from 'node:assert/strict'

// src/lib/news-validation.ts does not export validateImageFile or
// validateImageLink yet. Their module path stays a plain string on purpose:
// literal imports would fail compilation (TS2305), while this keeps the
// suite compiling so RED is a behavioral missing-export failure instead of
// a type error.
interface NewsValidationModule {
  validateImageFile: (file: { type: string; size: number }) => string | null
  validateImageLink: (url: string) => string | null
}

async function loadNewsValidation(): Promise<NewsValidationModule> {
  const modulePath: string = '../src/lib/news-validation'
  const mod = (await import(modulePath)) as Partial<NewsValidationModule>

  assert.equal(
    typeof mod.validateImageFile,
    'function',
    'validateImageFile must be exported from src/lib/news-validation',
  )
  assert.equal(
    typeof mod.validateImageLink,
    'function',
    'validateImageLink must be exported from src/lib/news-validation',
  )
  return mod as NewsValidationModule
}

const MAX_IMAGE_BYTES = 10 * 1024 * 1024

const validImageCases = [
  { name: 'accepts a jpeg file', file: { type: 'image/jpeg', size: 1024 } },
  { name: 'accepts a png file', file: { type: 'image/png', size: 1024 } },
  { name: 'accepts a webp file', file: { type: 'image/webp', size: 1024 } },
  { name: 'accepts a gif file', file: { type: 'image/gif', size: 1024 } },
  { name: 'accepts exactly 10 MB', file: { type: 'image/jpeg', size: MAX_IMAGE_BYTES } },
]

for (const { name, file } of validImageCases) {
  test(`validateImageFile ${name}`, async () => {
    const { validateImageFile } = await loadNewsValidation()

    assert.equal(validateImageFile(file), null)
  })
}

const invalidImageTypeCases = [
  { name: 'rejects bmp', file: { type: 'image/bmp', size: 1024 } },
  { name: 'rejects svg', file: { type: 'image/svg+xml', size: 1024 } },
  { name: 'rejects videos', file: { type: 'video/mp4', size: 1024 } },
  { name: 'rejects an empty type', file: { type: '', size: 1024 } },
]

for (const { name, file } of invalidImageTypeCases) {
  test(`validateImageFile ${name}`, async () => {
    const { validateImageFile } = await loadNewsValidation()

    assert.equal(validateImageFile(file), 'Підтримуються лише фото JPG, PNG, WebP або GIF.')
  })
}

test('validateImageFile rejects an empty file', async () => {
  const { validateImageFile } = await loadNewsValidation()

  for (const type of ['image/jpeg', 'image/png', 'image/webp', 'image/gif']) {
    assert.equal(
      validateImageFile({ type, size: 0 }),
      'Файл фото порожній.',
      `expected the empty-file message for ${type}`,
    )
  }
})

test('validateImageFile rejects a file larger than 10 MB', async () => {
  const { validateImageFile } = await loadNewsValidation()

  for (const type of ['image/jpeg', 'image/png', 'image/webp', 'image/gif']) {
    assert.equal(
      validateImageFile({ type, size: MAX_IMAGE_BYTES + 1 }),
      'Фото не може бути більшим за 10 МБ.',
      `expected the oversize message for ${type}`,
    )
  }
})

const validImageLinkCases = [
  { name: 'an https link', url: 'https://example.com/images/a.jpg' },
  { name: 'an http link', url: 'http://example.com/images/a.png' },
  { name: 'a link with surrounding whitespace', url: '  https://example.com/images/a.webp  ' },
]

for (const { name, url } of validImageLinkCases) {
  test(`validateImageLink accepts ${name}`, async () => {
    const { validateImageLink } = await loadNewsValidation()

    assert.equal(validateImageLink(url), null)
  })
}

const invalidImageLinkCases = [
  { name: 'a bare string', url: 'not-a-url' },
  { name: 'a host without a scheme', url: 'example.com/images/a.jpg' },
  { name: 'a non-http scheme', url: 'ftp://example.com/images/a.jpg' },
  { name: 'an empty string', url: '' },
  { name: 'whitespace only', url: '   ' },
]

for (const { name, url } of invalidImageLinkCases) {
  test(`validateImageLink rejects ${name}`, async () => {
    const { validateImageLink } = await loadNewsValidation()

    assert.equal(validateImageLink(url), 'Введіть коректне посилання на зображення.')
  })
}
