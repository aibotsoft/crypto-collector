package collector

import (
	"github.com/aibotsoft/crypto-collector/pkg/binance_ws"
	"github.com/aibotsoft/crypto-collector/pkg/ftx_ws"
)

func (c *Collector) loadBinance(symbol string) *TickerData {
	got, _ := c.binMap.Load(symbol)
	return got.(*TickerData)
}

func (c *Collector) loadTicker(symbol string) *TickerData {
	got, _ := c.fxtMap.Load(symbol)
	return got.(*TickerData)
}

func (c *Collector) binanceHandler(e *binance_ws.WsBookTickerEvent) {
	t := c.loadBinance(e.Symbol)
	prev := *t
	t.BidPrice = e.BestBidPrice
	t.BidQty = e.BestBidQty
	t.AskPrice = e.BestAskPrice
	t.AskQty = e.BestAskQty
	t.ReceiveTime = e.ReceiveTime

	t.PrevBidPrice = prev.BidPrice
	t.PrevBidQty = prev.BidQty
	t.PrevAskPrice = prev.AskPrice
	t.PrevAskQty = prev.AskQty
	t.PrevServerTime = prev.ServerTime
	t.PrevReceiveTime = prev.ReceiveTime

	c.send(t, c.binFtxSymbolMap[t.Symbol])
	if c.cfg.Service.EnableUSD {
		c.send(t, c.binFtxUsdSymbolMap[t.Symbol])
	}
}

func (c *Collector) ftxHandler(e *ftx_ws.Response) {
	t := c.loadTicker(e.Market)
	prev := *t
	t.BidPrice = e.Data.Bid
	t.BidQty = e.Data.BidSize
	t.AskPrice = e.Data.Ask
	t.AskQty = e.Data.AskSize
	t.ServerTime = int64(e.Data.Time * 1000000000)
	t.ReceiveTime = e.ReceiveTime

	t.PrevBidPrice = prev.BidPrice
	t.PrevBidQty = prev.BidQty
	t.PrevAskPrice = prev.AskPrice
	t.PrevAskQty = prev.AskQty
	t.PrevServerTime = prev.ServerTime
	t.PrevReceiveTime = prev.ReceiveTime
}
