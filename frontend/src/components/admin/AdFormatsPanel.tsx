import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowDown, ArrowUp, FileEdit, ImageOff, LoaderCircle, Megaphone, Plus, RefreshCw, Trash2 } from 'lucide-react'

import {
  createAdFormat,
  deleteAdFormat,
  listAdminAdFormats,
  moveAdFormat,
  updateAdFormat,
} from '@/api/adFormats'
import AdFormatEditorDialog from '@/components/admin/AdFormatEditorDialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { AdFormat, AdFormatFormValues, AdFormatMoveDirection } from '@/types'

const iconButtonClassName = 'rounded-none text-mandat-muted hover:bg-mandat-red/15 hover:text-mandat-red-bright'

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Сталася невідома помилка.'
}

/** Admin table of the advertising page's format cards: add, edit, delete, reorder. */
export default function AdFormatsPanel() {
  const [formats, setFormats] = useState<AdFormat[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingFormat, setEditingFormat] = useState<AdFormat | undefined>()
  const [saving, setSaving] = useState(false)
  const [mutationError, setMutationError] = useState<string | null>(null)
  const [movingId, setMovingId] = useState<string | null>(null)
  const [moveError, setMoveError] = useState<string | null>(null)
  const [deletingFormat, setDeletingFormat] = useState<AdFormat | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const requestId = useRef(0)

  const loadFormats = useCallback(async () => {
    const currentRequest = ++requestId.current
    setLoading(true)
    setLoadError(null)

    try {
      const items = await listAdminAdFormats()
      if (currentRequest === requestId.current) setFormats(items)
    } catch (error) {
      if (currentRequest === requestId.current) {
        setFormats([])
        setLoadError(errorMessage(error))
      }
    } finally {
      if (currentRequest === requestId.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadFormats()
  }, [loadFormats])

  const openEditor = (format?: AdFormat) => {
    setEditingFormat(format)
    setMutationError(null)
    setEditorOpen(true)
  }

  const saveFormat = async (values: AdFormatFormValues) => {
    setSaving(true)
    setMutationError(null)

    try {
      if (editingFormat) {
        await updateAdFormat(editingFormat.id, values)
      } else {
        await createAdFormat(values)
      }
      setEditorOpen(false)
      await loadFormats()
    } catch (error) {
      setMutationError(errorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  const move = async (format: AdFormat, direction: AdFormatMoveDirection) => {
    setMovingId(format.id)
    setMoveError(null)

    try {
      await moveAdFormat(format.id, direction)
      await loadFormats()
    } catch (error) {
      setMoveError(errorMessage(error))
    } finally {
      setMovingId(null)
    }
  }

  const removeFormat = async () => {
    if (!deletingFormat) return
    setDeleting(true)
    setDeleteError(null)

    try {
      await deleteAdFormat(deletingFormat.id)
      setDeletingFormat(null)
      await loadFormats()
    } catch (error) {
      setDeleteError(errorMessage(error))
    } finally {
      setDeleting(false)
    }
  }

  return (
    <>
      <div className="mb-8 flex flex-col justify-between gap-5 sm:flex-row sm:items-end">
        <div>
          <div className="mb-2 text-[11px] font-semibold tracking-[0.22em] text-mandat-red-bright">
            КОНТЕНТ / РЕКЛАМА
          </div>
          <h1 className="text-3xl font-medium tracking-wide md:text-4xl">Керування рекламою</h1>
          <p className="mt-2 text-sm text-mandat-muted">
            Картки блоку «Формати, з якими ми працюємо» на сторінці реклами.
          </p>
        </div>
        <Button
          onClick={() => openEditor()}
          className="h-11 rounded-none bg-mandat-red px-5 text-white shadow-lg shadow-mandat-red/10 hover:bg-mandat-red-bright"
        >
          <Plus className="size-4" />
          Додати формат
        </Button>
      </div>

      <section className="border border-mandat-line bg-mandat-surface">
        <div className="flex flex-col gap-3 border-b border-mandat-line p-4 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-2 text-sm text-mandat-muted">
            <Megaphone className="size-4 text-mandat-red-bright" />
            Форматів: {formats.length}
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="icon"
              title="Оновити список"
              aria-label="Оновити список"
              disabled={loading}
              onClick={() => void loadFormats()}
              className="size-10 rounded-none border-mandat-line bg-mandat-bg text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
            >
              <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
            </Button>
          </div>
        </div>

        {moveError && (
          <p role="alert" className="border-b border-mandat-line bg-mandat-red/10 px-4 py-3 text-sm text-mandat-red-bright">
            {moveError}
          </p>
        )}

        <div className="overflow-x-auto">
          <Table className="min-w-[760px]">
            <TableHeader>
              <TableRow className="border-mandat-line bg-mandat-bg/50 hover:bg-mandat-bg/50">
                <TableHead className="w-28 px-5 text-[10px] tracking-[0.16em] text-mandat-muted">ПОРЯДОК</TableHead>
                <TableHead className="w-28 text-[10px] tracking-[0.16em] text-mandat-muted">ЗОБРАЖЕННЯ</TableHead>
                <TableHead className="text-[10px] tracking-[0.16em] text-mandat-muted">ФОРМАТ</TableHead>
                <TableHead className="w-28 text-right text-[10px] tracking-[0.16em] text-mandat-muted">ДІЇ</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {formats.map((format, index) => (
                <TableRow key={format.id} className="border-mandat-line hover:bg-mandat-surface-2/50">
                  <TableCell className="px-5 py-4">
                    <div className="flex items-center gap-1">
                      <span className="w-6 text-sm tabular-nums text-mandat-muted">{index + 1}</span>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        title="Вище"
                        aria-label={`Перемістити «${format.title}» вище`}
                        disabled={index === 0 || movingId !== null}
                        onClick={() => void move(format, 'up')}
                        className={iconButtonClassName}
                      >
                        <ArrowUp className="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        title="Нижче"
                        aria-label={`Перемістити «${format.title}» нижче`}
                        disabled={index === formats.length - 1 || movingId !== null}
                        onClick={() => void move(format, 'down')}
                        className={iconButtonClassName}
                      >
                        <ArrowDown className="size-4" />
                      </Button>
                    </div>
                  </TableCell>
                  <TableCell>
                    {format.image_url ? (
                      <img
                        src={format.image_url}
                        alt=""
                        className="h-14 w-20 border border-mandat-line object-cover"
                      />
                    ) : (
                      <div
                        className="flex h-14 w-20 items-center justify-center border border-mandat-line bg-gradient-to-br from-mandat-red/60 to-mandat-surface-2"
                        title="Без зображення"
                      >
                        <ImageOff className="size-4 text-white/60" />
                      </div>
                    )}
                  </TableCell>
                  <TableCell className="max-w-xl py-4">
                    <div className="font-medium text-mandat-text">{format.title}</div>
                    {format.description && (
                      <div className="mt-1 line-clamp-2 text-xs text-mandat-muted">{format.description}</div>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        title="Редагувати"
                        aria-label={`Редагувати «${format.title}»`}
                        onClick={() => openEditor(format)}
                        className={iconButtonClassName}
                      >
                        <FileEdit className="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        title="Видалити"
                        aria-label={`Видалити «${format.title}»`}
                        onClick={() => {
                          setDeleteError(null)
                          setDeletingFormat(format)
                        }}
                        className={iconButtonClassName}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          {loading && (
            <div role="status" className="flex items-center justify-center gap-3 border-t border-mandat-line px-4 py-12 text-sm text-mandat-muted">
              <LoaderCircle className="size-5 animate-spin text-mandat-red-bright" />
              Завантаження форматів...
            </div>
          )}

          {loadError && !loading && (
            <div role="alert" className="flex flex-col items-center border-t border-mandat-line px-4 py-12 text-center">
              <h2 className="text-lg">Не вдалося завантажити формати</h2>
              <p className="mt-2 max-w-lg text-sm text-mandat-muted">{loadError}</p>
              <Button
                variant="outline"
                onClick={() => void loadFormats()}
                className="mt-5 rounded-none border-mandat-line bg-mandat-bg text-mandat-text hover:bg-mandat-surface-2"
              >
                Спробувати ще раз
              </Button>
            </div>
          )}

          {!loading && !loadError && formats.length === 0 && (
            <div className="flex flex-col items-center border-t border-mandat-line px-4 py-16 text-center">
              <h2 className="text-lg">Форматів ще немає</h2>
              <p className="mt-2 text-sm text-mandat-muted">Додайте перший формат реклами.</p>
            </div>
          )}
        </div>
      </section>

      {editorOpen && (
        <AdFormatEditorDialog
          key={editingFormat?.id ?? 'new'}
          open
          format={editingFormat}
          saving={saving}
          error={mutationError}
          onOpenChange={(open) => !open && setEditorOpen(false)}
          onSubmit={(values) => void saveFormat(values)}
        />
      )}

      <AlertDialog open={Boolean(deletingFormat)} onOpenChange={(open) => !open && !deleting && setDeletingFormat(null)}>
        <AlertDialogContent className="rounded-none border-mandat-line bg-mandat-surface text-mandat-text">
          <AlertDialogHeader>
            <div className="mb-2 flex size-10 items-center justify-center bg-mandat-red/15 text-mandat-red-bright">
              <Trash2 className="size-5" />
            </div>
            <AlertDialogTitle className="text-xl">Видалити формат?</AlertDialogTitle>
            <AlertDialogDescription className="text-mandat-muted">
              «{deletingFormat?.title}» зникне зі сторінки реклами разом із зображенням. Цю дію неможливо скасувати.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {deleteError && <p role="alert" className="bg-mandat-red/10 p-3 text-sm text-mandat-red-bright">{deleteError}</p>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting} className="rounded-none border-mandat-line bg-mandat-bg text-mandat-text hover:bg-mandat-surface-2 hover:text-mandat-text">
              Скасувати
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={(event) => {
                event.preventDefault()
                void removeFormat()
              }}
              className="rounded-none bg-mandat-red text-white hover:bg-mandat-red-bright"
            >
              {deleting && <LoaderCircle className="size-4 animate-spin" />}
              {deleting ? 'Видалення...' : 'Видалити'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
