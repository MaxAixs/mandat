import Header from '@/sections/Header'
import Hero from '@/sections/Hero'
import Ticker from '@/sections/Ticker'
import News from '@/sections/News'
import LatestNews from '@/sections/LatestNews'
import Telegram from '@/sections/Telegram'
import Footer from '@/sections/Footer'
import usePublicNews from '@/hooks/use-public-news'
import useExchangeRates from '@/hooks/use-exchange-rates'
import type { TickerRate } from '@/sections/Ticker'

// The «Останні новини» strip shows the ten newest articles.
const LATEST_NEWS_COUNT = 10

// Rate display metadata by currency code; the API order is preserved.
const rateMeta: Record<string, { name: string; flag: string }> = {
  EUR: { name: 'Євро', flag: '🇪🇺' },
  USD: { name: 'Долар США', flag: '🇺🇸' },
  PLN: { name: 'Злотий', flag: '🇵🇱' },
}

// toTickerRates maps the raw API rates onto display-ready ticker cards.
function toTickerRates(rates: { code: string; rate: number }[]): TickerRate[] {
  return rates.map((rate) => ({
    code: rate.code,
    name: rateMeta[rate.code]?.name ?? rate.code,
    flag: rateMeta[rate.code]?.flag ?? '🏳️',
    value: rate.rate.toFixed(2),
  }))
}

export default function Home() {
  const { latestNews, mainEvent } = usePublicNews()
  const { rates } = useExchangeRates()

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />
      <Hero mainEvent={mainEvent} />
      <Ticker items={toTickerRates(rates)} />
      <News />
      <LatestNews items={latestNews.slice(0, LATEST_NEWS_COUNT)} />
      <Telegram />
      <Footer />
    </div>
  )
}
