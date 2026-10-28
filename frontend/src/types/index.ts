// Shared TypeScript types — domain model of the site.
// These mirror what the Go backend will eventually serve.

/** Content rubric shown as a badge on cards. */
export type Category = 'Новини' | 'Міжнародні новини' | 'Спорт' | 'Інтерв\'ю' | 'Про нас'

/** A news/analytics article card. */
export interface Article {
  id: string
  category: Category
  title: string
  excerpt: string
  /** Preformatted for display ('12.4K'); the backend will send a number later. */
  views: string
  /** 'HH:MM' for now; will become an ISO date from the backend. */
  publishedAt: string
  /** Preformatted ('32 коментарі'); optional — hidden when absent. */
  comments?: string
  /** Featured article renders as a large card. */
  featured?: boolean
  /** Cover image served by the backend; a pattern is used when absent. */
  imageUrl?: string
}

/** A complete news article with its plain-text body. */
export interface ArticleDetail extends Article {
  text: string
}

/** A single entry in the "Останні новини" feed. */
export interface LatestNewsItem {
  id: string
  /** 'HH:MM' for now; will become an ISO date from the backend. */
  time: string
  /** Source/rubric line shown next to the time; hidden when absent. */
  source?: string
  /** Small outlined pill before the title ('Фото', 'Відео'); hidden when absent. */
  badge?: string
  /** Renders the red "Важливо" pill before the title. */
  important?: boolean
  title: string
}

/** A single "Цифра дня" statistic. */
export interface StatItem {
  value: string
  unit?: string
  caption: string
}

/** A navigation link. */
export interface NavLink {
  label: string
  href: string
}

/** Publication state used by the Go admin API. */
export type NewsStatus = 'draft' | 'published'

/** Editable news fields, mirroring the backend request DTO. */
export interface NewsFormValues {
  category: Category
  title: string
  description: string
  text: string
  language: string
  status: NewsStatus
  featured: boolean
  image_url: string
}

/** News article displayed in the administration panel. */
export interface AdminNews extends NewsFormValues {
  id: string
  slug: string
  views: number
  published_at: string | null
  created_at: string
  updated_at: string
}
