// Shared TypeScript types — domain model of the site.
// These mirror what the Go backend will eventually serve.

/** Content rubric shown as a badge on cards. */
export type Category = 'Новини' | 'Аналітика' | 'Інсайди' | 'Важливо' | 'Розслідування'

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
