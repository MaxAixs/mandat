import type { NewsFormValues } from '@/types'

export type NewsValidationErrors = Partial<Record<keyof NewsFormValues, string>>

export function validateNewsPayload(values: NewsFormValues): NewsValidationErrors {
  const errors: NewsValidationErrors = {}

  if (!values.title.trim()) {
    errors.title = 'Введіть заголовок новини.'
  }
  if (!values.category) {
    errors.category = 'Оберіть рубрику.'
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

  return errors
}

function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}
