package rate

import "time"

type CreateRateDto struct {
	FromCurrency string     `json:"from_currency" binding:"required"`
	ToCurrency   string     `json:"to_currency"   binding:"required"`
	Rate         string     `json:"rate"          binding:"required"`
	ValidFrom    time.Time  `json:"valid_from"    binding:"required"`
	ValidTo      *time.Time `json:"valid_to,omitempty"`
	Source       *string    `json:"source,omitempty"`
}

type UpdateRateDto struct {
	FromCurrency *string    `json:"from_currency,omitempty"`
	ToCurrency   *string    `json:"to_currency,omitempty"`
	Rate         *string    `json:"rate,omitempty"`
	ValidFrom    *time.Time `json:"valid_from,omitempty"`
	ValidTo      *time.Time `json:"valid_to,omitempty"`
	Source       *string    `json:"source,omitempty"`
}
