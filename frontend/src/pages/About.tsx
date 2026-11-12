import { useEffect } from 'react'

import { ABOUT_MISSION, ABOUT_POLICY, ABOUT_PRINCIPLES, ABOUT_TOPICS } from '@/constants/about'
import Footer from '@/sections/Footer'
import Header from '@/sections/Header'
import Telegram from '@/sections/Telegram'

export default function About() {
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'instant' })
  }, [])

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />
      <main className="mx-auto min-h-[60vh] max-w-[1240px] px-6 py-[72px]">
        <div className="mb-10">
          <h1 className="text-[30px] font-bold tracking-[1px] md:text-[42px]">
            <span className="inline-block w-2 h-8 bg-mandat-red-bright mr-3.5 align-middle md:h-10" />
            Про нас
          </h1>
        </div>

        <section className="max-w-[800px]">
          <p className="font-display text-[26px] md:text-[34px] font-bold leading-snug">
            {ABOUT_MISSION.lead}
          </p>
          {ABOUT_MISSION.paragraphs.map((paragraph) => (
            <p key={paragraph} className="mt-6 text-[17px] leading-8">
              {paragraph}
            </p>
          ))}
        </section>

        <section className="mt-14">
          <h2 className="text-[22px] sm:text-[26px] tracking-[1px] font-bold mb-6">
            <span className="inline-block w-2 h-6 bg-mandat-red-bright mr-3 align-middle" />
            Наші принципи
          </h2>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
            {ABOUT_PRINCIPLES.map((principle) => (
              <div key={principle.title} className="border border-mandat-line bg-mandat-surface p-6">
                <h3 className="font-display text-[18px] font-bold text-mandat-red-bright">
                  {principle.title}
                </h3>
                <p className="mt-3 text-[16px] leading-7 text-mandat-muted">{principle.text}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="mt-14">
          <h2 className="text-[22px] sm:text-[26px] tracking-[1px] font-bold mb-6">
            <span className="inline-block w-2 h-6 bg-mandat-red-bright mr-3 align-middle" />
            Про що ми пишемо
          </h2>
          <div className="border border-mandat-line bg-mandat-surface divide-y divide-mandat-line">
            {ABOUT_TOPICS.map((topic) => (
              <div key={topic.title} className="grid gap-1 px-6 py-4 md:grid-cols-[240px_1fr] md:gap-6">
                <h3 className="font-display text-[15px] uppercase tracking-[1px]">{topic.title}</h3>
                <p className="text-[16px] leading-7 text-mandat-muted">{topic.text}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="mt-14 max-w-[800px]">
          <h2 className="text-[22px] sm:text-[26px] tracking-[1px] font-bold mb-6">
            <span className="inline-block w-2 h-6 bg-mandat-red-bright mr-3 align-middle" />
            Редакційна політика
          </h2>
          {ABOUT_POLICY.map((rule) => (
            <div key={rule.title} className="mt-6">
              <h3 className="font-display text-[17px] font-bold">{rule.title}</h3>
              <p className="mt-2 text-[16px] leading-7 text-mandat-muted">{rule.text}</p>
            </div>
          ))}
        </section>
      </main>
      <Telegram />
      <Footer />
    </div>
  )
}
