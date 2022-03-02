package binance_ws

import (
	"github.com/shopspring/decimal"
)

type WsBookTickerEvent struct {
	UpdateID     int64           `json:"u"`
	Symbol       string          `json:"s"`
	BestBidPrice decimal.Decimal `json:"b"`
	BestBidQty   decimal.Decimal `json:"B"`
	BestAskPrice decimal.Decimal `json:"a"`
	BestAskQty   decimal.Decimal `json:"A"`
	ReceiveTime  int64           `json:"rt"`
}
type Event struct {
	Stream string            `json:"stream"`
	Data   WsBookTickerEvent `json:"data"`
}
