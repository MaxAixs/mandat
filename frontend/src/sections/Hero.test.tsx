import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'

import Hero from '@/sections/Hero'
import type { Article } from '@/types'

function makeArticle(index: number, overrides: Partial<Article> = {}): Article {
  return {
    id: `hero-історія-${index}/деталь`,
    category: 'Новини',
    title: `Заголовок картки Hero ${index}`,
    excerpt: `Короткий опис картки Hero ${index}`,
    views: '12,4\u00A0тис.',
    publishedAt: '25.10.2026, 14:35',
    imageUrl: `https://cdn.example.com/hero-${index}.jpg`,
    ...overrides,
  }
}

function makeArticles(count: number): Article[] {
  return Array.from({ length: count }, (_, position) => makeArticle(position + 1))
}

function renderHero(articles: Article[]) {
  return render(
    <MemoryRouter>
      <Hero articles={articles} />
    </MemoryRouter>,
  )
}

// Only article links matter for the five-card contract; any future auxiliary
// link inside the Hero must not be mistaken for a news card.
function newsLinks(): HTMLElement[] {
  return screen
    .queryAllByRole('link')
    .filter((link) => (link.getAttribute('href') ?? '').startsWith('/news/'))
}

describe('Hero', () => {
  it('renders the first five articles as news links with encoded hrefs, titles, excerpts and cover images', () => {
    const articles = makeArticles(5).map((article, index) =>
      index === 4 ? { ...article, imageUrl: undefined } : article,
    )
    renderHero(articles)

    const links = newsLinks()
    expect(links).toHaveLength(5)

    articles.forEach((article, index) => {
      const link = links[index]
      expect(link, `expected a news card link for article ${index + 1}`).toBeDefined()
      expect(link).toHaveAttribute('href', `/news/${encodeURIComponent(article.id)}`)
      expect(within(link).getByText(article.title)).toBeInTheDocument()
      expect(within(link).getByText(article.excerpt)).toBeInTheDocument()

      if (article.imageUrl) {
        const image = link.querySelector('img')
        expect(image, `expected the cover image for article ${index + 1}`).not.toBeNull()
        expect(image).toHaveAttribute('src', article.imageUrl)
      } else {
        expect(link.querySelector('img')).toBeNull()
      }
    })
  })

  it('renders exactly one page-level h1 naming the main news', () => {
    renderHero(makeArticles(3))

    const pageHeadings = screen.getAllByRole('heading', { level: 1 })
    expect(pageHeadings).toHaveLength(1)
    expect(pageHeadings[0]).toHaveAccessibleName('Головні новини')

    // Card titles stay below the page-level heading.
    expect(screen.getAllByRole('heading', { level: 2 })).toHaveLength(3)
  })

  it('drops the old static marketing hero and never renders a sixth article', () => {
    const articles = makeArticles(6)
    renderHero(articles)

    // The main-event marketing block and its subscriber counter must be gone.
    expect(screen.queryByText('42 728')).not.toBeInTheDocument()
    expect(screen.queryByText('підписників уже з нами')).not.toBeInTheDocument()
    expect(screen.queryByText(/Про владу, рішення,/)).not.toBeInTheDocument()
    expect(
      screen.queryByText(
        'Факти без емоцій. Аналітика без шуму. Новини та інсайти, яким довіряють — у зручному форматі сайту.',
      ),
    ).not.toBeInTheDocument()

    expect(newsLinks()).toHaveLength(5)

    const sixth = articles[5]
    expect(screen.queryByText(sixth.title)).not.toBeInTheDocument()
    expect(screen.queryByText(sixth.excerpt)).not.toBeInTheDocument()
    expect(
      document.querySelector(`a[href="/news/${encodeURIComponent(sixth.id)}"]`),
    ).toBeNull()
  })

  it('keeps card content visible on touch-only tablets by gating hidden state behind hover media', () => {
    const article = makeArticle(1)
    renderHero([article])

    const link = newsLinks()[0]
    expect(link).toBeDefined()

    const title = within(link).getByText(article.title)
    const content = title.parentElement
    expect(content).not.toBeNull()
    if (!content) throw new Error('expected a content wrapper around the card copy')

    // Hidden state must only apply on devices that actually support hover.
    expect(content).toHaveClass('md:[@media(hover:hover)]:opacity-0')
    expect(content).toHaveClass('md:[@media(hover:hover)]:translate-y-4')

    // The un-gated tokens hide the text on coarse-pointer tablets >= 768px.
    expect(content.classList.contains('md:opacity-0')).toBe(false)
    expect(content.classList.contains('md:translate-y-4')).toBe(false)
  })

  it('disables flex, cover and content motion for users who prefer reduced motion', () => {
    const article = makeArticle(1)
    renderHero([article])

    const link = newsLinks()[0]
    expect(link).toBeDefined()
    expect(link).toHaveClass('motion-reduce:transition-none')

    const image = link.querySelector('img')
    expect(image).not.toBeNull()
    if (!image) throw new Error('expected a cover image for the card')
    expect(image).toHaveClass('motion-reduce:transition-none')
    expect(image).toHaveClass('motion-reduce:group-hover:scale-100')
    expect(image).toHaveClass('motion-reduce:group-focus-visible:scale-100')

    const content = within(link).getByText(article.title).parentElement
    expect(content).not.toBeNull()
    if (!content) throw new Error('expected a content wrapper around the card copy')
    expect(content).toHaveClass('motion-reduce:transition-none')
  })
})
