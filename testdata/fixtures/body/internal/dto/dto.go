package dto

import "time"

const CurrencySGD = "SGD"

type Base struct {
	RequestID string `json:"requestId"`
}

type Amount struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type TopupReq struct {
	Base
	Source  Amount         `json:"source"`
	Target  *string        `json:"targetCurrency,omitempty"`
	Express bool           `json:"express"`
	Tags    []string       `json:"tags"`
	Meta    map[string]any `json:"meta"`
	At      time.Time      `json:"at"`
	Note    string
	Skip    string `json:"-"`
	hidden  string
}

type LegacyReq struct {
	Count  int  `json:"count"`
	DryRun bool `json:"dryRun"`
}
