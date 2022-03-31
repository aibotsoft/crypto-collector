package collector

import (
	"context"
	"fmt"
	"github.com/RobinUS2/golang-moving-average"
	"github.com/aibotsoft/crypto-collector/pkg/binance_ws"
	"github.com/aibotsoft/crypto-collector/pkg/config"
	"github.com/aibotsoft/crypto-collector/pkg/ftx_ws"
	"github.com/cheynewallace/tabby"
	"github.com/nats-io/nats.go"
	"github.com/shopspring/decimal"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"runtime"
	"strings"
	"sync"
	"time"
)

const cryptoSubject = "crypto-surebet"
const usdt = "USDT"
const usd = "USD"
const usdUsdtMarket = "USDT/USD"

var d100 = decimal.RequireFromString("100")

type Collector struct {
	cfg                *config.Config
	log                *zap.Logger
	ctx                context.Context
	binMap             sync.Map
	fxtMap             sync.Map
	errCh              chan error
	ftxClient          *ftx_ws.WsClient
	binClient          *binance_ws.WsClient
	ec                 *nats.EncodedConn
	binFtxSymbolMap    map[string]string
	binFtxUsdSymbolMap map[string]string
	maPriceMap         map[string]*movingaverage.MovingAverage
	maFtxDelay         map[string]*movingaverage.MovingAverage
	binCountMap        map[string]*atomic.Int64
	lastBinTime        int64
	lastID             int64
	idLock             sync.Mutex
	usdtPrice          decimal.Decimal
}

func NewCollector(cfg *config.Config, log *zap.Logger, ctx context.Context) *Collector {
	c := &Collector{
		cfg:                cfg,
		log:                log,
		ctx:                ctx,
		errCh:              make(chan error),
		binFtxSymbolMap:    make(map[string]string),
		binFtxUsdSymbolMap: make(map[string]string),
		maPriceMap:         make(map[string]*movingaverage.MovingAverage),
		maFtxDelay:         make(map[string]*movingaverage.MovingAverage),
		binCountMap:        make(map[string]*atomic.Int64),
		usdtPrice:          decimal.RequireFromString("1"),
	}
	c.ftxClient = ftx_ws.NewWsClient(cfg, log, ctx, c.ftxEventHandler)
	c.binClient = binance_ws.NewWsClient(cfg, log, ctx, c.binEventHandler)
	return c
}
func (c *Collector) Run() error {
	for _, m := range c.cfg.Markets {
		c.binFtxSymbolMap[fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)] = fmt.Sprintf("%s/%s", strings.ToUpper(m), usdt)
		c.binFtxUsdSymbolMap[fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)] = fmt.Sprintf("%s/%s", strings.ToUpper(m), usd)

		c.binFtxSymbolMap[fmt.Sprintf("%s/%s", strings.ToUpper(m), usdt)] = fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)
		c.binFtxSymbolMap[fmt.Sprintf("%s/%s", strings.ToUpper(m), usd)] = fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)
	}
	for _, m := range c.cfg.Markets {
		symbol := fmt.Sprintf("%s/%s", strings.ToUpper(m), usdt)
		t := TickerData{Symbol: symbol}
		c.fxtMap.Store(symbol, &t)
		c.maPriceMap[symbol] = movingaverage.New(c.cfg.Service.AverageWindow)

		if c.cfg.Service.EnableUSD {
			usdSymbol := fmt.Sprintf("%s/%s", strings.ToUpper(m), usd)
			c.log.Debug("usd_symbol", zap.String("symbol", usdSymbol))
			t2 := TickerData{Symbol: usdSymbol}
			c.fxtMap.Store(usdSymbol, &t2)
			c.maPriceMap[usdSymbol] = movingaverage.New(c.cfg.Service.AverageWindow)
		}
	}

	for _, m := range c.cfg.Markets {
		symbol := fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)
		b := TickerData{Symbol: symbol}
		c.binMap.Store(symbol, &b)
		c.binCountMap[symbol] = &atomic.Int64{}
	}
	err := c.connectNats()
	if err != nil {
		return fmt.Errorf("connect_nats_error: %w", err)
	}
	statTick := time.Tick(c.cfg.Service.LogStatPeriod)
	go func() {
		c.errCh <- c.binClient.Serve()
	}()
	go func() {
		c.errCh <- c.ftxClient.Serve()
	}()

	for {
		select {
		case <-statTick:
			go c.printStat()
		case err := <-c.errCh:
			return err
		case <-c.ctx.Done():
			return c.ctx.Err()
		}
	}
}

func (c *Collector) binEventHandler(event *binance_ws.WsBookTickerEvent) {
	c.lastBinTime = event.ReceiveTime
	c.binanceHandler(event)
	done := time.Now().UnixNano()
	if done-event.ReceiveTime > 2000000 {
		c.log.Debug("bin_receive_vs_send_time",
			zap.Duration("el", time.Duration(done-event.ReceiveTime)),
			zap.Int("goroutine", runtime.NumGoroutine()),
		)
	}
}

