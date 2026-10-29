import { useState, type FormEvent } from 'react'
import { Image, LoaderCircle, Star } from 'lucide-react'

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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { validateNewsPayload, type NewsValidationErrors } from '@/lib/news-validation'
import type { AdminNews, Category, NewsFormValues } from '@/types'

const categories: Category[] = ['Новини', 'Міжнародні новини', 'Спорт', 'Інтерв\'ю', 'Про нас']

const emptyNews: NewsFormValues = {
  category: 'Новини',
  title: '',
  description: '',
  text: '',
  language: 'uk',
  status: 'published',
  featured: false,
  image_url: '',
}

const inputClassName =
  'h-11 rounded-none border-mandat-line bg-mandat-bg text-mandat-text placeholder:text-mandat-muted focus-visible:ring-mandat-red-bright/30'

interface NewsEditorDialogProps {
  article?: AdminNews
  busy?: boolean
  error?: string | null
  onClose: () => void
  onSave: (values: NewsFormValues) => void
}

export default function NewsEditorDialog({ article, busy = false, error, onClose, onSave }: NewsEditorDialogProps) {
  const [values, setValues] = useState<NewsFormValues>(() =>
    article
      ? {
          category: article.category,
          title: article.title,
          description: article.description,
          text: article.text,
          language: article.language,
          status: 'published',
          featured: article.featured,
          image_url: article.image_url,
        }
      : emptyNews,
  )

  const [errors, setErrors] = useState<NewsValidationErrors>({})

  const update = <K extends keyof NewsFormValues>(key: K, value: NewsFormValues[K]) => {
    setValues((current) => ({ ...current, [key]: value }))
    setErrors((current) => {
      const next = { ...current }
      delete next[key]
      return next
    })
  }

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const nextErrors = validateNewsPayload(values)
    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors)
      return
    }

    onSave(values)
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent
        showCloseButton={false}
        className="flex max-h-[92vh] max-w-3xl flex-col gap-0 overflow-hidden rounded-none border-mandat-line bg-mandat-surface p-0 text-mandat-text shadow-2xl shadow-black/50"
      >
        <DialogHeader className="border-b border-mandat-line px-6 py-5 text-left md:px-8">
          <div className="mb-1 text-[11px] font-semibold tracking-[0.2em] text-mandat-red-bright">
            {article ? 'РЕДАГУВАННЯ МАТЕРІАЛУ' : 'НОВИЙ МАТЕРІАЛ'}
          </div>
          <DialogTitle className="text-2xl font-medium tracking-wide">
            {article ? 'Редагувати новину' : 'Додати новину'}
          </DialogTitle>
          <DialogDescription className="text-mandat-muted">
            Заповніть інформацію про матеріал. Після збереження новина буде опублікована.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="grid gap-5 overflow-y-auto px-6 py-6 md:grid-cols-2 md:px-8">
            <div className="space-y-2 md:col-span-2">
              <Label htmlFor="news-title" className="text-xs tracking-wide text-mandat-muted">
                ЗАГОЛОВОК
              </Label>
              <Input
                id="news-title"
                value={values.title}
                onChange={(event) => update('title', event.target.value)}
                placeholder="Введіть заголовок новини"
                aria-invalid={Boolean(errors.title)}
                className={inputClassName}
              />
              {errors.title && <p className="text-xs text-mandat-red-bright">{errors.title}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="news-category" className="text-xs tracking-wide text-mandat-muted">РУБРИКА</Label>
              <Select value={values.category} onValueChange={(value) => update('category', value as Category)}>
                <SelectTrigger id="news-category" aria-invalid={Boolean(errors.category)} className={`${inputClassName} w-full`}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className="rounded-none border-mandat-line bg-mandat-surface text-mandat-text">
                  {categories.map((category) => (
                    <SelectItem key={category} value={category}>
                      {category}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {errors.category && <p className="text-xs text-mandat-red-bright">{errors.category}</p>}
            </div>

            <div className="space-y-2 md:col-span-2">
              <Label htmlFor="news-description" className="text-xs tracking-wide text-mandat-muted">
                КОРОТКИЙ ОПИС
              </Label>
              <Textarea
                id="news-description"
                value={values.description}
                onChange={(event) => update('description', event.target.value)}
                placeholder="Короткий анонс для картки новини"
                className={`${inputClassName} min-h-20 resize-none`}
              />
            </div>

            <div className="space-y-2 md:col-span-2">
              <Label htmlFor="news-text" className="text-xs tracking-wide text-mandat-muted">
                ТЕКСТ НОВИНИ
              </Label>
              <Textarea
                id="news-text"
                value={values.text}
                onChange={(event) => update('text', event.target.value)}
                placeholder="Основний текст матеріалу..."
                aria-invalid={Boolean(errors.text)}
                className={`${inputClassName} min-h-40 resize-y`}
              />
              {errors.text && <p className="text-xs text-mandat-red-bright">{errors.text}</p>}
            </div>

            <div className="space-y-2 md:col-span-2">
              <Label htmlFor="news-image" className="text-xs tracking-wide text-mandat-muted">
                ПОСИЛАННЯ НА ОБКЛАДИНКУ
              </Label>
              <div className="relative">
                <Image className="absolute left-3 top-3.5 size-4 text-mandat-muted" />
                <Input
                  id="news-image"
                  value={values.image_url}
                  onChange={(event) => update('image_url', event.target.value)}
                  placeholder="https://..."
                  aria-invalid={Boolean(errors.image_url)}
                  className={`${inputClassName} pl-10`}
                />
              </div>
              {errors.image_url && <p className="text-xs text-mandat-red-bright">{errors.image_url}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="news-language" className="text-xs tracking-wide text-mandat-muted">МОВА</Label>
              <Select value={values.language} onValueChange={(value) => update('language', value)}>
                <SelectTrigger id="news-language" className={`${inputClassName} w-full`}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className="rounded-none border-mandat-line bg-mandat-surface text-mandat-text">
                  <SelectItem value="uk">Українська</SelectItem>
                  <SelectItem value="en">English</SelectItem>
                </SelectContent>
              </Select>
              {errors.language && <p className="text-xs text-mandat-red-bright">{errors.language}</p>}
            </div>

            <div className="flex items-center justify-between border border-mandat-line bg-mandat-bg px-4">
              <Label htmlFor="news-featured" className="flex cursor-pointer items-center gap-2 text-sm">
                <Star className="size-4 text-mandat-red-bright" />
                На головній
              </Label>
              <Switch
                id="news-featured"
                checked={values.featured}
                onCheckedChange={(checked) => update('featured', checked)}
                className="data-[state=checked]:bg-mandat-red-bright data-[state=unchecked]:bg-mandat-line"
              />
            </div>
          </div>

          {error && (
            <div role="alert" className="border-t border-mandat-red/40 bg-mandat-red/10 px-6 py-3 text-sm text-mandat-red-bright md:px-8">
              {error}
            </div>
          )}

          <DialogFooter className="border-t border-mandat-line px-6 py-4 md:px-8">
            <Button
              type="button"
              variant="ghost"
              disabled={busy}
              onClick={onClose}
              className="rounded-none text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
            >
              Скасувати
            </Button>
            <Button
              type="submit"
              disabled={busy}
              className="rounded-none bg-mandat-red px-6 text-white hover:bg-mandat-red-bright"
            >
              {busy && <LoaderCircle className="size-4 animate-spin" />}
              {busy ? 'Збереження...' : article ? 'Зберегти зміни' : 'Створити новину'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
