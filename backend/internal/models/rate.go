package models

// Rate is one quoted value against the hryvnia: the value of one unit
// of Code in UAH on Date. It carries both currency and fuel prices.
type Rate struct {
	Code string
	Rate float64
	Date string
}
