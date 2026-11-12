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
import type { TickerIcon, TickerRate } from '@/sections/Ticker'

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
const fuelMeta: Record<string, { name: string; icon: TickerIcon }> = {
  A95: { name: 'А-95', icon: 'fuel' },
  A92: { name: 'А-92', icon: 'fuel' },
  A98: { name: 'А-98', icon: 'fuel' },
  DIESEL: { name: 'Дизель', icon: 'droplet' },
  LPG: { name: 'Автогаз', icon: 'flame' },
}

// toFuelCards maps the raw API fuel rates onto display-ready ticker cards.
function toFuelCards(rates: { code: string; rate: number }[]): TickerRate[] {
  return rates.map((rate) => ({
    code: rate.code,
    name: fuelMeta[rate.code]?.name ?? rate.code,
    icon: fuelMeta[rate.code]?.icon ?? 'fuel',
    value: rate.rate.toFixed(2),
    unit: '₴/л',
  }))
}

export default function Home() {
  const { latestNews, mainEvent } = usePublicNews()
  const { rates } = useExchangeRates()
  const { rates: fuelRates } = useFuelPrices()

  const currencyCards = toTickerRates(rates)
  const fuelCards = toFuelCards(fuelRates)
  const hasTicker = currencyCards.length > 0 || fuelCards.length > 0

  return (
    <div className="min-h-screen bg-mandat-bg text-mandat-text">
      <Header />
      <Hero mainEvent={mainEvent} />
      {hasTicker && (
        <div className="mt-16 border-y border-mandat-line bg-mandat-surface">
          <Ticker items={currencyCards} ariaLabel="Курс валют" />
          {currencyCards.length > 0 && fuelCards.length > 0 && (
            <div aria-hidden="true" className="h-px bg-mandat-line" />
          )}
          <Ticker items={fuelCards} ariaLabel="Паливо" reverse />
        </div>
      )}
      <News />
      <LatestNews items={latestNews.slice(0, LATEST_NEWS_COUNT)} />
      <Telegram />
      <Footer />
    </div>
  )
}
