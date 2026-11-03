import { useEffect, useId, useRef, useState } from 'react'
import type { ChangeEvent, KeyboardEvent } from 'react'
import { useNavigate } from 'react-router'

import { DEFAULT_SEARCH_LIMIT, searchPublicNews } from '@/api/news'
import type { PublicNews } from '@/api/news'
import { cn } from '@/lib/utils'

interface SearchBoxProps {
  autoFocus?: boolean
  onNavigate?: () => void
}

const DEBOUNCE_MS = 250
const MAX_QUERY_LENGTH = 20

function formatDate(iso: string | null): string {
  if (!iso) return ''

  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''

  return date.toLocaleDateString('uk-UA', { day: 'numeric', month: 'long', year: 'numeric' })
}

export default function SearchBox({ autoFocus, onNavigate }: SearchBoxProps) {
  const navigate = useNavigate()
  const listId = useId()
  const containerRef = useRef<HTMLDivElement>(null)
  const requestId = useRef(0)

  const [value, setValue] = useState('')
  const [items, setItems] = useState<PublicNews[]>([])
  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)

  const showList = open && items.length > 0

  useEffect(() => {
    const trimmed = value.trim()
    if (trimmed === '') return

    const id = ++requestId.current
    const controller = new AbortController()
    const timer = setTimeout(() => {
      searchPublicNews(trimmed, DEFAULT_SEARCH_LIMIT, { signal: controller.signal }).then(
        (response) => {
          if (requestId.current !== id) return
          setItems(response.items)
          setOpen(response.items.length > 0)
          setActiveIndex(-1)
        },
        () => {
          if (requestId.current !== id) return
          setItems([])
          setOpen(false)
        },
      )
    }, DEBOUNCE_MS)

    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [value])

  useEffect(() => {
    function onPointerDown(event: PointerEvent) {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false)
      }
    }

    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [])

  function go(item: PublicNews) {
    setValue('')
    setItems([])
    setOpen(false)
    setActiveIndex(-1)
    onNavigate?.()
    navigate(`/news/${item.slug}`)
  }

  function onChange(event: ChangeEvent<HTMLInputElement>) {
    const next = event.target.value
    setValue(next)
    setActiveIndex(-1)
    setOpen(false)
    if (next.trim() === '') {
      setItems([])
    }
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      if (items.length === 0) return
      event.preventDefault()
      setOpen(true)
      setActiveIndex((prev) => {
        const next = event.key === 'ArrowDown' ? prev + 1 : prev - 1
        if (next < 0) return items.length - 1
        if (next >= items.length) return 0
        return next
      })
    } else if (event.key === 'Enter') {
      if (!showList) return
      const target = activeIndex >= 0 ? items[activeIndex] : items[0]
      if (target) {
        event.preventDefault()
        go(target)
      }
    } else if (event.key === 'Escape') {
      setOpen(false)
    }
  }

  return (
    <div ref={containerRef} className="relative w-full">
      <input
        type="search"
        role="combobox"
        aria-expanded={showList}
        aria-controls={showList ? listId : undefined}
        aria-activedescendant={showList && activeIndex >= 0 ? `${listId}-opt-${activeIndex}` : undefined}
        autoFocus={autoFocus}
        maxLength={MAX_QUERY_LENGTH}
        value={value}
        placeholder="Пошук новин…"
        onChange={onChange}
        onKeyDown={onKeyDown}
        onFocus={() => items.length > 0 && setOpen(true)}
        className="w-full h-10 px-4 text-[13px] text-mandat-text bg-mandat-bg outline-none border border-mandat-line focus:border-mandat-red-bright transition-colors"
      />
      {showList && (
        <ul
          id={listId}
          role="listbox"
          className="absolute left-0 right-0 top-full z-50 mt-1 max-h-[320px] overflow-y-auto border border-mandat-line bg-mandat-surface"
        >
          {items.map((item, index) => {
            const published = formatDate(item.published_at)
            return (
              <li
                key={item.slug}
                id={`${listId}-opt-${index}`}
                role="option"
                tabIndex={-1}
                aria-selected={index === activeIndex}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => go(item)}
                onMouseEnter={() => setActiveIndex(index)}
                className={cn(
                  'block w-full px-4 py-2.5 text-left cursor-pointer transition-colors',
                  index === activeIndex ? 'bg-mandat-surface-2' : 'bg-transparent',
                )}
              >
                <span className="block truncate text-[13px] text-mandat-text">{item.title}</span>
                <span className="mt-0.5 block text-[11px] uppercase tracking-wider text-mandat-muted">
                  {item.category}
                  {published && ` · ${published}`}
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
