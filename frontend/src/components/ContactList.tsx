import ContactIcon from '@/components/ContactIcon'
import { CONTACT_TYPE_LABELS } from '@/constants/contacts'
import type { Contact } from '@/types'

interface ContactListProps {
  contacts: Contact[]
}

/** Grid of contact tiles: icon by type, label (or the type name), display text, link. */
export default function ContactList({ contacts }: ContactListProps) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {contacts.map((contact) => {
        const external = /^https?:\/\//.test(contact.href)
        return (
          <a
            key={contact.id}
            href={contact.href}
            target={external ? '_blank' : undefined}
            rel={external ? 'noopener noreferrer' : undefined}
            className="group flex items-center gap-5 border border-mandat-line bg-mandat-surface p-5 no-underline transition-colors hover:border-mandat-red-bright sm:p-6"
          >
            <span className="grid size-12 shrink-0 place-items-center rounded-full border border-mandat-line text-white/80 transition-colors group-hover:border-mandat-red-bright group-hover:text-mandat-red-bright">
              <ContactIcon type={contact.type} className="size-5" />
            </span>
            <span className="min-w-0">
              <span className="block text-[14px] text-mandat-muted">
                {contact.label || CONTACT_TYPE_LABELS[contact.type]}
              </span>
              <span className="mt-1 block break-words text-[20px] font-semibold text-white md:text-[22px]">
                {contact.display}
              </span>
            </span>
          </a>
        )
      })}
    </div>
  )
}
