import AdFormatList from '@/components/AdFormatList'
import AdFormatPage from '@/components/AdFormatPage'
import AdFormatSectionTitle from '@/components/AdFormatSectionTitle'
import {
  LONGREADS_CTA_TITLE,
  LONGREADS_FORMATS,
  LONGREADS_FORMATS_TITLE,
  LONGREADS_LEAD,
  LONGREADS_PRINCIPLE,
  LONGREADS_TITLE,
} from '@/constants/longreads'

export default function Longreads() {
  return (
    <AdFormatPage
      title={LONGREADS_TITLE}
      lead={LONGREADS_LEAD}
      principle={LONGREADS_PRINCIPLE}
      ctaTitle={LONGREADS_CTA_TITLE}
    >
      <section className="mt-16 md:mt-20">
        <AdFormatSectionTitle>{LONGREADS_FORMATS_TITLE}</AdFormatSectionTitle>
        <AdFormatList items={LONGREADS_FORMATS} />
      </section>
    </AdFormatPage>
  )
}
