import { useState } from 'react'
import { ArrowDown, ArrowUp, Link2, Trash2 } from 'lucide-react'

import { MAX_IMAGES, IMAGE_TYPES } from '@/api/news'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { ImageItem } from '@/hooks/use-image-uploads'
import { cn } from '@/lib/utils'

interface NewsImagesFieldProps {
  items: ImageItem[]
  error?: string
  onAddFiles: (files: File[]) => void
  /** Returns false when the link was rejected, so the input keeps it for editing. */
  onAddLink: (url: string) => boolean | void
  onMove: (id: string, direction: 'up' | 'down') => void
  onRemove: (id: string) => void
}

const inputClassName =
  'h-11 rounded-none border-mandat-line bg-mandat-bg text-mandat-text placeholder:text-mandat-muted focus-visible:ring-mandat-red-bright/30'

const iconButtonClassName =
  'size-8 rounded-none text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text disabled:opacity-30'

export default function NewsImagesField({ items, error, onAddFiles, onAddLink, onMove, onRemove }: NewsImagesFieldProps) {
  const [link, setLink] = useState('')
  const full = items.length >= MAX_IMAGES

  const addLink = () => {
    if (!link.trim()) return
    if (onAddLink(link) !== false) setLink('')
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-xs text-mandat-muted">
        <span>JPG, PNG, WebP або GIF, до 10 МБ</span>
        <span>
          {items.length} / {MAX_IMAGES}
        </span>
      </div>

      {items.length > 0 && (
        <ol className="grid gap-2 sm:grid-cols-2">
          {items.map((item, index) => (
            <li key={item.id} className="flex gap-3 border border-mandat-line bg-mandat-bg p-2">
              <img
                src={item.preview}
                alt={`Фото ${index + 1}`}
                className={cn('size-20 shrink-0 object-cover', item.status !== 'ready' && 'opacity-60')}
              />
              <div className="flex min-w-0 flex-1 flex-col justify-between gap-2">
                {item.status === 'uploading' && (
                  <div className="space-y-1">
                    <div className="text-xs text-mandat-muted">{Math.round(item.progress * 100)}%</div>
                    <div
                      role="progressbar"
                      aria-valuemin={0}
                      aria-valuemax={100}
                      aria-valuenow={Math.round(item.progress * 100)}
                      className="h-1 bg-mandat-line"
                    >
                      <div className="h-full bg-mandat-red-bright" style={{ width: `${item.progress * 100}%` }} />
                    </div>
                  </div>
                )}
                {item.status === 'error' && item.error && (
                  <p role="alert" className="text-xs text-mandat-red-bright">
                    {item.error}
                  </p>
                )}
                <div className="mt-auto flex justify-end gap-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label="Перемістити вгору"
                    disabled={index === 0}
                    onClick={() => onMove(item.id, 'up')}
                    className={iconButtonClassName}
                  >
                    <ArrowUp className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label="Перемістити вниз"
                    disabled={index === items.length - 1}
                    onClick={() => onMove(item.id, 'down')}
                    className={iconButtonClassName}
                  >
                    <ArrowDown className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label="Видалити фото"
                    onClick={() => onRemove(item.id)}
                    className={iconButtonClassName}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </div>
            </li>
          ))}
        </ol>
      )}

      {full ? (
        <p className="text-xs text-mandat-muted">Досягнуто ліміт {MAX_IMAGES} фото.</p>
      ) : (
        <div className="grid gap-2 sm:grid-cols-2">
          <Input
            id="news-images-file"
            type="file"
            multiple
            accept={IMAGE_TYPES.join(',')}
            onChange={(event) => {
              const files = Array.from(event.target.files ?? [])
              event.target.value = ''
              if (files.length > 0) onAddFiles(files)
            }}
            className={cn(inputClassName, 'pt-2.5')}
          />
          <div className="flex gap-2">
            <div className="relative flex-1">
              <Link2 className="absolute left-3 top-3.5 size-4 text-mandat-muted" />
              <Input
                id="news-image-url"
                value={link}
                onChange={(event) => setLink(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault()
                    addLink()
                  }
                }}
                placeholder="https://... посилання на фото"
                className={cn(inputClassName, 'pl-10')}
              />
            </div>
            <Button
              type="button"
              variant="ghost"
              onClick={addLink}
              className="h-11 rounded-none border border-mandat-line text-mandat-text hover:bg-mandat-surface-2"
            >
              Додати
            </Button>
          </div>
        </div>
      )}

      {error && <p className="text-xs text-mandat-red-bright">{error}</p>}
    </div>
  )
}
