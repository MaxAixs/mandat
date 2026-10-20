import Header from '@/sections/Header'
import Hero from '@/sections/Hero'
import Ticker from '@/sections/Ticker'
import News from '@/sections/News'
import Insight from '@/sections/Insight'
import Digest from '@/sections/Digest'
import Telegram from '@/sections/Telegram'
import Footer from '@/sections/Footer'

export default function Home() {
  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />
      <Hero />
      <Ticker />
      <News />
      <Insight />
      <Digest />
      <Telegram />
      <Footer />
    </div>
  )
}
