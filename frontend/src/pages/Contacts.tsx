import { useEffect, useState } from 'react'

import { listContacts } from '@/api/contacts'
import ContactList from '@/components/ContactList'
import HeadingMarker from '@/components/HeadingMarker'
import { CONTACTS_TITLE } from '@/constants/contacts'
import Footer from '@/sections/Footer'
import Header from '@/sections/Header'
import type { Contact } from '@/types'

export default function Contacts() {
  const [contacts, setContacts] = useState<Contact[]>([])

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'instant' })
    document.title = CONTACTS_TITLE
  }, [])

  // Contacts are managed in the admin panel; on failure the grid just stays empty.
  useEffect(() => {
    let active = true
    listContacts()
      .then((items) => active && setContacts(items))
      .catch(() => undefined)
    return () => {
      active = false
    }
  }, [])

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />

      <main className="mx-auto max-w-[1080px] px-6 pb-20 pt-10 md:pb-28 md:pt-14">
        <h1 className="text-[34px] font-bold leading-[1.1] text-white md:text-[54px]">
          <HeadingMarker />
          {CONTACTS_TITLE}
        </h1>

        {contacts.length > 0 && (
          <div className="mt-10 md:mt-12">
            <ContactList contacts={contacts} />
          </div>
        )}
      </main>

      <Footer />
    </div>
  )
}
