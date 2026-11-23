import { useEffect, type ReactNode } from 'react'
import { Link } from 'react-router'
import { ArrowLeft } from 'lucide-react'

import HeadingMarker from '@/components/HeadingMarker'
import MandatLogo from '@/components/MandatLogo'
import { ADVERTISING_BACK_LABEL, ADVERTISING_ORDER_LABEL, ADVERTISING_PATH } from '@/constants/advertising'
import { CONTACTS_PATH } from '@/constants/contacts'
import Footer from '@/sections/Footer'
import Header from '@/sections/Header'

interface AdFormatPageProps {
  title: string
  lead: string
  /** Key idea of the format, shown in a highlighted block above the content. */
  principle: string
  ctaTitle: string
  children: ReactNode
}

/** Layout of an ad format page: back link, title, key idea, page sections and an order call to action. */
export default function AdFormatPage({ title, lead, principle, ctaTitle, children }: AdFormatPageProps) {
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'instant' })
    document.title = title
  }, [title])

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />

      <section className="mx-auto max-w-[1080px] px-6 pb-10 pt-10 md:pb-14 md:pt-14">
        <Link
          to={ADVERTISING_PATH}
          className="inline-flex items-center gap-2 text-[15px] text-mandat-muted no-underline transition-colors hover:text-white"
        >
          <ArrowLeft className="size-4" strokeWidth={1.75} aria-hidden="true" />
          {ADVERTISING_BACK_LABEL}
        </Link>
        <h1 className="mt-6 text-[34px] font-bold leading-[1.1] text-white md:text-[54px]">
          <HeadingMarker />
          {title}
        </h1>
        <p className="mt-6 max-w-[680px] text-[16px] leading-relaxed text-mandat-muted md:text-[18px]">{lead}</p>
      </section>

      <main className="mx-auto max-w-[1080px] px-6 pb-20">
        <p className="m-0 max-w-[860px] border-l-4 border-mandat-red-bright bg-mandat-surface py-6 pl-6 pr-6 text-[20px] font-medium leading-snug text-white md:py-8 md:pl-8 md:text-[26px]">
          {principle}
        </p>
        {children}
      </main>

      <section className="bg-mandat-red">
        <div className="mx-auto flex max-w-[1080px] flex-col gap-8 px-6 py-12 md:flex-row md:items-end md:justify-between md:py-16">
          <MandatLogo className="h-8 self-start md:h-10 md:self-auto" barClassName="fill-white" />
          <div className="md:text-right">
            <p className="text-[28px] font-bold leading-tight text-white md:text-[36px]">{ctaTitle}</p>
            <Link
              to={CONTACTS_PATH}
              className="mt-6 inline-flex h-12 items-center justify-center bg-white px-7 text-[15px] font-medium text-mandat-bg no-underline transition-colors hover:bg-white/90"
            >
              {ADVERTISING_ORDER_LABEL}
            </Link>
          </div>
        </div>
      </section>

      <Footer />
    </div>
  )
}
