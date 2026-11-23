import { Globe, Mail, Phone } from 'lucide-react'

import SocialIcon from '@/components/SocialIcon'
import type { ContactType } from '@/types'

interface ContactIconProps {
  type: ContactType
  className?: string
}

/** Icon of a contact type: brand glyphs for social profiles, line icons for the rest. */
export default function ContactIcon({ type, className }: ContactIconProps) {
  switch (type) {
    case 'email':
      return <Mail className={className} strokeWidth={1.75} aria-hidden="true" />
    case 'phone':
      return <Phone className={className} strokeWidth={1.75} aria-hidden="true" />
    case 'website':
      return <Globe className={className} strokeWidth={1.75} aria-hidden="true" />
    default:
      return <SocialIcon id={type} className={className} />
  }
}
