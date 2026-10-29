import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import {
  ArrowLeft,
  ChevronLeft,
  ChevronRight,
  FileEdit,
  FileText,
  LayoutDashboard,
  LoaderCircle,
  LogOut,
  Newspaper,
  Plus,
  RefreshCw,
  Search,
  Star,
  Trash2,
} from 'lucide-react'

import {
  createNews,
  deleteNews,
  getNewsErrorMessage,
  listNews,
  updateNews,
} from '@/api/news'
import NewsEditorDialog from '@/components/admin/NewsEditorDialog'
import useAuth from '@/hooks/use-auth'
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
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { AdminNews, NewsFormValues, NewsStatus } from '@/types'

const PAGE_SIZE = 20

const dateFormatter = new Intl.DateTimeFormat('uk-UA', {
  day: '2-digit',
  month: 'short',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

const viewsFormatter = new Intl.NumberFormat('uk-UA', {
  notation: 'compact',
  maximumFractionDigits: 1,
})

type StatusFilter = 'all' | NewsStatus

export default function Admin() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [articles, setArticles] = useState<AdminNews[]>([])
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState<StatusFilter>('all')
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingArticle, setEditingArticle] = useState<AdminNews | undefined>()
  const [saving, setSaving] = useState(false)
  const [mutationError, setMutationError] = useState<string | null>(null)
  const [deletingArticle, setDeletingArticle] = useState<AdminNews | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const requestId = useRef(0)

  const loadNews = useCallback(async () => {
    const currentRequest = ++requestId.current
    setLoading(true)
    setLoadError(null)

    try {
      const result = await listNews(page, PAGE_SIZE, status === 'all' ? undefined : status)
      if (currentRequest === requestId.current) setArticles(result.items)
    } catch (error) {
      if (currentRequest === requestId.current) {
        setArticles([])
        setLoadError(getNewsErrorMessage(error))
      }
    } finally {
      if (currentRequest === requestId.current) setLoading(false)
    }
  }, [page, status])

  useEffect(() => {
    void loadNews()
  }, [loadNews])

  const visibleArticles = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase('uk')
    if (!normalizedQuery) return articles

    return articles.filter((article) =>
      article.title.toLocaleLowerCase('uk').includes(normalizedQuery) ||
      article.slug.toLocaleLowerCase('uk').includes(normalizedQuery),
    )
  }, [articles, query])

  const publishedCount = articles.filter((article) => article.status === 'published').length
  const draftCount = articles.length - publishedCount
  const featuredCount = articles.filter((article) => article.featured).length
  const hasNextPage = articles.length === PAGE_SIZE

  const openCreateDialog = () => {
    setEditingArticle(undefined)
    setMutationError(null)
    setEditorOpen(true)
  }

  const openEditDialog = (article: AdminNews) => {
    setEditingArticle(article)
    setMutationError(null)
    setEditorOpen(true)
  }

  const saveArticle = async (values: NewsFormValues) => {
    setSaving(true)
    setMutationError(null)

    try {
      if (editingArticle) {
        await updateNews(editingArticle.slug, values)
        setEditorOpen(false)
        await loadNews()
      } else {
        await createNews(values)
        setEditorOpen(false)
        setQuery('')
        const alreadyOnFirstPage = page === 1 && status === 'all'
        setPage(1)
        setStatus('all')
        if (alreadyOnFirstPage) await loadNews()
      }
    } catch (error) {
      setMutationError(getNewsErrorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  const openDeleteDialog = (article: AdminNews) => {
    setDeleteError(null)
    setDeletingArticle(article)
  }

  const deleteArticle = async () => {
    if (!deletingArticle) return
    setDeleting(true)
    setDeleteError(null)

    try {
      await deleteNews(deletingArticle.slug)
      setDeletingArticle(null)
      if (articles.length === 1 && page > 1) {
        setPage(page - 1)
      } else {
        await loadNews()
      }
    } catch (error) {
      setDeleteError(getNewsErrorMessage(error))
    } finally {
      setDeleting(false)
    }
  }

  const handleLogout = async () => {
    await logout().catch(() => undefined)
    navigate('/admin/login', { replace: true })
  }

  const stats = [
    { label: 'Новин на сторінці', value: articles.length.toString(), icon: Newspaper },
    { label: 'Опубліковано на сторінці', value: publishedCount.toString(), icon: FileText },
    { label: 'Чернетки на сторінці', value: draftCount.toString(), icon: FileEdit },
  ]

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <div className="flex min-h-screen">
        <aside className="sticky top-0 hidden h-screen w-64 shrink-0 flex-col border-r border-mandat-line bg-mandat-surface/40 lg:flex">
          <div className="flex h-20 items-center gap-3 border-b border-mandat-line px-6">
            <div className="flex size-9 items-center justify-center bg-mandat-red oswald text-xl font-bold text-white">
              М
            </div>
            <div>
              <div className="oswald text-xl font-semibold leading-none tracking-[0.14em]">МАНДАТ</div>
              <div className="mt-1 text-[9px] tracking-[0.25em] text-mandat-muted">АДМІНПАНЕЛЬ</div>
            </div>
          </div>

          <nav className="flex-1 px-3 py-6">
            <div className="mb-3 px-3 text-[10px] font-medium tracking-[0.2em] text-mandat-muted">
              РОБОЧА ОБЛАСТЬ
            </div>
            <div className="flex items-center gap-3 border-l-2 border-mandat-red-bright bg-mandat-red/15 px-3 py-3 text-sm font-medium text-white">
              <Newspaper className="size-4 text-mandat-red-bright" />
              Керування новинами
            </div>
          </nav>

          <div className="border-t border-mandat-line p-4">
            <Link
              to="/"
              className="flex items-center gap-2 px-2 py-2 text-sm text-mandat-muted transition-colors hover:text-mandat-text"
            >
              <ArrowLeft className="size-4" />
              Повернутися на сайт
            </Link>
            <div className="mt-4 px-2 text-[10px] tracking-wider text-mandat-muted/60">МАНДАТ CMS · v1.0</div>
          </div>
        </aside>

        <div className="min-w-0 flex-1">
          <header className="flex h-20 items-center justify-between border-b border-mandat-line bg-mandat-surface/50 px-4 md:px-8">
            <div className="flex items-center gap-3 lg:hidden">
              <div className="flex size-8 items-center justify-center bg-mandat-red oswald font-bold text-white">М</div>
              <span className="oswald text-lg tracking-widest">МАНДАТ</span>
            </div>
            <div className="hidden items-center gap-2 text-sm text-mandat-muted lg:flex">
              <LayoutDashboard className="size-4" />
              <span>Адмінпанель</span>
              <span className="text-mandat-line">/</span>
              <span className="text-mandat-text">Новини</span>
            </div>
            <div className="flex items-center gap-3">
              <div className="hidden text-right sm:block">
                <div className="text-sm font-medium">{user?.login}</div>
                <div className="text-xs text-mandat-muted">
                  {user?.role === 'admin' ? 'Адміністратор' : 'Модератор'}
                </div>
              </div>
              <div className="flex size-9 items-center justify-center rounded-full border border-mandat-red/50 bg-mandat-red/15 text-xs font-semibold uppercase text-mandat-red-bright">
                {user?.login.slice(0, 2)}
              </div>
              <Button
                variant="ghost"
                size="icon-sm"
                title="Вийти"
                aria-label="Вийти з адмінпанелі"
                onClick={() => void handleLogout()}
                className="rounded-none text-mandat-muted hover:bg-mandat-red/15 hover:text-mandat-red-bright"
              >
                <LogOut className="size-4" />
              </Button>
            </div>
          </header>

          <main className="mx-auto max-w-[1600px] p-4 md:p-8">
            <div className="mb-8 flex flex-col justify-between gap-5 sm:flex-row sm:items-end">
              <div>
                <div className="mb-2 text-[11px] font-semibold tracking-[0.22em] text-mandat-red-bright">
                  КОНТЕНТ / НОВИНИ
                </div>
                <h1 className="text-3xl font-medium tracking-wide md:text-4xl">Керування новинами</h1>
                <p className="mt-2 text-sm text-mandat-muted">
                  Створюйте, редагуйте та публікуйте матеріали редакції.
                </p>
              </div>
              <Button
                onClick={openCreateDialog}
                className="h-11 rounded-none bg-mandat-red px-5 text-white shadow-lg shadow-mandat-red/10 hover:bg-mandat-red-bright"
              >
                <Plus className="size-4" />
                Додати новину
              </Button>
            </div>

            <section aria-label="Статистика" className="mb-6 grid grid-cols-1 gap-3 sm:grid-cols-3">
              {stats.map(({ label, value, icon: Icon }, index) => (
                <div
                  key={label}
                  className="relative overflow-hidden border border-mandat-line bg-mandat-surface p-4 md:p-5"
                >
                  {index === 0 && <div className="absolute inset-y-0 left-0 w-0.5 bg-mandat-red-bright" />}
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <div className="text-xs text-mandat-muted md:text-sm">{label}</div>
                      <div className="mt-2 oswald text-2xl font-medium tracking-wide md:text-3xl">{value}</div>
                    </div>
                    <div className="flex size-9 items-center justify-center border border-mandat-line bg-mandat-bg text-mandat-muted">
                      <Icon className="size-4" />
                    </div>
                  </div>
                </div>
              ))}
            </section>

            <section className="border border-mandat-line bg-mandat-surface">
              <div className="flex flex-col gap-3 border-b border-mandat-line p-4 md:flex-row md:items-center md:justify-between">
                <div className="relative max-w-md flex-1">
                  <Search className="absolute left-3 top-3 size-4 text-mandat-muted" />
                  <Input
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    aria-label="Пошук новин на поточній сторінці"
                    placeholder="Пошук на поточній сторінці..."
                    className="h-10 rounded-none border-mandat-line bg-mandat-bg pl-10 text-mandat-text placeholder:text-mandat-muted focus-visible:ring-mandat-red-bright/30"
                  />
                </div>
                <div className="flex gap-2">
                  <Select
                    value={status}
                    onValueChange={(value) => {
                      setStatus(value as StatusFilter)
                      setPage(1)
                    }}
                  >
                    <SelectTrigger aria-label="Фільтр за статусом" className="h-10 flex-1 rounded-none border-mandat-line bg-mandat-bg text-mandat-text md:w-48 md:flex-none">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent className="rounded-none border-mandat-line bg-mandat-surface text-mandat-text">
                      <SelectItem value="all">Усі статуси</SelectItem>
                      <SelectItem value="published">Опубліковано</SelectItem>
                      <SelectItem value="draft">Чернетки</SelectItem>
                    </SelectContent>
                  </Select>
                  <Button
                    variant="outline"
                    size="icon"
                    title="Оновити список"
                    disabled={loading}
                    onClick={() => void loadNews()}
                    className="size-10 rounded-none border-mandat-line bg-mandat-bg text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
                  >
                    <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
                  </Button>
                </div>
              </div>

              <div className="overflow-x-auto">
                <Table className="min-w-[920px]">
                  <TableHeader>
                    <TableRow className="border-mandat-line bg-mandat-bg/50 hover:bg-mandat-bg/50">
                      <TableHead className="px-5 text-[10px] tracking-[0.16em] text-mandat-muted">НОВИНА</TableHead>
                      <TableHead className="text-[10px] tracking-[0.16em] text-mandat-muted">РУБРИКА</TableHead>
                      <TableHead className="text-[10px] tracking-[0.16em] text-mandat-muted">СТАТУС</TableHead>
                      <TableHead className="text-[10px] tracking-[0.16em] text-mandat-muted">ДАТА</TableHead>
                      <TableHead className="text-right text-[10px] tracking-[0.16em] text-mandat-muted">ПЕРЕГЛЯДИ</TableHead>
                      <TableHead className="w-28 text-right text-[10px] tracking-[0.16em] text-mandat-muted">ДІЇ</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {visibleArticles.map((article) => (
                      <TableRow key={article.id} className="group border-mandat-line hover:bg-mandat-surface-2/50">
                        <TableCell className="max-w-md px-5 py-4">
                          <div className="flex items-start gap-2">
                            <div className="min-w-0">
                              <div className="line-clamp-1 font-medium text-mandat-text">{article.title}</div>
                              <div className="mt-1 truncate font-mono text-[11px] text-mandat-muted">/{article.slug}</div>
                            </div>
                            {article.featured && (
                              <Star className="mt-0.5 size-3.5 shrink-0 fill-mandat-red-bright text-mandat-red-bright" />
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <span className="border border-mandat-line bg-mandat-bg px-2.5 py-1 text-xs text-mandat-muted">
                            {article.category}
                          </span>
                        </TableCell>
                        <TableCell>
                          <span
                            className={`inline-flex items-center gap-2 text-xs ${
                              article.status === 'published' ? 'text-mandat-text' : 'text-mandat-muted'
                            }`}
                          >
                            <span
                              className={`size-1.5 rounded-full ${
                                article.status === 'published' ? 'bg-mandat-red-bright' : 'bg-mandat-muted'
                              }`}
                            />
                            {article.status === 'published' ? 'Опубліковано' : 'Чернетка'}
                          </span>
                        </TableCell>
                        <TableCell className="whitespace-nowrap text-xs text-mandat-muted">
                          {dateFormatter.format(new Date(article.published_at ?? article.updated_at))}
                        </TableCell>
                        <TableCell className="text-right text-sm text-mandat-muted">
                          {viewsFormatter.format(article.views)}
                        </TableCell>
                        <TableCell>
                          <div className="flex justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              title="Редагувати"
                              onClick={() => openEditDialog(article)}
                              className="rounded-none text-mandat-muted hover:bg-mandat-red/15 hover:text-mandat-red-bright"
                            >
                              <FileEdit className="size-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              title="Видалити"
                              onClick={() => openDeleteDialog(article)}
                              className="rounded-none text-mandat-muted hover:bg-mandat-red/15 hover:text-mandat-red-bright"
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
                    Завантаження новин...
                  </div>
                )}

                {loadError && !loading && (
                  <div role="alert" className="flex flex-col items-center border-t border-mandat-line px-4 py-12 text-center">
                    <h2 className="text-lg">Не вдалося завантажити новини</h2>
                    <p className="mt-2 max-w-lg text-sm text-mandat-muted">{loadError}</p>
                    <Button
                      variant="outline"
                      onClick={() => void loadNews()}
                      className="mt-5 rounded-none border-mandat-line bg-mandat-bg text-mandat-text hover:bg-mandat-surface-2"
                    >
                      Спробувати ще раз
                    </Button>
                  </div>
                )}

                {!loading && !loadError && visibleArticles.length === 0 && (
                  <div className="flex flex-col items-center border-t border-mandat-line px-4 py-16 text-center">
                    <div className="mb-4 flex size-12 items-center justify-center border border-mandat-line bg-mandat-bg">
                      <Search className="size-5 text-mandat-muted" />
                    </div>
                    <h2 className="text-lg">Новин не знайдено</h2>
                    <p className="mt-2 text-sm text-mandat-muted">Спробуйте змінити пошуковий запит або фільтр.</p>
                  </div>
                )}
              </div>

              <div className="flex flex-col gap-3 border-t border-mandat-line px-4 py-4 text-xs text-mandat-muted sm:flex-row sm:items-center sm:justify-between">
                <div>
                  {query
                    ? `Знайдено на цій сторінці: ${visibleArticles.length}`
                    : `На сторінці: ${articles.length}`}
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page === 1 || loading}
                    onClick={() => setPage((current) => current - 1)}
                    className="rounded-none border-mandat-line bg-mandat-bg text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
                  >
                    <ChevronLeft className="size-4" />
                    Назад
                  </Button>
                  <span className="px-2">Сторінка {page}</span>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={!hasNextPage || loading}
                    onClick={() => setPage((current) => current + 1)}
                    className="rounded-none border-mandat-line bg-mandat-bg text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
                  >
                    Далі
                    <ChevronRight className="size-4" />
                  </Button>
                </div>
              </div>
            </section>

            <div className="mt-5 flex items-center gap-2 text-xs text-mandat-muted">
              <Star className="size-3.5 text-mandat-red-bright" />
              Матеріалів на головній на цій сторінці: {featuredCount}
            </div>
          </main>
        </div>
      </div>

      {editorOpen && (
        <NewsEditorDialog
          key={editingArticle?.id ?? 'new'}
          article={editingArticle}
          busy={saving}
          error={mutationError}
          onClose={() => setEditorOpen(false)}
          onSave={saveArticle}
        />
      )}

      <AlertDialog open={Boolean(deletingArticle)} onOpenChange={(open) => !open && !deleting && setDeletingArticle(null)}>
        <AlertDialogContent className="rounded-none border-mandat-line bg-mandat-surface text-mandat-text">
          <AlertDialogHeader>
            <div className="mb-2 flex size-10 items-center justify-center bg-mandat-red/15 text-mandat-red-bright">
              <Trash2 className="size-5" />
            </div>
            <AlertDialogTitle className="text-xl">Видалити новину?</AlertDialogTitle>
            <AlertDialogDescription className="text-mandat-muted">
              «{deletingArticle?.title}» буде видалено зі списку. Цю дію неможливо скасувати.
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
                void deleteArticle()
              }}
              className="rounded-none bg-mandat-red text-white hover:bg-mandat-red-bright"
            >
              {deleting && <LoaderCircle className="size-4 animate-spin" />}
              {deleting ? 'Видалення...' : 'Видалити'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
