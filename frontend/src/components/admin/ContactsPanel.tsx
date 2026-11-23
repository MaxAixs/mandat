import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowDown, ArrowUp, ContactRound, FileEdit, LoaderCircle, Plus, RefreshCw, Trash2 } from 'lucide-react'

import { createContact, deleteContact, listAdminContacts, moveContact, updateContact } from '@/api/contacts'
import ContactIcon from '@/components/ContactIcon'
import ContactEditorDialog from '@/components/admin/ContactEditorDialog'
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { CONTACT_TYPE_LABELS } from '@/constants/contacts'
import type { Contact, ContactFormValues, ContactMoveDirection } from '@/types'

const iconButtonClassName = 'rounded-none text-mandat-muted hover:bg-mandat-red/15 hover:text-mandat-red-bright'

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Сталася невідома помилка.'
}

/** The name a contact goes by in the admin: its label, or the type name without one. */
function contactName(contact: Contact): string {
  return contact.label || CONTACT_TYPE_LABELS[contact.type]
}

/** Admin table of the /contacts page contacts: add, edit, delete, reorder. */
export default function ContactsPanel() {
  const [contacts, setContacts] = useState<Contact[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingContact, setEditingContact] = useState<Contact | undefined>()
  const [saving, setSaving] = useState(false)
  const [mutationError, setMutationError] = useState<string | null>(null)
  const [movingId, setMovingId] = useState<string | null>(null)
  const [moveError, setMoveError] = useState<string | null>(null)
  const [deletingContact, setDeletingContact] = useState<Contact | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const requestId = useRef(0)

  const loadContacts = useCallback(async () => {
    const currentRequest = ++requestId.current
    setLoading(true)
    setLoadError(null)

    try {
      const items = await listAdminContacts()
      if (currentRequest === requestId.current) setContacts(items)
    } catch (error) {
      if (currentRequest === requestId.current) {
        setContacts([])
        setLoadError(errorMessage(error))
      }
    } finally {
      if (currentRequest === requestId.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadContacts()
  }, [loadContacts])

  const openEditor = (contact?: Contact) => {
    setEditingContact(contact)
    setMutationError(null)
    setEditorOpen(true)
  }

  const saveContact = async (values: ContactFormValues) => {
    setSaving(true)
    setMutationError(null)

    try {
      if (editingContact) {
        await updateContact(editingContact.id, values)
      } else {
        await createContact(values)
      }
      setEditorOpen(false)
      await loadContacts()
    } catch (error) {
      setMutationError(errorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  const move = async (contact: Contact, direction: ContactMoveDirection) => {
    setMovingId(contact.id)
    setMoveError(null)

    try {
      await moveContact(contact.id, direction)
      await loadContacts()
    } catch (error) {
      setMoveError(errorMessage(error))
    } finally {
      setMovingId(null)
    }
  }

  const removeContact = async () => {
    if (!deletingContact) return
    setDeleting(true)
    setDeleteError(null)

    try {
      await deleteContact(deletingContact.id)
      setDeletingContact(null)
      await loadContacts()
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
            КОНТЕНТ / КОНТАКТИ
          </div>
          <h1 className="text-3xl font-medium tracking-wide md:text-4xl">Керування контактами</h1>
          <p className="mt-2 text-sm text-mandat-muted">Контакти сторінки «Наші контакти».</p>
        </div>
        <Button
          onClick={() => openEditor()}
          className="h-11 rounded-none bg-mandat-red px-5 text-white shadow-lg shadow-mandat-red/10 hover:bg-mandat-red-bright"
        >
          <Plus className="size-4" />
          Додати контакт
        </Button>
      </div>

      <section className="border border-mandat-line bg-mandat-surface">
        <div className="flex flex-col gap-3 border-b border-mandat-line p-4 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-2 text-sm text-mandat-muted">
            <ContactRound className="size-4 text-mandat-red-bright" />
            Контактів: {contacts.length}
          </div>
          <Button
            variant="outline"
            size="icon"
            title="Оновити список"
            aria-label="Оновити список"
            disabled={loading}
            onClick={() => void loadContacts()}
            className="size-10 rounded-none border-mandat-line bg-mandat-bg text-mandat-muted hover:bg-mandat-surface-2 hover:text-mandat-text"
          >
            <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
          </Button>
        </div>

        {moveError && (
          <p role="alert" className="border-b border-mandat-line bg-mandat-red/10 px-4 py-3 text-sm text-mandat-red-bright">
            {moveError}
          </p>
        )}

        <div className="overflow-x-auto">
          <Table className="min-w-[640px]">
            <TableHeader>
              <TableRow className="border-mandat-line bg-mandat-bg/50 hover:bg-mandat-bg/50">
                <TableHead className="w-28 px-5 text-[10px] tracking-[0.16em] text-mandat-muted">ПОРЯДОК</TableHead>
                <TableHead className="text-[10px] tracking-[0.16em] text-mandat-muted">КОНТАКТ</TableHead>
                <TableHead className="w-28 text-right text-[10px] tracking-[0.16em] text-mandat-muted">ДІЇ</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {contacts.map((contact, index) => {
                const name = contactName(contact)
                return (
                  <TableRow key={contact.id} className="border-mandat-line hover:bg-mandat-surface-2/50">
                    <TableCell className="px-5 py-4">
                      <div className="flex items-center gap-1">
                        <span className="w-6 text-sm tabular-nums text-mandat-muted">{index + 1}</span>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title="Вище"
                          aria-label={`Перемістити «${name}» вище`}
                          disabled={index === 0 || movingId !== null}
                          onClick={() => void move(contact, 'up')}
                          className={iconButtonClassName}
                        >
                          <ArrowUp className="size-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title="Нижче"
                          aria-label={`Перемістити «${name}» нижче`}
                          disabled={index === contacts.length - 1 || movingId !== null}
                          onClick={() => void move(contact, 'down')}
                          className={iconButtonClassName}
                        >
                          <ArrowDown className="size-4" />
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell className="py-4">
                      <div className="flex items-center gap-3">
                        <span className="grid size-9 shrink-0 place-items-center rounded-full border border-mandat-line text-mandat-muted">
                          <ContactIcon type={contact.type} className="size-4" />
                        </span>
                        <div className="min-w-0">
                          <div className="text-xs text-mandat-muted">{name}</div>
                          <a
                            href={contact.href}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="break-all font-medium text-mandat-text no-underline hover:text-mandat-red-bright"
                          >
                            {contact.display}
                          </a>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title="Редагувати"
                          aria-label={`Редагувати «${name}»`}
                          onClick={() => openEditor(contact)}
                          className={iconButtonClassName}
                        >
                          <FileEdit className="size-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title="Видалити"
                          aria-label={`Видалити «${name}»`}
                          onClick={() => {
                            setDeleteError(null)
                            setDeletingContact(contact)
                          }}
                          className={iconButtonClassName}
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>

          {loading && (
            <div role="status" className="flex items-center justify-center gap-3 border-t border-mandat-line px-4 py-12 text-sm text-mandat-muted">
              <LoaderCircle className="size-5 animate-spin text-mandat-red-bright" />
              Завантаження контактів...
            </div>
          )}

          {loadError && !loading && (
            <div role="alert" className="flex flex-col items-center border-t border-mandat-line px-4 py-12 text-center">
              <h2 className="text-lg">Не вдалося завантажити контакти</h2>
              <p className="mt-2 max-w-lg text-sm text-mandat-muted">{loadError}</p>
              <Button
                variant="outline"
                onClick={() => void loadContacts()}
                className="mt-5 rounded-none border-mandat-line bg-mandat-bg text-mandat-text hover:bg-mandat-surface-2"
              >
                Спробувати ще раз
              </Button>
            </div>
          )}

          {!loading && !loadError && contacts.length === 0 && (
            <div className="flex flex-col items-center border-t border-mandat-line px-4 py-16 text-center">
              <h2 className="text-lg">Контактів ще немає</h2>
              <p className="mt-2 text-sm text-mandat-muted">Додайте перший контакт.</p>
            </div>
          )}
        </div>
      </section>

      {editorOpen && (
        <ContactEditorDialog
          key={editingContact?.id ?? 'new'}
          open
          contact={editingContact}
          saving={saving}
          error={mutationError}
          onOpenChange={(open) => !open && setEditorOpen(false)}
          onSubmit={(values) => void saveContact(values)}
        />
      )}

      <AlertDialog open={Boolean(deletingContact)} onOpenChange={(open) => !open && !deleting && setDeletingContact(null)}>
        <AlertDialogContent className="rounded-none border-mandat-line bg-mandat-surface text-mandat-text">
          <AlertDialogHeader>
            <div className="mb-2 flex size-10 items-center justify-center bg-mandat-red/15 text-mandat-red-bright">
              <Trash2 className="size-5" />
            </div>
            <AlertDialogTitle className="text-xl">Видалити контакт?</AlertDialogTitle>
            <AlertDialogDescription className="text-mandat-muted">
              «{deletingContact ? `${contactName(deletingContact)}: ${deletingContact.display}` : ''}» зникне зі сторінки
              контактів. Цю дію неможливо скасувати.
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
                void removeContact()
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
