package collector

import (
	"github.com/shopspring/decimal"
)

type TickerData struct {
	Symbol          string          `json:"s"`
	BidPrice        decimal.Decimal `json:"b"`
	BidQty          decimal.Decimal `json:"B"`
	AskPrice        decimal.Decimal `json:"a"`
	AskQty          decimal.Decimal `json:"A"`
	ServerTime      int64           `json:"st"`
	ReceiveTime     int64           `json:"rt"`
	PrevBidPrice    decimal.Decimal `json:"pb"`
	PrevBidQty      decimal.Decimal `json:"pB"`
	PrevAskPrice    decimal.Decimal `json:"pa"`
	PrevAskQty      decimal.Decimal `json:"pA"`
	PrevServerTime  int64           `json:"pst"`
	PrevReceiveTime int64           `json:"prt"`
}
type Surebet struct {
	ID           int64           `json:"id"`
	BinTicker    *TickerData     `json:"b"`
	FtxTicker    *TickerData     `json:"o"`
	LastBinTime  int64           `json:"last_bin_time"`
	AvgPriceDiff decimal.Decimal `json:"avg_price_diff"`
	MaxPriceDiff decimal.Decimal `json:"max_price_diff"`
	MinPriceDiff decimal.Decimal `json:"min_price_diff"`
}