func (c *Collector) ftxEventHandler(event *ftx_ws.Response) {
	if event.Market == usdUsdtMarket {
		c.usdtPrice = event.Data.Bid
	} else {
		c.ftxHandler(event)
	}
	done := time.Now().UnixNano()
	if done-event.ReceiveTime > 10000 {
		c.log.Debug("ftx_receive_vs_send_time",
			zap.Duration("el", time.Duration(done-event.ReceiveTime)),
			zap.Int("goroutine", runtime.NumGoroutine()),
		)
	}
}
func (c *Collector) printStat() {
	t := tabby.New()
	t.AddHeader("symbol", "ma", "count", "min", "max", "bin_all", "ftx_delay", "usdt_price", "time")
	for _, m := range c.cfg.Markets {
		symbol := fmt.Sprintf("%s/%s", strings.ToUpper(m), usdt)
		ma := c.maPriceMap[symbol]
		min, _ := ma.Min()
		max, _ := ma.Max()
		t.AddLine(
			symbol,
			fmt.Sprintf("%.4f", ma.Avg()),
			ma.Count(),
			fmt.Sprintf("%.4f", min),
			fmt.Sprintf("%.4f", max),
			c.getCountAndReset(symbol),
			c.avgDelay(symbol),
			c.usdtPrice,
			time.Now(),
		)
	}
	if c.cfg.Service.EnableUSD {
		for _, m := range c.cfg.Markets {
			symbol := fmt.Sprintf("%s/%s", strings.ToUpper(m), usd)
			ma := c.maPriceMap[symbol]
			min, _ := ma.Min()
			max, _ := ma.Max()
			t.AddLine(
				symbol,
				fmt.Sprintf("%.4f", ma.Avg()),
				ma.Count(),
				fmt.Sprintf("%.4f", min),
				fmt.Sprintf("%.4f", max),
				c.getCountAndReset(symbol),
				c.avgDelay(symbol),
				c.usdtPrice,
				time.Now(),
			)
		}
	}
	t.Print()
}

func (c *Collector) Close() error {
	if c.ec != nil {
		c.ec.Close()
	}
	return nil
}

func (c *Collector) connectNats() error {
	url := fmt.Sprintf("nats://%s:%s", c.cfg.Nats.Host, c.cfg.Nats.Port)
	nc, err := nats.Connect(url)
	if err != nil {
		return fmt.Errorf("connect_nats_error: %w", err)
	}
	ec, err := nats.NewEncodedConn(nc, nats.GOB_ENCODER)
	if err != nil {
		return fmt.Errorf("encoded_connection_error: %w", err)
	}
	c.ec = ec
	return nil
}
func (c *Collector) errHandler(err error) {
	c.log.Error("binance_ws_error", zap.Error(err))
}

func (c *Collector) getCountAndReset(sym string) int64 {
	cnt := c.binCountMap[sym]
	if cnt == nil {
		return 0
	}
	value := cnt.Load()
	cnt.Store(0)
	return value

}
func (c *Collector) addCount(sym string) {
	cnt, ok := c.binCountMap[sym]
	if !ok {
		c.binCountMap[sym] = &atomic.Int64{}
		c.binCountMap[sym].Inc()
	} else {
		cnt.Inc()
	}
}
func (c *Collector) addDelay(sym string, value float64) {
	average, ok := c.maFtxDelay[sym]
	if !ok {
		c.log.Debug("new_delay_map_element", zap.String("sym", sym), zap.Float64("delay", value))
		c.maFtxDelay[sym] = movingaverage.New(c.cfg.Service.AverageWindow)
		c.maFtxDelay[sym].Add(value)
	} else {
		average.Add(value)
	}
}
func (c *Collector) avgDelay(sym string) int64 {
	delay := c.maFtxDelay[sym]
	if delay != nil {
		return int64(delay.Avg())
	}
	return 0
}
func (c *Collector) send(t *TickerData, symbol string) {
	var sb Surebet
	sb.FtxTicker = c.loadTicker(symbol)
	sb.BinTicker = t
	if sb.FtxTicker.AskPrice.IsZero() || sb.BinTicker.AskPrice.IsZero() {
		return
	}

	avgFtx := decimal.Avg(sb.FtxTicker.AskPrice, sb.FtxTicker.BidPrice)
	if strings.Index(sb.FtxTicker.Symbol, usdt) == -1 {
		avgFtx = avgFtx.Div(c.usdtPrice)
	}

	diff := decimal.Avg(sb.BinTicker.AskPrice, sb.BinTicker.BidPrice).Sub(avgFtx).Div(avgFtx).Mul(d100)
	ma := c.maPriceMap[sb.FtxTicker.Symbol]
	c.addDelay(sb.FtxTicker.Symbol, float64(sb.FtxTicker.ReceiveTime-sb.FtxTicker.ServerTime)/1000000)
	ma.Add(diff.InexactFloat64())
	c.addCount(sb.FtxTicker.Symbol)

	if !ma.SlotsFilled() {
		return
	}
	sb.UsdtPrice = c.usdtPrice
	sb.LastBinTime = c.lastBinTime
	sb.ID = c.createID(t.ReceiveTime)
	sb.AvgPriceDiff = decimal.NewFromFloat(ma.Avg()).Round(4)
	max, _ := ma.Max()
	min, _ := ma.Min()
	sb.MaxPriceDiff = decimal.NewFromFloat(max).Round(4)
	sb.MinPriceDiff = decimal.NewFromFloat(min).Round(4)
	//start := time.Now().UnixNano()
	err := c.ec.Publish(cryptoSubject, &sb)
	if err != nil {
		c.log.Warn("send_error", zap.Error(err))
	}
}
func (c *Collector) createID(id int64) int64 {
	c.idLock.Lock()
	defer c.idLock.Unlock()
	if id <= c.lastID {
		id = c.lastID + 1
	}
	c.lastID = id
	return id
}
