import { useState, type FormEvent } from 'react'
import { LoaderCircle } from 'lucide-react'

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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { CONTACT_TYPE_LABELS, CONTACT_VALUE_PLACEHOLDERS } from '@/constants/contacts'
import type { Contact, ContactFormValues, ContactType } from '@/types'

// Mirrors the backend limit of a contact label.
const LABEL_MAX_LENGTH = 40

const CONTACT_TYPES = Object.keys(CONTACT_TYPE_LABELS) as ContactType[]

const inputClassName =
  'rounded-none border-mandat-line bg-mandat-bg text-mandat-text placeholder:text-mandat-muted focus-visible:ring-mandat-red-bright/30'

export interface ContactEditorDialogProps {
  open: boolean
  /** The contact being edited; undefined creates a new one. */
  contact?: Contact
  saving?: boolean
  error?: string | null
  onOpenChange: (open: boolean) => void
  onSubmit: (values: ContactFormValues) => void
}

/** Create/edit dialog of a contact: type, value and optional label. */
export default function ContactEditorDialog({
  open,
  contact,
  saving = false,
  error,
  onOpenChange,
  onSubmit,
}: ContactEditorDialogProps) {
  const [type, setType] = useState<ContactType>(contact?.type ?? 'email')
  const [value, setValue] = useState(contact?.value ?? '')
  const [label, setLabel] = useState(contact?.label ?? '')
  const [valueError, setValueError] = useState<string | null>(null)

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!value.trim()) {
      setValueError('Вкажіть значення контакту.')
      return
    }

    onSubmit({ type, value: value.trim(), label: label.trim() })
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !saving && onOpenChange(next)}>
      <DialogContent
        showCloseButton={false}
        className="flex max-h-[92vh] max-w-xl flex-col gap-0 overflow-hidden rounded-none border-mandat-line bg-mandat-surface p-0 text-mandat-text shadow-2xl shadow-black/50"
      >
        <DialogHeader className="border-b border-mandat-line px-6 py-5 text-left md:px-8">
          <div className="mb-1 text-[11px] font-semibold tracking-[0.2em] text-mandat-red-bright">КОНТАКТИ</div>
          <DialogTitle className="text-2xl font-medium tracking-wide">
            {contact ? 'Редагувати контакт' : 'Новий контакт'}
          </DialogTitle>
          <DialogDescription className="text-mandat-muted">
            Контакт з’явиться на сторінці «Наші контакти». Посилання та іконка підставляються за типом.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="grid gap-5 overflow-y-auto px-6 py-6 md:px-8">
            <div className="space-y-2">
              <Label htmlFor="contact-type" className="text-xs tracking-wide text-mandat-muted">
                ТИП
              </Label>
              <Select value={type} onValueChange={(next) => setType(next as ContactType)}>
                <SelectTrigger id="contact-type" className={`${inputClassName} h-11 w-full`}>
                  {/* The label is given explicitly, so the trigger shows it before the list ever opens. */}
                  <SelectValue>{CONTACT_TYPE_LABELS[type]}</SelectValue>
                </SelectTrigger>
                <SelectContent className="rounded-none border-mandat-line bg-mandat-surface text-mandat-text">
                  {CONTACT_TYPES.map((option) => (
                    <SelectItem key={option} value={option} className="rounded-none">
                      {CONTACT_TYPE_LABELS[option]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="contact-value" className="text-xs tracking-wide text-mandat-muted">
                ЗНАЧЕННЯ
              </Label>
              <Input
                id="contact-value"
                value={value}
                onChange={(event) => {
                  setValue(event.target.value)
                  setValueError(null)
                }}
                placeholder={CONTACT_VALUE_PLACEHOLDERS[type]}
                aria-invalid={Boolean(valueError)}
                className={`${inputClassName} h-11`}
              />
              {valueError && <p className="text-xs text-mandat-red-bright">{valueError}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="contact-label" className="text-xs tracking-wide text-mandat-muted">
                ПІДПИС (НЕОБОВ’ЯЗКОВО)
              </Label>
              <Input
                id="contact-label"
                value={label}
                maxLength={LABEL_MAX_LENGTH}
                onChange={(event) => setLabel(event.target.value)}
                placeholder={`Замість «${CONTACT_TYPE_LABELS[type]}», наприклад Реклама`}
                className={`${inputClassName} h-11`}
              />
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
