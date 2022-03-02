package ftx_ws

import (
	"github.com/shopspring/decimal"
)

type Request struct {
	Op      string `json:"op"`
	Channel string `json:"channel,omitempty"`
	Market  string `json:"market,omitempty"`
}
type Response struct {
	Type    string `json:"type"`
	Channel string `json:"channel,omitempty"`
	Market  string `json:"market,omitempty"`

	Code int64 `json:"code,omitempty"`
	Data struct {
		Ask     decimal.Decimal `json:"ask,omitempty"`
		AskSize decimal.Decimal `json:"askSize,omitempty"`
		Bid     decimal.Decimal `json:"bid,omitempty"`
		BidSize decimal.Decimal `json:"bidSize,omitempty"`
		Last    decimal.Decimal `json:"last,omitempty"`
		Time    float64         `json:"time,omitempty"`
	} `json:"data,omitempty"`
	Msg         string `json:"msg,omitempty"`
	ReceiveTime int64  `json:"rt,omitempty"`
}
