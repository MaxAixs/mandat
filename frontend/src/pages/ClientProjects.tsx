import AdFormatList from '@/components/AdFormatList'
import AdFormatPage from '@/components/AdFormatPage'
import AdFormatSectionTitle from '@/components/AdFormatSectionTitle'
import {
  CLIENT_PROJECTS_CTA_TITLE,
  CLIENT_PROJECTS_FORMATS,
  CLIENT_PROJECTS_FORMATS_TITLE,
  CLIENT_PROJECTS_LEAD,
  CLIENT_PROJECTS_PRINCIPLE,
  CLIENT_PROJECTS_TITLE,
} from '@/constants/client-projects'

export default function ClientProjects() {
  return (
    <AdFormatPage
      title={CLIENT_PROJECTS_TITLE}
      lead={CLIENT_PROJECTS_LEAD}
      principle={CLIENT_PROJECTS_PRINCIPLE}
      ctaTitle={CLIENT_PROJECTS_CTA_TITLE}
    >
      <section className="mt-16 md:mt-20">
        <AdFormatSectionTitle>{CLIENT_PROJECTS_FORMATS_TITLE}</AdFormatSectionTitle>
        <AdFormatList items={CLIENT_PROJECTS_FORMATS} />
      </section>
    </AdFormatPage>
  )
}
