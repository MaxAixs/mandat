import Header from '../sections/Header'
import Hero from '../sections/Hero'
import Ticker from '../sections/Ticker'
import News from '../sections/News'
import Analytics from '../sections/Analytics'
import Insight from '../sections/Insight'
import Digest from '../sections/Digest'
import Footer from '../sections/Footer'

export default function Home() {
  return (
    <div className="min-h-screen" style={{ background: 'var(--mandat-bg)', color: 'var(--mandat-text)' }}>
      <Header />
      <Hero />
      <Ticker />
      <News />
      <Analytics />
      <Insight />
      <Digest />
      <Footer />
    </div>
  )
}
