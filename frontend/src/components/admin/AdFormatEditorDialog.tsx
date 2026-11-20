import { useEffect, useRef, useState, type FormEvent } from 'react'
import { ImagePlus, LoaderCircle, Trash2 } from 'lucide-react'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { AdFormat, AdFormatFormValues } from '@/types'

// Mirrors the backend limits of an ad format card.
const TITLE_MAX_LENGTH = 80
const DESCRIPTION_MAX_LENGTH = 240

const inputClassName =
  'rounded-none border-mandat-line bg-mandat-bg text-mandat-text placeholder:text-mandat-muted focus-visible:ring-mandat-red-bright/30'

export interface AdFormatEditorDialogProps {
  open: boolean
  /** The card being edited; undefined creates a new one. */
  format?: AdFormat
  saving?: boolean
  error?: string | null
  onOpenChange: (open: boolean) => void
  onSubmit: (values: AdFormatFormValues) => void
}

/** Create/edit dialog of an ad format card: title, description, image upload or removal. */
export default function AdFormatEditorDialog({
  open,
  format,
  saving = false,
  error,
  onOpenChange,
  onSubmit,
}: AdFormatEditorDialogProps) {
  const [title, setTitle] = useState(format?.title ?? '')
  const [description, setDescription] = useState(format?.description ?? '')
  const [image, setImage] = useState<File | null>(null)
  const [removeImage, setRemoveImage] = useState(false)
  const [titleError, setTitleError] = useState<string | null>(null)
  const [imagePreview, setImagePreview] = useState<string | null>(null)
  const previewUrlRef = useRef<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // A picked file previews from a temporary object URL, released on change and on close.
  const pickImage = (file: File | null) => {
    if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current)
    previewUrlRef.current = file ? URL.createObjectURL(file) : null
    setImagePreview(previewUrlRef.current)
    setImage(file)
  }

  useEffect(
    () => () => {
      if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current)
    },
    [],
  )

  const previewSrc = imagePreview ?? (removeImage ? '' : (format?.image_url ?? ''))

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!title.trim()) {
      setTitleError('Вкажіть назву формату.')
      return
    }

    onSubmit({ title: title.trim(), description: description.trim(), image, removeImage: removeImage && !image })
  }

  const clearImage = () => {
    pickImage(null)
    setRemoveImage(true)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !saving && onOpenChange(next)}>
      <DialogContent
        showCloseButton={false}
        className="flex max-h-[92vh] max-w-2xl flex-col gap-0 overflow-hidden rounded-none border-mandat-line bg-mandat-surface p-0 text-mandat-text shadow-2xl shadow-black/50"
      >
        <DialogHeader className="border-b border-mandat-line px-6 py-5 text-left md:px-8">
          <div className="mb-1 text-[11px] font-semibold tracking-[0.2em] text-mandat-red-bright">РЕКЛАМА</div>
          <DialogTitle className="text-2xl font-medium tracking-wide">
            {format ? 'Редагувати формат' : 'Новий формат'}
          </DialogTitle>
          <DialogDescription className="text-mandat-muted">
            Картка з’явиться в блоці «Формати, з якими ми працюємо» на сторінці реклами.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="grid gap-5 overflow-y-auto px-6 py-6 md:px-8">
            <div className="space-y-2">
              <Label htmlFor="ad-format-title" className="text-xs tracking-wide text-mandat-muted">
                НАЗВА
              </Label>
              <Input
                id="ad-format-title"
                value={title}
                maxLength={TITLE_MAX_LENGTH}
                onChange={(event) => {
                  setTitle(event.target.value)
                  setTitleError(null)
                }}
                placeholder="Наприклад, Спецпроєкти"
                aria-invalid={Boolean(titleError)}
                className={`${inputClassName} h-11`}
              />
              {titleError && <p className="text-xs text-mandat-red-bright">{titleError}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="ad-format-description" className="text-xs tracking-wide text-mandat-muted">
                ОПИС
              </Label>
              <Textarea
                id="ad-format-description"
                value={description}
                maxLength={DESCRIPTION_MAX_LENGTH}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="Коротко, що отримує рекламодавець"
                className={`${inputClassName} min-h-24 resize-none`}
              />
              <p className="text-right text-xs text-mandat-muted">
                {description.length}/{DESCRIPTION_MAX_LENGTH}
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="ad-format-image" className="text-xs tracking-wide text-mandat-muted">
                ЗОБРАЖЕННЯ
              </Label>
              {previewSrc && (
                <img
                  src={previewSrc}
                  alt="Зображення формату"
                  className="aspect-[4/3] w-full max-w-sm border border-mandat-line object-cover"
                />
              )}
              <div className="flex flex-wrap items-center gap-2">
                <label
                  htmlFor="ad-format-image"
                  className="inline-flex h-10 cursor-pointer items-center gap-2 border border-mandat-line px-4 text-sm text-mandat-text transition-colors hover:border-mandat-red-bright"
                >
                  <ImagePlus className="size-4" aria-hidden="true" />
                  {previewSrc ? 'Замінити зображення' : 'Завантажити зображення'}
                </label>
                <input
                  ref={fileInputRef}
                  id="ad-format-image"
                  type="file"
                  accept="image/*"
                  className="sr-only"
                  onChange={(event) => {
                    const file = event.target.files?.[0] ?? null
                    pickImage(file)
                    if (file) setRemoveImage(false)
                  }}
                />
                {previewSrc && (
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={clearImage}
                    className="h-10 rounded-none text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-red-bright"
                  >
                    <Trash2 className="size-4" aria-hidden="true" />
                    Прибрати зображення
                  </Button>
                )}
              </div>
              <p className="text-xs text-mandat-muted">JPG, PNG, WebP або GIF, до 10 МБ.</p>
            </div>

            {error && <p className="text-sm text-mandat-red-bright">{error}</p>}
          </div>

          <DialogFooter className="border-t border-mandat-line px-6 py-4 md:px-8">
            <Button
              type="button"
              variant="ghost"
              disabled={saving}
              onClick={() => onOpenChange(false)}
              className="rounded-none text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
            >
              Скасувати
            </Button>
            <Button
              type="submit"
              disabled={saving}
              className="rounded-none bg-mandat-red px-6 text-white hover:bg-mandat-red-bright"
            >
              {saving && <LoaderCircle className="size-4 animate-spin" />}
              Зберегти
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
