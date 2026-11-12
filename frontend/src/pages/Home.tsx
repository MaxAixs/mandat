import Header from '@/sections/Header'
import Hero from '@/sections/Hero'
import Ticker from '@/sections/Ticker'
import News from '@/sections/News'
import LatestNews from '@/sections/LatestNews'
import Telegram from '@/sections/Telegram'
import Footer from '@/sections/Footer'
import usePublicNews from '@/hooks/use-public-news'
import useExchangeRates from '@/hooks/use-exchange-rates'
import useFuelPrices from '@/hooks/use-fuel-prices'
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

// Fuel display metadata by fuel code; the API order is preserved.
const fuelMeta: Record<string, { name: string; flag: string }> = {
  A95: { name: 'А-95', flag: '⛽' },
  A92: { name: 'А-92', flag: '⛽' },
  A98: { name: 'А-98', flag: '⛽' },
  DIESEL: { name: 'Дизель', flag: '🛢️' },
  LPG: { name: 'Автогаз', flag: '🚗' },
}

// toFuelCards maps the raw API fuel rates onto display-ready ticker cards.
function toFuelCards(rates: { code: string; rate: number }[]): TickerRate[] {
  return rates.map((rate) => ({
    code: rate.code,
    name: fuelMeta[rate.code]?.name ?? rate.code,
    flag: fuelMeta[rate.code]?.flag ?? '🏳️',
    value: rate.rate.toFixed(2),
    unit: '₴/л',
  }))
}

export default function Home() {
  const { latestNews, mainEvent } = usePublicNews()
  const { rates } = useExchangeRates()
  const { rates: fuelRates } = useFuelPrices()

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />
      <Hero mainEvent={mainEvent} />
      <Ticker items={[...toTickerRates(rates), ...toFuelCards(fuelRates)]} />
      <News />
      <LatestNews items={latestNews.slice(0, LATEST_NEWS_COUNT)} />
      <Telegram />
      <Footer />
    </div>
  )
}
