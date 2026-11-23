import { test } from 'node:test'
import assert from 'node:assert/strict'

import type { NewsFormValues } from '../src/types'

// src/lib/news-validation.ts does not export validateVideoFile yet. The module
// path stays a plain string on purpose: a literal import would fail compilation
// (TS2305), while this keeps the suite compiling so RED is a behavioral
// missing-export failure instead of a type error.
interface NewsValidationModule {
  validateNewsPayload: (values: NewsFormValues) => Partial<Record<string, string>>
  validateVideoFile: (file: { type: string; size: number }) => string | null
}

async function loadNewsValidation(): Promise<NewsValidationModule> {
  const modulePath: string = '../src/lib/news-validation'
  const mod = (await import(modulePath)) as Partial<NewsValidationModule>

  assert.equal(
    typeof mod.validateNewsPayload,
    'function',
    'validateNewsPayload must be exported from src/lib/news-validation',
  )
  assert.equal(
    typeof mod.validateVideoFile,
    'function',
    'validateVideoFile must be exported from src/lib/news-validation',
  )
  return mod as NewsValidationModule
}

const payload: NewsFormValues = {
  category: 'Новини',
  title: 'Тестова новина',
  description: 'Короткий опис',
  text: 'Повний текст',
  language: 'uk',
  status: 'draft',
  featured: false,
  main_event: false,
  image_url: '',
}

const MAX_VIDEO_BYTES = 500 * 1024 * 1024

const validVideoCases = [
  { name: 'accepts an mp4 file', file: { type: 'video/mp4', size: 1024 } },
  { name: 'accepts a webm file', file: { type: 'video/webm', size: 1024 } },
  { name: 'accepts exactly 500 MB', file: { type: 'video/mp4', size: MAX_VIDEO_BYTES } },
]

for (const { name, file } of validVideoCases) {
  test(`validateVideoFile ${name}`, async () => {
    const { validateVideoFile } = await loadNewsValidation()

    assert.equal(validateVideoFile(file), null)
  })
}

const invalidTypeCases = [
  { name: 'rejects ogg', file: { type: 'video/ogg', size: 1024 } },
  { name: 'rejects quicktime', file: { type: 'video/quicktime', size: 1024 } },
  { name: 'rejects images', file: { type: 'image/png', size: 1024 } },
  { name: 'rejects an empty type', file: { type: '', size: 1024 } },
]

for (const { name, file } of invalidTypeCases) {
  test(`validateVideoFile ${name}`, async () => {
    const { validateVideoFile } = await loadNewsValidation()

    assert.equal(validateVideoFile(file), 'Підтримуються лише відео MP4 або WebM.')
  })
}

test('validateVideoFile rejects an empty file', async () => {
  const { validateVideoFile } = await loadNewsValidation()

  for (const type of ['video/mp4', 'video/webm']) {
    assert.equal(
      validateVideoFile({ type, size: 0 }),
      'Файл відео порожній.',
      `expected the empty-file message for ${type}`,
    )
  }
})

test('validateVideoFile rejects a file larger than 500 MB', async () => {
  const { validateVideoFile } = await loadNewsValidation()

  for (const type of ['video/mp4', 'video/webm']) {
    assert.equal(
      validateVideoFile({ type, size: MAX_VIDEO_BYTES + 1 }),
      'Відео не може бути більшим за 500 МБ.',
      `expected the oversize message for ${type}`,
    )
  }
})

test('validateNewsPayload leaves empty video fields alone', async () => {
  const { validateNewsPayload } = await loadNewsValidation()

  assert.deepEqual(validateNewsPayload({ ...payload }), {})
  assert.deepEqual(validateNewsPayload({ ...payload, video_key: '', video_url: '' }), {})
})

const invalidVideoUrlCases = [
  { name: 'a bare string', video_url: 'not-a-url' },
  { name: 'a host without a scheme', video_url: 'www.youtube.com/watch?v=AbC123dEfG_' },
  { name: 'a non-http scheme', video_url: 'ftp://cdn.example.com/videos/a.mp4' },
]

for (const { name, video_url } of invalidVideoUrlCases) {
  test(`validateNewsPayload rejects ${name} as video_url`, async () => {
    const { validateNewsPayload } = await loadNewsValidation()

    assert.equal(
      validateNewsPayload({ ...payload, video_url }).video_url,
      'Введіть коректне посилання на YouTube або Vimeo.',
    )
  })
}

const validVideoUrlCases = [
  { name: 'a YouTube watch link', video_url: 'https://www.youtube.com/watch?v=AbC123dEfG_' },
  { name: 'a YouTube short link', video_url: 'https://youtu.be/AbC123dEfG_' },
  { name: 'a Vimeo link', video_url: 'https://vimeo.com/123456789' },
]

for (const { name, video_url } of validVideoUrlCases) {
  test(`validateNewsPayload accepts ${name} as video_url`, async () => {
    const { validateNewsPayload } = await loadNewsValidation()

    assert.equal(validateNewsPayload({ ...payload, video_url }).video_url, undefined)
  })
}

test('validateNewsPayload allows two [video] lines in the text', async () => {
  const { validateNewsPayload } = await loadNewsValidation()

  const errors = validateNewsPayload({
    ...payload,
    status: 'published',
    text: 'A\n[video]\nB\n[video]\nC',
  })

  assert.deepEqual(errors, {}, `expected no errors for marker-like lines, got: ${JSON.stringify(errors)}`)
})

test('validateNewsPayload keeps the published-text error priority on empty text', async () => {
  const { validateNewsPayload } = await loadNewsValidation()

  const errors = validateNewsPayload({ ...payload, status: 'published', text: '   ' })

  assert.equal(errors.text, 'Додайте текст перед публікацією.')
})
