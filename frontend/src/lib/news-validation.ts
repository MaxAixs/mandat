// Relative imports keep this module loadable by the node:test suites without the alias.
import { MAX_IMAGE_BYTES, MAX_VIDEO_BYTES, IMAGE_TYPES, VIDEO_TYPES } from '../api/news'
import type { NewsFormValues } from '@/types'

export type NewsValidationErrors = Partial<Record<keyof NewsFormValues, string>>

export function validateNewsPayload(values: NewsFormValues): NewsValidationErrors {
  const errors: NewsValidationErrors = {}

  if (!values.title.trim()) {
    errors.title = 'Введіть заголовок новини.'
  }
  if (!values.category) {
    errors.category = 'Оберіть категорію.'
  }
  if (values.status === 'published' && !values.text.trim()) {
    errors.text = 'Додайте текст перед публікацією.'
  }
  if ([...values.language].length > 10) {
    errors.language = 'Код мови не може бути довшим за 10 символів.'
  }
  if (values.image_url && !isHttpUrl(values.image_url)) {
    errors.image_url = 'Введіть коректне посилання, яке починається з http:// або https://.'
  }
  if (values.source_url && !isHttpUrl(values.source_url)) {
    errors.source_url = 'Введіть коректне посилання, яке починається з http:// або https://.'
  }
  if (values.source_url && !values.source?.trim()) {
    errors.source = 'Вкажіть назву джерела, якщо додаєте посилання.'
  }
  if (values.video_url && !isHttpUrl(values.video_url)) {
    errors.video_url = 'Введіть коректне посилання на YouTube або Vimeo.'
  }

  return errors
}

// validateVideoFile checks a picked video before it is uploaded; null means ok.
export function validateVideoFile(file: { type: string; size: number }): string | null {
  if (!VIDEO_TYPES.includes(file.type)) return 'Підтримуються лише відео MP4 або WebM.'
  if (file.size <= 0) return 'Файл відео порожній.'
  if (file.size > MAX_VIDEO_BYTES) return 'Відео не може бути більшим за 500 МБ.'

  return null
}

// validateImageFile checks a picked image before it is uploaded; null means ok.
export function validateImageFile(file: { type: string; size: number }): string | null {
  if (!IMAGE_TYPES.includes(file.type)) return 'Підтримуються лише фото JPG, PNG, WebP або GIF.'
  if (file.size <= 0) return 'Файл фото порожній.'
  if (file.size > MAX_IMAGE_BYTES) return 'Фото не може бути більшим за 10 МБ.'

  return null
}

// validateImageLink checks a pasted image link; null means ok.
export function validateImageLink(url: string): string | null {
  return isHttpUrl(url.trim()) ? null : 'Введіть коректне посилання на зображення.'
}

function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}
