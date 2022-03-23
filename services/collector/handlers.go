package collector

import (
	"github.com/aibotsoft/crypto-collector/pkg/binance_ws"
	"github.com/aibotsoft/crypto-collector/pkg/ftx_ws"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

var ftxDelayList []int64
var binAll, binSend, ftxAll, ftxSend atomic.Int64

func (c *Collector) loadBinance(symbol string) *TickerData {
	got, _ := c.binMap.Load(symbol)
	return got.(*TickerData)
}

func (c *Collector) loadTicker(symbol string) *TickerData {
	got, ok := c.fxtMap.Load(symbol)
	if !ok {
		c.log.Info("not_found_symbol", zap.String("symbol", symbol))
	}
	return got.(*TickerData)
}

func (c *Collector) binanceHandler(e *binance_ws.WsBookTickerEvent) {
	binAll.Inc()
	c.binCountMap[e.Symbol].Inc()

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

	if !t.BidPrice.Equal(t.PrevBidPrice) || !t.AskPrice.Equal(t.PrevAskPrice) {
		binSend.Inc()
		c.send(t, binanceExchange, c.binFtxSymbolMap[t.Symbol])
		c.send(t, binanceExchange, c.binFtxUsdSymbolMap[t.Symbol])
		//c.log.Debug("binance_event",
		//	zap.Any("t", t),
		//	)
	}
}

func (c *Collector) fxtHandler(e *ftx_ws.Response) {
	if e.Market == usdUsdtMarket {
		//c.log.Info("e", zap.Any("e", e))
		c.usdtPrice = e.Data.Bid
		return
	}

	ftxAll.Inc()
	c.ftxCountMap[e.Market].Inc()

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
	ftxDelayList = append(ftxDelayList, t.ReceiveTime-t.ServerTime)

	if !t.BidPrice.Equal(t.PrevBidPrice) || !t.AskPrice.Equal(t.PrevAskPrice) {
		c.send(t, ftxExchange, c.binFtxSymbolMap[t.Symbol])
		//c.log.Info("ftx_event",
		//	zap.Any("t", t),
		//	zap.Any("symbol", c.binFtxSymbolMap[t.Symbol]),
		//)
		ftxSend.Inc()
	}
}
