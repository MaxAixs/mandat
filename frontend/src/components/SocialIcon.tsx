interface SocialIconProps {
  id: string
  className?: string
}

/**
 * Brand glyphs for the footer social buttons (Instagram, Threads, Telegram).
 * Drawn in `currentColor` so the parent controls color and hover state.
 */
export default function SocialIcon({ id, className }: SocialIconProps) {
  switch (id) {
    case 'instagram':
      return (
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className={className} aria-hidden="true">
          <rect x="3" y="3" width="18" height="18" rx="5" />
          <circle cx="12" cy="12" r="4.2" />
          <circle cx="17.4" cy="6.6" r="1.1" fill="currentColor" stroke="none" />
        </svg>
      )
    case 'threads':
      return (
        <svg viewBox="0 0 24 24" fill="currentColor" className={className} aria-hidden="true">
          <path d="M17.2 11.1a7.3 7.3 0 0 0-.3-.1c-.2-3-1.8-4.7-4.6-4.7h-.1c-1.7 0-3 .7-3.9 2l1.5 1c.6-1 1.6-1.2 2.4-1.2 1 0 1.7.3 2.2.9.4.4.6 1 .7 1.8a12 12 0 0 0-2.8-.1c-2.8.2-4.6 1.8-4.5 4.1.1 1.2.7 2.2 1.7 2.8.8.5 1.9.8 3 .7 1.5-.1 2.7-.6 3.5-1.7.6-.8 1-1.8 1.2-3.1.7.4 1.2 1 1.5 1.7.5 1.2.5 3.1-1 4.6-1.4 1.4-3 2-5.5 2-2.8 0-4.9-.9-6.2-2.6C4.9 17.6 4.3 15.6 4.2 13c.1-2.6.7-4.6 1.9-6.2 1.3-1.7 3.4-2.6 6.2-2.6 2.8 0 5 .9 6.4 2.7.7.8 1.2 1.9 1.6 3.1l1.8-.5c-.4-1.5-1.1-2.8-2-3.9-1.8-2.2-4.4-3.4-7.8-3.4-3.4 0-6 1.2-7.7 3.3C3 7.5 2.2 9.9 2.2 13c0 3 .8 5.5 2.4 7.4 1.7 2.1 4.3 3.3 7.7 3.3 3 0 5.1-.8 6.9-2.6 2.3-2.3 2.2-5.2 1.5-6.9-.5-1.2-1.6-2.3-3.5-3.1Zm-5.2 5.6c-1.2.1-2.5-.5-2.6-1.6 0-.9.6-1.8 2.7-1.9h.7c.8 0 1.5.1 2.1.2-.2 3-1.6 3.2-2.9 3.3Z" />
        </svg>
      )
    case 'telegram':
      return (
        <svg viewBox="0 0 24 24" fill="currentColor" className={className} aria-hidden="true">
          <path d="M21.4 3.6 2.9 10.8c-1.3.5-1.2 1.2-.2 1.5l4.7 1.5 1.8 5.6c.2.6.4.8.8.8.4 0 .6-.2.9-.5l2.3-2.2 4.8 3.5c.9.5 1.5.2 1.7-.8l3.1-14.7c.3-1.3-.5-1.9-1.4-1.5Zm-3.3 3.5-8.5 7.7-.3 3.6-1.6-5 9.9-6.3c.5-.3.9 0 .5 0Z" />
        </svg>
      )
    default:
      return null
  }
}
