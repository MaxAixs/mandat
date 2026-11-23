import { useState } from 'react'
import { Film, Link2, X } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { VideoUploadState } from '@/hooks/use-video-upload'
import { cn } from '@/lib/utils'
import type { NewsVideo } from '@/types'

type VideoMode = 'file' | 'link'

interface NewsVideoFieldProps {
  videoKey: string
  videoUrl: string
  /** The video saved with the article; null for a new article or none. */
  currentVideo: NewsVideo | null
  upload: VideoUploadState
  error?: string
  onSelectFile: (file: File) => void
  onCancelUpload: () => void
  onLinkChange: (url: string) => void
  onRemove: () => void
}

const inputClassName =
  'h-11 rounded-none border-mandat-line bg-mandat-bg text-mandat-text placeholder:text-mandat-muted focus-visible:ring-mandat-red-bright/30'

const modeButtonClassName = 'h-9 flex-1 rounded-none border border-mandat-line text-sm'

export default function NewsVideoField({
  videoKey,
  videoUrl,
  currentVideo,
  upload,
  error,
  onSelectFile,
  onCancelUpload,
  onLinkChange,
  onRemove,
}: NewsVideoFieldProps) {
  const isCurrentFile = currentVideo?.kind === 'file' && videoUrl === currentVideo.url
  const [mode, setMode] = useState<VideoMode>(() =>
    currentVideo?.kind === 'embed' || (videoUrl !== '' && !isCurrentFile) ? 'link' : 'file',
  )

  const removeButton = (
    <Button
      type="button"
      variant="ghost"
      onClick={onRemove}
      className="rounded-none text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
    >
      <X className="size-4" />
      Видалити відео
    </Button>
  )

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        {(['file', 'link'] as const).map((value) => (
          <button
            key={value}
            type="button"
            aria-pressed={mode === value}
            onClick={() => setMode(value)}
            className={cn(
              modeButtonClassName,
              mode === value ? 'border-mandat-red-bright text-mandat-text' : 'text-mandat-muted',
            )}
          >
            {value === 'file' ? 'Файл' : 'Посилання'}
          </button>
        ))}
      </div>

      {mode === 'file' ? (
        <div className="space-y-2">
          {upload.status === 'uploading' ? (
            <div className="space-y-2">
              <div className="flex items-center justify-between text-sm text-mandat-muted">
                <span className="truncate">{upload.fileName}</span>
                <span>{Math.round(upload.progress * 100)}%</span>
              </div>
              <div
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={Math.round(upload.progress * 100)}
                className="h-1.5 bg-mandat-line"
              >
                <div className="h-full bg-mandat-red-bright" style={{ width: `${upload.progress * 100}%` }} />
              </div>
              <Button
                type="button"
                variant="ghost"
                onClick={onCancelUpload}
                className="rounded-none text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
              >
                Скасувати завантаження
              </Button>
            </div>
          ) : videoKey ? (
            <div className="flex items-center justify-between gap-3 border border-mandat-line bg-mandat-bg px-4 py-2">
              <span className="flex min-w-0 items-center gap-2 text-sm">
                <Film className="size-4 shrink-0 text-mandat-red-bright" />
                <span className="truncate">Відео завантажено: {upload.fileName}</span>
              </span>
              {removeButton}
            </div>
          ) : isCurrentFile ? (
            <div className="space-y-2">
              <video src={videoUrl} controls preload="metadata" className="max-h-60 w-full bg-black" />
              {removeButton}
            </div>
          ) : (
            <>
              <Input
                id="news-video-file"
                type="file"
                accept="video/mp4,video/webm"
                onChange={(event) => {
                  const file = event.target.files?.[0]
                  event.target.value = ''
                  if (file) onSelectFile(file)
                }}
                className={cn(inputClassName, 'pt-2.5')}
              />
              <p className="text-xs text-mandat-muted">MP4 або WebM, до 500 МБ</p>
            </>
          )}
          {upload.status === 'error' && upload.error && (
            <p role="alert" className="text-xs text-mandat-red-bright">
              {upload.error}
            </p>
          )}
        </div>
      ) : (
        <div className="space-y-2">
          <div className="relative">
            <Link2 className="absolute left-3 top-3.5 size-4 text-mandat-muted" />
            <Input
              id="news-video-url"
              value={isCurrentFile ? '' : videoUrl}
              onChange={(event) => onLinkChange(event.target.value)}
              placeholder="https://www.youtube.com/watch?v=..."
              aria-invalid={Boolean(error)}
              className={cn(inputClassName, 'pl-10')}
            />
          </div>
          <p className="text-xs text-mandat-muted">Посилання на відео YouTube або Vimeo</p>
          {videoUrl !== '' && removeButton}
        </div>
      )}

      {error && <p className="text-xs text-mandat-red-bright">{error}</p>}
    </div>
  )
}
