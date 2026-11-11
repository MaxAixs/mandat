package models

// ExchangeRate is one foreign currency rate against the hryvnia:
// the price of one unit of Code in UAH on Date.
type ExchangeRate struct {
	Code string
	Rate float64
	Date string
}
