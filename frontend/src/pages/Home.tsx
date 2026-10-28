import Header from '@/sections/Header'
import Hero from '@/sections/Hero'
import Ticker from '@/sections/Ticker'
import News from '@/sections/News'
import LatestNews from '@/sections/LatestNews'
import Telegram from '@/sections/Telegram'
import Footer from '@/sections/Footer'
import usePublicNews from '@/hooks/use-public-news'

export default function Home() {
  const { articles, latestNews, tickerItems, loading, error, retry } = usePublicNews()

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />
      <Hero />
      <Ticker headlines={tickerItems} />
      <News articles={articles} loading={loading} error={error} onRetry={retry} />
      <LatestNews items={latestNews} />
      <Telegram />
      <Footer />
    </div>
  )
}
