import AdFormatList from '@/components/AdFormatList'
import AdFormatPage from '@/components/AdFormatPage'
import AdFormatSectionTitle from '@/components/AdFormatSectionTitle'
import {
  PARTNER_MATERIALS_CTA_TITLE,
  PARTNER_MATERIALS_FORMATS,
  PARTNER_MATERIALS_FORMATS_TITLE,
  PARTNER_MATERIALS_LEAD,
  PARTNER_MATERIALS_PRINCIPLE,
  PARTNER_MATERIALS_TITLE,
} from '@/constants/partner-materials'

export default function PartnerMaterials() {
  return (
    <AdFormatPage
      title={PARTNER_MATERIALS_TITLE}
      lead={PARTNER_MATERIALS_LEAD}
      principle={PARTNER_MATERIALS_PRINCIPLE}
      ctaTitle={PARTNER_MATERIALS_CTA_TITLE}
    >
      <section className="mt-16 md:mt-20">
        <AdFormatSectionTitle>{PARTNER_MATERIALS_FORMATS_TITLE}</AdFormatSectionTitle>
        <AdFormatList items={PARTNER_MATERIALS_FORMATS} />
      </section>
    </AdFormatPage>
  )
}
