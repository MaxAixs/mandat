import { useEffect } from 'react'

import MandatLogo from '@/components/MandatLogo'
import {
  RULES_INTRO,
  RULES_LEAD,
  RULES_PARTS,
  RULES_PRINCIPLES,
  RULES_PRINCIPLES_TITLE,
  RULES_TITLE,
  type RulesParagraph,
} from '@/constants/rules'
import Footer from '@/sections/Footer'
import Header from '@/sections/Header'

function Paragraph({ paragraph }: { paragraph: RulesParagraph }) {
  return (
    <p className="mt-5 text-[17px] leading-[1.75]">
      {paragraph.lead && <strong className="font-semibold text-white">{paragraph.lead} </strong>}
      {paragraph.text}
    </p>
  )
}

function SectionTitle({ children }: { children: string }) {
  return (
    <h2 className="mt-10 text-[26px] font-bold leading-tight tracking-[0.3px] text-white md:text-[32px]">
      <span className="mr-3.5 inline-block h-[1cap] w-2 bg-mandat-red-bright align-baseline" />
      {children}
    </h2>
  )
}

function Bullets({ items }: { items: string[] }) {
  return (
    <ul className="m-0 mt-5 grid list-none gap-3 p-0">
      {items.map((item) => (
        <li key={item} className="relative pl-6 text-[17px] leading-[1.75]">
          <span className="absolute left-0 top-[0.78em] h-[2px] w-3 bg-mandat-red-bright" />
          {item}
        </li>
      ))}
    </ul>
  )
}

export default function Rules() {
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'instant' })
    document.title = RULES_TITLE
  }, [])

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />

      <section className="bg-mandat-red">
        <div className="mx-auto max-w-[1080px] px-6 py-12 md:py-16">
          <MandatLogo className="h-7 opacity-60 md:h-8" barClassName="fill-white" />
          <h1 className="mt-8 max-w-[760px] text-[34px] font-bold leading-[1.1] text-white md:text-[54px]">
            {RULES_TITLE}
          </h1>
          <p className="mt-8 max-w-[760px] text-[16px] leading-relaxed text-white/90 md:text-[18px]">
            {RULES_LEAD}
          </p>
        </div>
      </section>

      <main className="mx-auto max-w-[1080px] px-6 pb-24 pt-8">
        <div className="max-w-[780px]">
          {RULES_INTRO.map((paragraph) => (
            <Paragraph key={paragraph.text} paragraph={paragraph} />
          ))}

          <SectionTitle>{RULES_PRINCIPLES_TITLE}</SectionTitle>
          <ol className="m-0 list-none p-0">
            {RULES_PRINCIPLES.map((principle, index) => (
              <li key={principle.title} className="mt-6">
                <h3 className="text-[18px] font-semibold text-white md:text-[19px]">
                  <span className="mr-2 text-mandat-red-bright">{index + 1})</span>
                  {principle.title}
                </h3>
                {principle.paragraphs.map((text) => (
                  <p key={text} className="mt-3 text-[17px] leading-[1.75]">
                    {text}
                  </p>
                ))}
              </li>
            ))}
          </ol>

          {RULES_PARTS.map((part) => (
            <section key={part.title}>
              <SectionTitle>{part.title}</SectionTitle>
              {part.intro?.map((paragraph) => (
                <Paragraph key={paragraph.text} paragraph={paragraph} />
              ))}
              {part.blocks.map((block, index) => (
                <div key={block.heading ?? `${part.title}-${index}`} className="mt-6">
                  {block.heading && (
                    <h3 className="text-[20px] font-semibold text-white md:text-[22px]">{block.heading}</h3>
                  )}
                  {block.paragraphs?.map((paragraph) => (
                    <Paragraph key={paragraph.text} paragraph={paragraph} />
                  ))}
                  {block.bullets && <Bullets items={block.bullets} />}
                </div>
              ))}
            </section>
          ))}
        </div>
      </main>

      <Footer />
    </div>
  )
}
